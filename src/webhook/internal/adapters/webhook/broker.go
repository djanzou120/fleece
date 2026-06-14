// Package webhook fournit l'abstraction de transport asynchrone du service webhook
// (couche 3, Interface Adapters).
//
// L'interface Broker est definie ICI — dans les adapters (couche 3) — et non dans
// l'infrastructure (couche 4). Cela respecte la regle de dependance de la Clean
// Architecture : les adapters (C3) ne peuvent pas importer l'infrastructure (C4).
// L'implementation concrete (NoopBroker, AMQPBroker…) vit en C4
// (internal/infrastructure/rabbitmq/) et satisfait cette interface implicitement ;
// elle est injectee au composition root (main.go).
//
// Pattern identique a celui du service wallet (src/wallet/internal/adapters/messaging/broker.go).
package webhook

import "context"

// Broker est l'abstraction minimale du transport de messages asynchrones.
// Les adapters consumer et publisher de couche 3 dependent de cette interface
// et jamais d'une bibliotheque AMQP concrete.
type Broker interface {
	// Publish envoie un message vers la queue (ou l'exchange) identifiee par routingKey.
	Publish(ctx context.Context, routingKey string, body []byte) error

	// Consume demarre la consommation de messages depuis la queue identifiee par queueName.
	// Le handler est appele pour chaque message recu. Bloque jusqu'a l'annulation du contexte.
	Consume(ctx context.Context, queueName string, handler func(body []byte) error) error
}
