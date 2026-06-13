// Package rabbitmq fournit l'implementation concrete du Broker RabbitMQ (couche 4, infrastructure).
//
// L'interface Broker est definie en couche 3 (internal/adapters/messaging).
// Ce package ne contient que la/les implementations concretes qui la satisfont.
// Elles sont instanciees au composition root (main.go) et injectees dans les adapters.
//
// TODO(amqp): remplacer NoopBroker par une implementation AMQP reelle
// (p. ex. github.com/rabbitmq/amqp091-go) une fois la dependance disponible offline.
package rabbitmq

import (
	"context"
	"log"

	"fleece/src/wallet/internal/adapters/messaging"
)

// Assertion de conformite a la compilation : NoopBroker doit implementer messaging.Broker.
// Couche 4 → couche 3 : sens autorise (vers l'interieur).
var _ messaging.Broker = (*NoopBroker)(nil)

// NoopBroker est l'implementation par defaut : elle logue et ne fait rien.
// Utilisee quand RabbitMQ n'est pas configure ou pas encore disponible.
//
// TODO(amqp): creer AMQPBroker qui implemente messaging.Broker avec amqp091.
type NoopBroker struct{}

// NewNoopBroker retourne un NoopBroker (satisfait messaging.Broker).
func NewNoopBroker() *NoopBroker {
	return &NoopBroker{}
}

// Publish logue le message sans le transmettre reellement.
func (b *NoopBroker) Publish(_ context.Context, routingKey string, body []byte) error {
	log.Printf("[broker:noop] publish routingKey=%s body_len=%d", routingKey, len(body))
	return nil
}

// Consume logue et bloque jusqu'a l'annulation du contexte sans consommer de messages reels.
func (b *NoopBroker) Consume(ctx context.Context, queueName string, _ func([]byte) error) error {
	log.Printf("[broker:noop] consume queueName=%s (no-op, waiting for context cancellation)", queueName)
	<-ctx.Done()
	return nil
}
