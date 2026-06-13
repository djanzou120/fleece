// Package consumer implémente les consumers de files de messages (couche 3, driving).
//
// TODO(amqp): remplacer le broker no-op par une implémentation AMQP réelle
// (voir infrastructure/rabbitmq/broker.go) quand la dépendance sera disponible.
package consumer

import (
	"context"
	"encoding/json"
	"log"

	"fleece/src/messaging/internal/adapters/messaging"
	"fleece/src/messaging/internal/application/usecases"
	"fleece/src/messaging/internal/domain"
)

const sendQueueName = "messaging.send"

// sendWorkerMessage est la structure du message consommé depuis la queue.
type sendWorkerMessage struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Recipient   string `json:"recipient"`
	Content     string `json:"content"`
}

// SendWorker consomme des messages depuis la queue de travail et invoque SendMessage.
type SendWorker struct {
	broker      messaging.Broker
	sendMessage *usecases.SendMessage
}

// NewSendWorker crée un SendWorker.
func NewSendWorker(broker messaging.Broker, uc *usecases.SendMessage) *SendWorker {
	return &SendWorker{broker: broker, sendMessage: uc}
}

// Start démarre la consommation de la queue. Bloque jusqu'à l'annulation du contexte.
func (w *SendWorker) Start(ctx context.Context) error {
	log.Printf("send_worker: starting consumer on queue=%s", sendQueueName)
	return w.broker.Consume(ctx, sendQueueName, func(body []byte) error {
		var msg sendWorkerMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			log.Printf("send_worker: unmarshal error: %v", err)
			// On ne rejette pas le message pour éviter une boucle infinie ;
			// on logue et on continue.
			return nil
		}

		m := domain.NewMessage(msg.ID, msg.WorkspaceID, msg.Recipient, msg.Content)
		if err := w.sendMessage.Execute(ctx, m); err != nil {
			log.Printf("send_worker: SendMessage error message_id=%s: %v", msg.ID, err)
			// Retourner l'erreur permet au broker (AMQP réel) de nack le message.
			return err
		}
		return nil
	})
}
