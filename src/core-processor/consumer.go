package coreprocessor

// consumer.go — connexion au consumer AMQP et dispatch par routing key.
//
// CONTRAT DE L'EVENEMENT AMQP "message.delivered" / "message.failed" (D-M21,
// COTE CONSOMMATEUR — voir src/api/webhooks_telegram.go pour le contrat cote
// PRODUCTEUR, qui doit rester rigoureusement identique) :
//
//	{
//	  "message_id":  "<uuid Fleece — messaging.messages.id>",
//	  "external_id": "<identifiant provider, ex. message_id Telegram>",
//	  "provider_id": "telegram-bot",
//	  "status":      "delivered|failed|...",
//	  "source":      "telegram"
//	}
//
// Seul message_id est utilise par ce worker pour ses ecritures (WHERE id = $1
// sur messaging.messages) : c'est la seule clef de correlation fiable garantie
// par le producteur (D-M21). external_id/provider_id/status/source sont des
// champs d'enrichissement/observabilite (evitent un aller-retour DB au
// worker pour retrouver le provider ou le statut d'origine) MAIS ce worker ne
// leur fait PAS confiance pour ses ecritures monetaires : le montant a
// rembourser (voir on_message_failed.go) est TOUJOURS relu depuis
// messaging.messages.cost en base, jamais depuis le payload — un evenement
// forge ou rejoue ne doit jamais pouvoir crediter arbitrairement un wallet.
//
// Un evenement sans message_id exploitable (vide, ou syntaxiquement pas un
// UUID) est un message empoisonne : rejet permanent (voir isPermanentError),
// jamais de requeue.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"runtime/debug"
	"sort"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// messageEvent est la structure JSON exacte de l'evenement AMQP "message.delivered"/
// "message.failed" (voir contrat ci-dessus, D-M21).
type messageEvent struct {
	MessageID  string `json:"message_id"`
	ExternalID string `json:"external_id,omitempty"`
	ProviderID string `json:"provider_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Source     string `json:"source,omitempty"`
}

// uuidLike valide grossierement la FORME d'un UUID (8-4-4-4-12 caracteres
// hexadecimaux). Ne verifie pas la version/variant — suffisant pour rejeter
// tot un message_id manifestement invalide avant toute requete SQL (qui
// echouerait sinon avec une erreur Postgres "invalid input syntax for type
// uuid", indiscernable d'une panne technique transitoire).
var uuidLike = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// parseMessageEvent decode le corps JSON d'une delivery AMQP en messageEvent.
// Fonction pure (aucune I/O), testable isolement de tout acces AMQP/DB.
func parseMessageEvent(body []byte) (messageEvent, error) {
	var evt messageEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return messageEvent{}, fmt.Errorf("parseMessageEvent: json invalide: %w", err)
	}
	return evt, nil
}

// permanentError marque une erreur de traitement qui ne doit JAMAIS etre
// requeue (le message est structurellement invalide/inconnu pour ce worker —
// un requeue infini sur un message empoisonne serait un incident de
// production : boucle CPU/IO indefinie, aucune chance de succes futur).
// Distincte d'une erreur "nue" (panne technique transitoire, ex. DB
// indisponible), qui DOIT etre requeue — voir processDelivery.
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// newPermanentError enveloppe err en erreur permanente (jamais requeue).
func newPermanentError(err error) error { return &permanentError{err: err} }

// isPermanentError signale si err (ou l'une de ses causes, via errors.As) a
// ete produite par newPermanentError.
func isPermanentError(err error) bool {
	var pe *permanentError
	return errors.As(err, &pe)
}

// eventHandler traite un messageEvent deja parse et valide (message_id non
// vide et syntaxiquement UUID — verifie par dispatch avant l'appel). Retourne
// une erreur "nue" pour toute panne technique transitoire (requeue) ; les
// handlers n'ont pas a produire d'erreur permanente puisque dispatch a deja
// filtre les cas structurellement invalides.
type eventHandler func(ctx context.Context, s *Service, evt messageEvent) error

// handlersByRoutingKey associe chaque routing key AMQP geree par ce worker a
// son handler. handleMessageDelivered/handleMessageFailed sont definis
// respectivement dans on_message_delivered.go et on_message_failed.go (regle
// transverse n°1 : un fichier par type d'evenement).
var handlersByRoutingKey = map[string]eventHandler{
	"message.delivered": handleMessageDelivered,
	"message.failed":    handleMessageFailed,
}

// resolveHandler retrouve le handler associe a une routing key AMQP. Fonction
// pure (pas d'I/O), testable isolement — brique de dispatch avec
// parseMessageEvent.
func resolveHandler(routingKey string) (eventHandler, bool) {
	h, ok := handlersByRoutingKey[routingKey]
	return h, ok
}

// BoundRoutingKeys retourne, dans un ordre deterministe (trie), les routing
// keys que ce worker consomme reellement (les cles de handlersByRoutingKey,
// SOURCE UNIQUE) — utilisee par le composition root (cmd/core-processor/main.go,
// D-M29, Phase 3) pour lier la queue "core-processor" a l'exchange "fleece" :
// la topologie AMQP declaree au demarrage ne peut ainsi JAMAIS diverger de ce
// que dispatch() route effectivement (un ajout/retrait de routing key dans
// handlersByRoutingKey se repercute automatiquement sur la binding declaree,
// sans jamais avoir a maintenir une seconde liste en parallele).
func BoundRoutingKeys() []string {
	keys := make([]string, 0, len(handlersByRoutingKey))
	for k := range handlersByRoutingKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Consume ouvre un canal de consommation AMQP sur s.Queue et traite chaque
// delivery recue jusqu'a annulation de ctx (SIGINT/SIGTERM propage depuis le
// composition root via goapp.Context, voir cmd/core-processor/main.go) ou
// fermeture du canal de delivery (ex. perte de connexion AMQP cote broker).
//
// autoAck=false (voir goamqp.Conn.Consume) : chaque delivery DOIT etre
// acquittee explicitement — c'est processDelivery qui s'en charge selon le
// resultat de dispatch :
//   - succes                                        -> Ack(false)
//   - erreur technique transitoire (panne DB, etc.) -> Nack(false, true)  (requeue)
//   - erreur permanente / panique recuperee          -> Nack(false, false) (PAS de requeue)
//
// Voir processDelivery pour la justification detaillee de cette politique.
func (s *Service) Consume(ctx context.Context) error {
	deliveries, err := s.AMQP.Consume(s.Queue)
	if err != nil {
		return err
	}

	s.Logger.Info("core-processor: consommation demarree", "queue", s.Queue)
	return s.consumeLoop(ctx, deliveries)
}

// consumeLoop est le corps de la boucle de consommation, isole de l'ouverture
// du canal AMQP (Consume) pour rester testable sans connexion RabbitMQ reelle :
// un simple chan amqp091.Delivery construit a la main (meme forme que celui
// retourne par goamqp.Conn.Consume) suffit a exercer l'arret sur annulation de
// contexte et la fermeture de canal.
func (s *Service) consumeLoop(ctx context.Context, deliveries <-chan amqp091.Delivery) error {
	for {
		select {
		case <-ctx.Done():
			s.Logger.Info("core-processor: arret demande, fin de la boucle de consommation")
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				s.Logger.Warn("core-processor: canal de delivery AMQP ferme")
				return nil
			}
			s.processDelivery(ctx, delivery)
		}
	}
}

// processDelivery traite une delivery AMQP unique : dispatch, puis Ack/Nack
// selon le resultat, avec recuperation de panique.
//
// Recuperation de panique (D-M18, meme correctif applique ici cote worker que
// pour recoverMiddleware de src/api/service.go) : une panique dans un handler
// (ex. nil pointer, assertion de type) ne doit JAMAIS tuer le processus
// worker entier — seul CE message est rejete (Nack sans requeue, la panique
// indique un defaut structurel qui se reproduirait a l'identique). La stack
// est loggee (runtime/debug.Stack()) pour rester debuggable en production.
func (s *Service) processDelivery(ctx context.Context, delivery amqp091.Delivery) {
	defer func() {
		if rec := recover(); rec != nil {
			s.Logger.Error("core-processor: panique recuperee dans un handler",
				"panic", rec, "routing_key", delivery.RoutingKey, "stack", string(debug.Stack()))
			if nackErr := delivery.Nack(false, false); nackErr != nil {
				s.Logger.Error("core-processor: nack (post-panique) echoue", "err", nackErr)
			}
		}
	}()

	err := s.dispatch(ctx, delivery)
	switch {
	case err == nil:
		if ackErr := delivery.Ack(false); ackErr != nil {
			s.Logger.Error("core-processor: ack echoue", "err", ackErr, "routing_key", delivery.RoutingKey)
		}
	case isPermanentError(err):
		s.Logger.Error("core-processor: message rejete definitivement (pas de requeue)",
			"err", err, "routing_key", delivery.RoutingKey)
		if nackErr := delivery.Nack(false, false); nackErr != nil {
			s.Logger.Error("core-processor: nack (permanent) echoue", "err", nackErr)
		}
	default:
		s.Logger.Warn("core-processor: erreur technique, message requeue",
			"err", err, "routing_key", delivery.RoutingKey)
		if nackErr := delivery.Nack(false, true); nackErr != nil {
			s.Logger.Error("core-processor: nack (requeue) echoue", "err", nackErr)
		}
	}
}

// dispatch parse le corps de la delivery, valide message_id, resout le
// handler par routing key et l'invoque. Isole du Ack/Nack (processDelivery)
// pour rester unitairement testable sans AMQP reel (un amqp091.Delivery
// zero-valeur suffit, RoutingKey/Body sont des champs exportes).
func (s *Service) dispatch(ctx context.Context, delivery amqp091.Delivery) error {
	evt, err := parseMessageEvent(delivery.Body)
	if err != nil {
		return newPermanentError(err)
	}

	if evt.MessageID == "" {
		return newPermanentError(errors.New("dispatch: message_id vide dans l'evenement"))
	}
	if !uuidLike.MatchString(evt.MessageID) {
		return newPermanentError(fmt.Errorf("dispatch: message_id syntaxiquement invalide (pas un UUID): %q", evt.MessageID))
	}

	handler, ok := resolveHandler(delivery.RoutingKey)
	if !ok {
		return newPermanentError(fmt.Errorf("dispatch: routing key inconnue: %q", delivery.RoutingKey))
	}

	return handler(ctx, s, evt)
}
