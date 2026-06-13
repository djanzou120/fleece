// Package messaging fournit les abstractions de transport asynchrone (couche 3, Interface Adapters).
//
// L'interface Broker est définie ici — à l'intérieur des adapters — pour respecter la règle de
// dépendance de la Clean Architecture : les adapters (couche 3) ne peuvent pas importer
// l'infrastructure (couche 4). L'implémentation concrète (NoopBroker, AMQPBroker…) vit en
// infrastructure et satisfait cette interface implicitement ; elle est injectée au composition root.
package messaging

import "context"

// Broker est l'abstraction minimale du transport de messages asynchrones.
// Les adapters publisher et consumer de couche 3 dépendent de cette interface
// et jamais d'une bibliothèque AMQP concrète.
type Broker interface {
	// Publish envoie un message vers la queue (ou l'exchange) identifiée par routingKey.
	Publish(ctx context.Context, routingKey string, body []byte) error
	// Consume démarre la consommation de messages depuis la queue identifiée par queueName.
	// Le handler est appelé pour chaque message reçu. Bloque jusqu'à l'annulation du contexte.
	Consume(ctx context.Context, queueName string, handler func(body []byte) error) error
}
