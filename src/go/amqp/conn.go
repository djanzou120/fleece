// Package goamqp fournit une connexion RabbitMQ partagée entre les services Go.
// Il enveloppe [github.com/rabbitmq/amqp091-go] et est compatible avec le
// système de configuration par struct tags (key:"" / inject:"").
// Cette lib est transverse : elle ne contient AUCUNE règle métier.
package goamqp

import (
	"context"
	"errors"
	"fmt"

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
// explicitement NACK la publication (ex. message unroutable envoyé à un
// exchange sans queue liée qui le refuse, ressources indisponibles côté
// broker...). Distincte d'une erreur de canal/réseau (retournée telle
// quelle) : ErrPublishNotConfirmed signale que la publication A BIEN atteint
// le broker, qui l'a explicitement rejetée.
var ErrPublishNotConfirmed = errors.New("goamqp: publication non confirmée par le broker (nack)")

// Publish publie un message JSON sur l'exchange donné avec la routing key
// indiquée. Un canal est ouvert puis fermé à chaque appel. Les flags
// mandatory et immediate sont positionnés à false.
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
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("goamqp: publish: passage en mode confirm: %w", err)
	}

	confirmation, err := ch.PublishWithDeferredConfirmWithContext(
		ctx,
		exchange,
		key,
		false, // mandatory
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
	return interpretConfirmation(ok, err)
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
