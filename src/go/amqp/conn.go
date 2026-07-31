// Package goamqp fournit une connexion RabbitMQ partagée entre les services Go.
// Il enveloppe [github.com/rabbitmq/amqp091-go] et est compatible avec le
// système de configuration par struct tags (key:"" / inject:"").
// Cette lib est transverse : elle ne contient AUCUNE règle métier.
package goamqp

import (
	"context"
	"errors"
	"fmt"
	"time"

	golog "fleece/src/go/log"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// DefaultPrefetch est la valeur de [Conn.Prefetch] appliquée quand celle-ci
// n'est pas renseignée (0 ou négative). RabbitMQ interprète un prefetchCount
// de 0 comme "illimité" — précisément le comportement à éviter : sans borne,
// une boucle de requeue (ex. message classé à tort comme transitoire, cf.
// E2/Phase 3 sur intelligence-processor : channel NULL non géré) peut saturer
// un worker en redeliveries ininterrompues, sans aucun contrôle de débit. 10
// est une valeur raisonnable pour un worker qui traite ses deliveries
// séquentiellement (core-processor, intelligence-processor : une seule
// goroutine de consommation chacun) — borne le nombre de messages "en vol"
// sans dégrader le débit nominal.
const DefaultPrefetch = 10

// Conn est une connexion RabbitMQ partagée, compatible zconfig (struct tags).
// Les champs exportés sont renseignés par injection de dépendances avant
// l'appel à [Conn.Init].
type Conn struct {
	// DSN est l'URL de connexion RabbitMQ au format amqp://user:pass@host:port/vhost.
	DSN string `key:"dsn"`
	// Logger est injecté par le conteneur DI. Peut être nil.
	Logger *golog.Logger `inject:"logger"`
	// Prefetch borne (Channel.Qos, prefetchCount) le nombre de messages
	// non-acquittés livrés simultanément à un consumer ouvert via [Conn.Consume]
	// (global=false : la limite s'applique par consumer, pas par connexion).
	// Champ additif et rétrocompatible : 0 (valeur zéro, y compris pour tout
	// appelant existant qui ne le renseigne pas) retombe sur [DefaultPrefetch] —
	// aucun changement de comportement pour un Conn construit sans ce champ,
	// hormis le passage d'un Qos auparavant absent (illimité) à une valeur bornée.
	Prefetch int `key:"prefetch"`
	// PublishTimeout borne l'attente de confirmation d'une publication
	// ([Conn.Publish], D-M34). Champ additif et rétrocompatible : 0 (valeur
	// zéro, y compris pour tout appelant existant qui ne le renseigne pas)
	// retombe sur [DefaultPublishTimeout]. Le seul changement de comportement
	// est le passage d'une attente auparavant NON BORNÉE à une attente bornée.
	PublishTimeout time.Duration `key:"publish_timeout"`

	conn *amqp091.Connection
}

// Init établit la connexion AMQP en utilisant [Conn.DSN].
// Si un Logger est disponible, il est enrichi du module "amqp".
// Retourne une erreur si la connexion échoue.
func (c *Conn) Init() error {
	if c.Logger != nil {
		c.Logger = c.Logger.With("module", "amqp")
	}

	conn, err := amqp091.Dial(c.DSN)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// Channel ouvre et retourne un nouveau canal AMQP depuis la connexion active.
func (c *Conn) Channel() (*amqp091.Channel, error) {
	return c.conn.Channel()
}

// buildPersistentPublishing construit le message amqp091.Publishing émis par
// [Conn.Publish] : DeliveryMode: Persistent (D-M29, Phase 3) — sans ce champ,
// la valeur par défaut d'amqp091-go est Transient : un redémarrage du broker
// (ou une simple perte du processus RabbitMQ) entre la publication et sa
// consommation effacerait silencieusement le message, MÊME si la queue
// cible est elle-même durable (la durabilité de la queue ne survit un
// redémarrage QUE pour les messages eux-mêmes marqués persistants). Fonction
// pure, extraite de Publish pour rester testable sans connexion AMQP réelle.
func buildPersistentPublishing(body []byte) amqp091.Publishing {
	return amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		Body:         body,
	}
}

// ErrPublishNotConfirmed est retournée par [Conn.Publish] quand le broker a
// explicitement NACK la publication (ressources indisponibles côté broker,
// erreur interne...). Distincte d'une erreur de canal/réseau (retournée telle
// quelle) : ErrPublishNotConfirmed signale que la publication A BIEN atteint
// le broker, qui l'a explicitement rejetée.
//
// CORRECTION DE DOCUMENTATION (D-M35) — cette variable NE COUVRE PAS le cas
// « message envoyé à un exchange sans queue liée », contrairement à ce que ce
// commentaire affirmait. Avec mandatory=false, RabbitMQ **ACK** un message
// non routable : il le jette et le confirme. C'est exactement pourquoi
// mandatory vaut désormais true et pourquoi ce cas a son propre sentinel,
// [ErrPublishUnroutable].
var ErrPublishNotConfirmed = errors.New("goamqp: publication non confirmée par le broker (nack)")

// ErrPublishUnroutable est retournée par [Conn.Publish] quand le broker a
// RENVOYÉ le message parce qu'aucune queue ne correspondait à sa routing key
// (basic.return, mandatory=true).
//
// CORRECTIF D-M35 — LE MESSAGE ÉTAIT SILENCIEUSEMENT DÉTRUIT. Avec
// mandatory=false, un message non routable est ACK par le broker : Publish
// retournait nil, l'appelant croyait la publication réussie, et l'événement
// disparaissait sans une ligne de log. Le scénario n'était pas théorique :
// src/api/cmd/api/main.go ne déclare QUE l'exchange, jamais les queues ni les
// bindings (ceux-ci appartiennent aux workers). Si src/api démarrait avant le
// premier lancement de core-processor, TOUS les DLR étaient détruits — et
// depuis D-M26, la publication est la seule voie d'écriture du statut d'un
// message : la perte aurait été totale et muette.
var ErrPublishUnroutable = errors.New("goamqp: message non routable, renvoyé par le broker (aucune queue liée à cette routing key)")

// DefaultPublishTimeout borne l'attente de confirmation d'une publication
// (D-M34).
//
// SANS CETTE BORNE, Publish pouvait pendre INDÉFINIMENT : WaitContext n'a
// d'autre sortie que le contexte fourni, et le contexte d'une requête HTTP
// n'est annulé que si le client coupe. Or RabbitMQ, en alarme mémoire ou
// disque, BLOQUE ses publishers — connexion TCP vivante, aucun ack, aucune
// erreur. Tous les POST /messages restaient alors suspendus, là où ils
// répondaient immédiatement avant l'introduction du mode confirm (D-M29).
//
// 5 secondes : largement au-dessus d'un aller-retour broker normal
// (millisecondes), largement en dessous du WriteTimeout du serveur HTTP
// (30 s, cf. src/api/cmd/api/main.go) pour que le handler ait le temps de
// formater sa réponse d'erreur.
const DefaultPublishTimeout = 5 * time.Second

// Publish publie un message JSON sur l'exchange donné avec la routing key
// indiquée. Un canal est ouvert puis fermé à chaque appel.
//
// mandatory=true (D-M35) : un message qu'aucune queue ne peut recevoir est
// RENVOYÉ par le broker et remonte en [ErrPublishUnroutable], au lieu d'être
// acquitté puis jeté en silence. immediate reste à false (déprécié par
// RabbitMQ, qui refuse la connexion s'il vaut true).
//
// L'attente de confirmation est BORNÉE par [Conn.PublishTimeout] (défaut
// [DefaultPublishTimeout], D-M34) en plus du contexte de l'appelant.
//
// FIABILISATION (D-M29, Phase 3) — AVANT ce correctif, Publish n'utilisait ni
// le mode confirm ni DeliveryMode: Persistent : ch.PublishWithContext renvoie
// nil dès que le message a été ACCEPTÉ PAR LE CANAL LOCAL, AVANT toute
// confirmation du broker. Si l'exchange ciblé n'existe pas (ex. topologie non
// déclarée, voir [Conn.DeclareExchange]), le broker ferme le canal
// (channel-exception, code AMQP 404) de façon ASYNCHRONE — Publish avait déjà
// retourné nil, l'appelant (ex. webhooks_telegram.go, messages_send.go)
// croyait la publication réussie alors que RIEN n'avait été routé : perte
// SILENCIEUSE et totale de l'événement, sans une seule ligne de log côté
// producteur.
//
// CORRECTIF : le canal est mis en mode confirm (Channel.Confirm(false)) avant
// publication, puis PublishWithDeferredConfirmWithContext est utilisé à la
// place de PublishWithContext pour obtenir un [amqp091.DeferredConfirmation],
// dont on attend explicitement l'issue (WaitContext) avant de retourner. Une
// publication qui ne peut pas être confirmée (nack explicite, ou erreur
// réseau/canal survenue entre-temps) retourne désormais une erreur — jamais
// nil — permettant à l'appelant de logger/traiter l'échec au lieu de croire à
// tort la publication réussie.
//
// Signature INCHANGÉE (rétrocompatibilité) : tous les appelants existants
// (src/api/messages_send.go, src/api/webhooks_telegram.go,
// src/intelligence-processor/campaign_scheduler.go) continuent de fonctionner
// à l'identique sur le chemin nominal (ack reçu -> nil, comme avant) ; seul le
// chemin d'échec, auparavant silencieux, retourne désormais une erreur.
func (c *Conn) Publish(ctx context.Context, exchange, key string, body []byte) error {
	// D-M34 : borne dure de l'attente de confirmation, INDÉPENDANTE du contexte
	// de l'appelant. Le ctx reçu reste respecté (annulation client, SIGTERM) —
	// on ne fait qu'ajouter une échéance qu'il n'avait pas.
	timeout := c.PublishTimeout
	if timeout <= 0 {
		timeout = DefaultPublishTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("goamqp: publish: passage en mode confirm: %w", err)
	}

	// D-M35 : capter les basic.return AVANT de publier. Le broker émet le
	// return avant l'ack, et le canal est fermé au retour de cette fonction —
	// s'abonner après la publication laisserait une fenêtre de perte.
	// Tampon de 1 : un seul message est publié par appel.
	returns := ch.NotifyReturn(make(chan amqp091.Return, 1))

	confirmation, err := ch.PublishWithDeferredConfirmWithContext(
		ctx,
		exchange,
		key,
		true,  // mandatory (D-M35) — sans cela, un message non routable est ACK puis jeté
		false, // immediate
		buildPersistentPublishing(body),
	)
	if err != nil {
		return err
	}
	if confirmation == nil {
		// Ne devrait jamais arriver après un Confirm() réussi (amqp091-go ne
		// retourne nil que si le canal n'est pas en mode confirm) — gardé par
		// défense, pour ne jamais faire croire à tort qu'une publication a été
		// confirmée en son absence.
		return errors.New("goamqp: publish: aucune confirmation retournée par le canal (mode confirm non actif)")
	}

	ok, err := confirmation.WaitContext(ctx)
	if cerr := interpretConfirmation(ok, err); cerr != nil {
		return cerr
	}

	// D-M35 : le message a été ACK — ce qui, avec mandatory=true, n'exclut PAS
	// qu'il ait d'abord été renvoyé comme non routable. Le protocole garantit
	// que le basic.return précède l'ack, donc à ce point la lecture non
	// bloquante est suffisante : si un return existe, il est déjà là.
	select {
	case ret := <-returns:
		return fmt.Errorf("%w (exchange=%q, routing_key=%q, code=%d, raison=%q)",
			ErrPublishUnroutable, ret.Exchange, ret.RoutingKey, ret.ReplyCode, ret.ReplyText)
	default:
		return nil
	}
}

// interpretConfirmation traduit le résultat de
// [amqp091.DeferredConfirmation.WaitContext] (ok, err) en la valeur de retour
// de [Conn.Publish]. Extraite en fonction PURE (aucune I/O) pour rester
// testable sans connexion AMQP réelle — c'est la brique qui matérialise le
// correctif D-M29 "Publish non confirmé ⇒ erreur, jamais nil silencieux".
func interpretConfirmation(ok bool, err error) error {
	if err != nil {
		return fmt.Errorf("goamqp: publish: attente de la confirmation: %w", err)
	}
	if !ok {
		return ErrPublishNotConfirmed
	}
	return nil
}

// Consume enregistre un consumer sur la queue indiquée et retourne le canal
// de delivery. Le canal AMQP reste ouvert pour la durée de vie du flux.
// autoAck est positionné à false : les messages doivent être acquittés
// manuellement par le consommateur.
//
// Qos (prefetchCount = [Conn.Prefetch], ou [DefaultPrefetch] si non renseigné ;
// global=false) est posé sur CE canal avant Consume — borne le nombre de
// deliveries non-acquittées en vol simultanément (E2, Phase 3) : sans cette
// borne, une boucle de requeue (message à tort classé transitoire, ex. erreur
// de scan sur une colonne nullable non gérée) tourne à débit illimité.
func (c *Conn) Consume(queue string) (<-chan amqp091.Delivery, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}

	prefetch := c.Prefetch
	if prefetch <= 0 {
		prefetch = DefaultPrefetch
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return nil, err
	}

	return ch.Consume(
		queue,
		"",    // consumer tag (vide = auto-généré)
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
}

// Close ferme la connexion AMQP sous-jacente si elle est active.
func (c *Conn) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
