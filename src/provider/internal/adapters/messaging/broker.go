// Package messaging fournit les abstractions de transport asynchrone (couche 3, Interface Adapters).
//
// L'interface Broker est definie ici — a l'interieur des adapters — pour respecter la regle de
// dependance de la Clean Architecture : les adapters (couche 3) ne peuvent pas importer
// l'infrastructure (couche 4). L'implementation concrete (NoopBroker, AMQPBroker…) vit en
// infrastructure et satisfait cette interface implicitement ; elle est injectee au composition root.
package messaging

import "context"

// Broker est l'abstraction minimale du transport de messages asynchrones.
// Les adapters publisher de couche 3 dependent de cette interface
// et jamais d'une bibliotheque AMQP concrete.
type Broker interface {
	// Publish envoie un message vers la queue (ou l'exchange) identifiee par routingKey.
	Publish(ctx context.Context, routingKey string, body []byte) error
	// Consume demarre la consommation de messages depuis la queue identifiee par queueName.
	// Le handler est appele pour chaque message recu. Bloque jusqu'a l'annulation du contexte.
	Consume(ctx context.Context, queueName string, handler func(body []byte) error) error
}
