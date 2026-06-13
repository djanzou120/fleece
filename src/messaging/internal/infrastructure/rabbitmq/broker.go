// Package rabbitmq fournit l'implémentation concrète du Broker RabbitMQ (couche 4, infrastructure).
//
// L'interface Broker est définie en couche 3 (internal/adapters/messaging).
// Ce package ne contient que la/les implémentations concrètes qui la satisfont.
// Elles sont instanciées au composition root (main.go) et injectées dans les adapters.
//
// TODO(amqp): remplacer NoopBroker par une implémentation AMQP réelle
// (p. ex. github.com/rabbitmq/amqp091-go) une fois la dépendance disponible offline.
package rabbitmq

import (
	"context"
	"log"

	"fleece/src/messaging/internal/adapters/messaging"
)

// Assertion de conformité à la compilation : NoopBroker doit implémenter messaging.Broker.
// Couche 4 → couche 3 : sens autorisé (vers l'intérieur).
var _ messaging.Broker = (*NoopBroker)(nil)

// NoopBroker est l'implémentation par défaut : elle logue et ne fait rien.
// Utilisée quand RabbitMQ n'est pas configuré ou pas encore disponible.
//
// TODO(amqp): créer AMQPBroker qui implémente messaging.Broker avec amqp091.
type NoopBroker struct{}

// NewNoopBroker retourne un NoopBroker (satisfait messaging.Broker).
func NewNoopBroker() *NoopBroker {
	return &NoopBroker{}
}

// Publish logue le message sans le transmettre réellement.
func (b *NoopBroker) Publish(_ context.Context, routingKey string, body []byte) error {
	log.Printf("[broker:noop] publish routingKey=%s body_len=%d", routingKey, len(body))
	return nil
}

// Consume logue et bloque jusqu'à l'annulation du contexte sans consommer de messages réels.
func (b *NoopBroker) Consume(ctx context.Context, queueName string, _ func([]byte) error) error {
	log.Printf("[broker:noop] consume queueName=%s (no-op, waiting for context cancellation)", queueName)
	<-ctx.Done()
	return nil
}
