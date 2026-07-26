package intelligenceprocessor

// consumer.go — connexion au consumer AMQP et dispatch par routing key pour le
// worker intelligence-processor.
//
// QUEUE DISTINCTE de src/core-processor (decision PM, M-021) : s.Queue
// (defaut "intelligence-processor", env INTELLIGENCE_PROCESSOR_QUEUE) est
// BINDEE (cote infrastructure RabbitMQ, hors perimetre Go de cette tache) aux
// MEMES routing keys que la queue "core-processor" sur le MEME exchange
// "fleece" — au minimum "message.delivered"/"message.failed", plus
// "message.sent" et "campaign.run" ici specifiquement. Deux queues distinctes
// bindees au meme exchange topic recoivent CHACUNE leur propre copie de
// chaque publication (RabbitMQ topic exchange : un message publie est route
// vers TOUTES les queues dont un binding matche la routing key — pas une
// seule qui le "consommerait" au sens FIFO partage). core-processor et
// intelligence-processor traitent donc chacun INDEPENDAMMENT le meme
// evenement message.delivered/message.failed, sans jamais se disputer LA
// MEME delivery AMQP. C'est ce qui permet a intelligence-processor de ne
// JAMAIS toucher messaging.messages.status (proprietaire exclusif :
// core-processor, Decision C) tout en reagissant au meme evenement pour ses
// propres effets (score contact, analytics, correlation campagne).
//
// CONTRATS D'EVENEMENTS CONSOMMES PAR CE WORKER :
//
//	"message.sent" (producteur : src/api/messages_send.go, publishMessageSent) :
//	  {"message_id":"...", "workspace_id":"...", "recipient":"...", "provider_id":"...", "status":"..."}
//
//	"message.delivered" / "message.failed" (producteur : src/api/webhooks_telegram.go,
//	CONTRAT D-M21 — verifie dans le code avant d'ecrire ce fichier — identique
//	cote consommateur a src/core-processor/consumer.go) :
//	  {"message_id":"...", "external_id":"...", "provider_id":"...", "status":"...", "source":"..."}
//
//	"campaign.run" (producteur : campaign_scheduler.go DE CE WORKER — contrat
//	FIGE par ce worker, voir on_campaign_run.go) :
//	  {"campaign_id":"...", "run_id":"..."}
//
// AUTONOMIE (voir chaque handler, on_message_*.go/on_campaign_run.go) : seuls
// message_id / campaign_id / run_id sont utilises pour les ecritures. Tous
// les autres champs du payload message.* sont des enrichissements
// d'observabilite — jamais une source de verite pour un compteur ou un
// montant (un evenement rejoue ou force ne doit jamais pouvoir fausser une
// agregation) : workspace_id/recipient/channel/cost/created_at sont TOUJOURS
// relus depuis messaging.messages par message_id.
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

// messageEvent est la structure JSON commune aux evenements "message.sent",
// "message.delivered" et "message.failed" (voir contrats ci-dessus). Tous les
// champs autres que MessageID sont optionnels (omitempty) car aucun des trois
// producteurs ne les fournit tous.
type messageEvent struct {
	MessageID   string `json:"message_id"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Recipient   string `json:"recipient,omitempty"`
	ExternalID  string `json:"external_id,omitempty"`
	ProviderID  string `json:"provider_id,omitempty"`
	Status      string `json:"status,omitempty"`
	Source      string `json:"source,omitempty"`
}

// uuidLike valide grossierement la FORME d'un UUID (8-4-4-4-12 caracteres
// hexadecimaux) — identique a src/core-processor/consumer.go. Utilise pour
// message_id (messageEvent) ET campaign_id (campaignRunEvent, voir
// on_campaign_run.go).
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

// validateMessageID rejette (erreur permanente, voir isPermanentError) tout
// message_id vide ou syntaxiquement invalide, AVANT toute requete SQL (qui
// echouerait sinon avec une erreur Postgres "invalid input syntax for type
// uuid", indiscernable d'une panne technique transitoire). Appelee
// explicitement par chaque handler message.* (pas de validation generique
// dans dispatch : les 4 evenements consommes par ce worker n'ont pas tous la
// meme forme de payload — voir campaignRunEvent, on_campaign_run.go, qui a sa
// propre validation).
func validateMessageID(id string) error {
	if id == "" {
		return errors.New("message_id vide dans l'evenement")
	}
	if !uuidLike.MatchString(id) {
		return fmt.Errorf("message_id syntaxiquement invalide (pas un UUID): %q", id)
	}
	return nil
}

// permanentError marque une erreur de traitement qui ne doit JAMAIS etre
// requeue (le message est structurellement invalide/inconnu pour ce worker) —
// identique a src/core-processor/consumer.go. Distincte d'une erreur "nue"
// (panne technique transitoire), qui DOIT etre requeue.
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

// eventHandler traite le corps brut d'une delivery AMQP deja resolue par
// routing key. Chaque handler decode et valide lui-meme son payload (les 4
// routing keys geres par ce worker n'ont pas la meme forme de payload) et
// retourne une erreur "nue" pour toute panne technique transitoire (requeue)
// ou une permanentError pour un message structurellement invalide/inconnu
// (pas de requeue).
type eventHandler func(ctx context.Context, s *Service, body []byte) error

// handlersByRoutingKey associe chaque routing key AMQP geree par ce worker a
// son handler. Chaque handler est defini dans son propre fichier (regle
// transverse n°1 : 1 fichier par type d'evenement) :
//   - handleMessageSent      -> on_message_sent.go
//   - handleMessageDelivered -> on_message_delivered.go
//   - handleMessageFailed    -> on_message_failed.go
//   - handleCampaignRun      -> on_campaign_run.go
var handlersByRoutingKey = map[string]eventHandler{
	"message.sent":      handleMessageSent,
	"message.delivered": handleMessageDelivered,
	"message.failed":    handleMessageFailed,
	"campaign.run":      handleCampaignRun,
}

// resolveHandler retrouve le handler associe a une routing key AMQP. Fonction
// pure (pas d'I/O), testable isolement.
func resolveHandler(routingKey string) (eventHandler, bool) {
	h, ok := handlersByRoutingKey[routingKey]
	return h, ok
}

// BoundRoutingKeys retourne, dans un ordre deterministe (trie), les routing
// keys que ce worker consomme reellement (les cles de handlersByRoutingKey,
// SOURCE UNIQUE) — utilisee par le composition root
// (cmd/intelligence-processor/main.go, D-M29, Phase 3) pour lier la queue
// "intelligence-processor" a l'exchange "fleece" : la topologie AMQP declaree
// au demarrage ne peut ainsi JAMAIS diverger de ce que dispatch() route
// effectivement (meme principe que src/core-processor/consumer.go).
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
// composition root via goapp.Context, voir cmd/intelligence-processor/main.go)
// ou fermeture du canal de delivery (ex. perte de connexion AMQP cote broker).
//
// autoAck=false (voir goamqp.Conn.Consume) : chaque delivery DOIT etre
// acquittee explicitement — c'est processDelivery qui s'en charge selon le
// resultat de dispatch (meme politique que src/core-processor) :
//   - succes                                        -> Ack(false)
//   - erreur technique transitoire (panne DB, etc.) -> Nack(false, true)  (requeue)
//   - erreur permanente / panique recuperee          -> Nack(false, false) (PAS de requeue)
func (s *Service) Consume(ctx context.Context) error {
	deliveries, err := s.AMQP.Consume(s.Queue)
	if err != nil {
		return err
	}

	s.Logger.Info("intelligence-processor: consommation demarree", "queue", s.Queue)
	return s.consumeLoop(ctx, deliveries)
}

// consumeLoop est le corps de la boucle de consommation, isole de l'ouverture
// du canal AMQP (Consume) pour rester testable sans connexion RabbitMQ reelle.
func (s *Service) consumeLoop(ctx context.Context, deliveries <-chan amqp091.Delivery) error {
	for {
		select {
		case <-ctx.Done():
			s.Logger.Info("intelligence-processor: arret demande, fin de la boucle de consommation")
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				s.Logger.Warn("intelligence-processor: canal de delivery AMQP ferme")
				return nil
			}
			s.processDelivery(ctx, delivery)
		}
	}
}

// processDelivery traite une delivery AMQP unique : dispatch, puis Ack/Nack
// selon le resultat, avec recuperation de panique (D-M18, meme correctif
// applique ici que src/core-processor/consumer.go et src/api/service.go).
func (s *Service) processDelivery(ctx context.Context, delivery amqp091.Delivery) {
	defer func() {
		if rec := recover(); rec != nil {
			s.Logger.Error("intelligence-processor: panique recuperee dans un handler",
				"panic", rec, "routing_key", delivery.RoutingKey, "stack", string(debug.Stack()))
			if nackErr := delivery.Nack(false, false); nackErr != nil {
				s.Logger.Error("intelligence-processor: nack (post-panique) echoue", "err", nackErr)
			}
		}
	}()

	err := s.dispatch(ctx, delivery)
	switch {
	case err == nil:
		if ackErr := delivery.Ack(false); ackErr != nil {
			s.Logger.Error("intelligence-processor: ack echoue", "err", ackErr, "routing_key", delivery.RoutingKey)
		}
	case isPermanentError(err):
		s.Logger.Error("intelligence-processor: message rejete definitivement (pas de requeue)",
			"err", err, "routing_key", delivery.RoutingKey)
		if nackErr := delivery.Nack(false, false); nackErr != nil {
			s.Logger.Error("intelligence-processor: nack (permanent) echoue", "err", nackErr)
		}
	default:
		s.Logger.Warn("intelligence-processor: erreur technique, message requeue",
			"err", err, "routing_key", delivery.RoutingKey)
		if nackErr := delivery.Nack(false, true); nackErr != nil {
			s.Logger.Error("intelligence-processor: nack (requeue) echoue", "err", nackErr)
		}
	}
}

// dispatch resout le handler par routing key et l'invoque avec le corps brut
// de la delivery. Isole du Ack/Nack (processDelivery) pour rester
// unitairement testable sans AMQP reel.
func (s *Service) dispatch(ctx context.Context, delivery amqp091.Delivery) error {
	handler, ok := resolveHandler(delivery.RoutingKey)
	if !ok {
		return newPermanentError(fmt.Errorf("dispatch: routing key inconnue: %q", delivery.RoutingKey))
	}
	return handler(ctx, s, delivery.Body)
}
