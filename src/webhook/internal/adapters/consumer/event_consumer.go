// Package consumer implemente les consumers de files de messages (couche 3, driving).
//
// EventConsumer ecoute les evenements RabbitMQ emis par les autres services
// (messaging, wallet) et invoque le use case DeliverEventUseCase pour les router
// vers les endpoints webhook enregistres.
//
// TODO(amqp): remplacer le broker no-op par une implementation AMQP reelle
// (voir infrastructure/rabbitmq/broker.go) quand la dependance sera disponible.
package consumer

import (
	"context"
	"encoding/json"
	"log"

	"fleece/src/webhook/internal/adapters/webhook"
	"fleece/src/webhook/internal/application/ports/input"
)

// queueName est la queue sur laquelle le consumer ecoute les evenements a router.
const queueName = "webhook.events"

// incomingEvent est la structure du message consomme depuis la queue RabbitMQ.
// Il correspond aux evenements publies par messaging et wallet.
type incomingEvent struct {
	Event       string `json:"event"`
	WorkspaceID string `json:"workspace_id"`
	// Payload brut serialise en JSON — les champs specifiques a chaque type d'evenement.
	// On conserve le message complet comme payload pour les endpoints.
}

// EventConsumer consomme des evenements depuis la queue et invoque DeliverEventUseCase.
type EventConsumer struct {
	broker      webhook.Broker
	deliverEvent input.DeliverEventUseCase
}

// NewEventConsumer cree un EventConsumer.
func NewEventConsumer(broker webhook.Broker, deliverEvent input.DeliverEventUseCase) *EventConsumer {
	return &EventConsumer{
		broker:      broker,
		deliverEvent: deliverEvent,
	}
}

// Start demarre la consommation de la queue. Bloque jusqu'a l'annulation du contexte.
func (c *EventConsumer) Start(ctx context.Context) error {
	log.Printf("event_consumer: demarrage du consumer sur queue=%s", queueName)
	return c.broker.Consume(ctx, queueName, func(body []byte) error {
		// Decoder le type d'evenement et le workspace_id pour le routing.
		var ev incomingEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			log.Printf("event_consumer: erreur unmarshal: %v", err)
			// Ne pas rejeter pour eviter une boucle infinie ; logue et continue.
			return nil
		}

		if ev.WorkspaceID == "" || ev.Event == "" {
			log.Printf("event_consumer: evenement invalide (workspace_id ou event vide): %s", string(body))
			return nil
		}

		// Le body complet est le payload transmis aux endpoints abonnes.
		if err := c.deliverEvent.Execute(ctx, ev.WorkspaceID, ev.Event, body); err != nil {
			log.Printf("event_consumer: DeliverEvent workspace=%s event=%s erreur: %v",
				ev.WorkspaceID, ev.Event, err)
			// Retourner l'erreur permet au broker AMQP reel de nack le message.
			return err
		}
		return nil
	})
}
