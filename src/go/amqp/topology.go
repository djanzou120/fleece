package goamqp

// topology.go — déclaration idempotente de la topologie RabbitMQ (D-M29,
// Phase 3, correctif issu de la revue d'architecture).
//
// AVANT ce correctif, aucun ExchangeDeclare/QueueDeclare/QueueBind n'existait
// nulle part dans le dépôt (ni Go, ni deploy/) : l'exchange "fleece" et les
// queues "core-processor"/"intelligence-processor" (avec leurs bindings
// distincts, voir src/core-processor/consumer.go et
// src/intelligence-processor/consumer.go) n'existaient qu'en théorie —
// Publish/Consume échouaient (ou pire, échouaient de façon asynchrone et
// silencieuse, voir conn.go) contre un exchange/une queue jamais créés.
//
// CORRECTIF : chaque composition root (src/api/cmd/api/main.go,
// src/core-processor/cmd/core-processor/main.go,
// src/intelligence-processor/cmd/intelligence-processor/main.go) déclare,
// AU DÉMARRAGE, la topologie dont IL A BESOIN via les helpers ci-dessous :
//   - TOUS déclarent l'exchange "fleece" (topic, durable) — y compris src/api
//     qui ne fait QUE publier : sans cette déclaration côté producteur, un
//     démarrage de src/api avant tout worker publierait dans le vide.
//   - core-processor déclare SA queue "core-processor" (durable) et la lie
//     aux routing keys qu'il consomme réellement — voir
//     coreprocessor.BoundRoutingKeys(), qui les dérive de
//     handlersByRoutingKey (consumer.go) : la binding ne peut donc JAMAIS
//     diverger de ce que dispatch() route effectivement.
//   - intelligence-processor fait de même pour sa propre queue
//     "intelligence-processor" via intelligenceprocessor.BoundRoutingKeys().
//
// IDEMPOTENCE ET DIVERGENCE DE PARAMÈTRES : ExchangeDeclare/QueueDeclare sont
// idempotents SI ET SEULEMENT SI les paramètres (durable, autoDelete,
// exclusive, arguments) sont IDENTIQUES à chaque appel — sinon le broker
// répond PRECONDITION_FAILED (406) et ferme le canal. Les trois composition
// roots DOIVENT donc appeler EXACTEMENT les mêmes helpers avec les mêmes
// arguments pour l'exchange "fleece" (topic, durable) : c'est précisément ce
// que factorisent DeclareExchange/DeclareQueue/BindQueue ci-dessous — aucun
// composition root n'appelle directement amqp091.Channel.ExchangeDeclare/
// QueueDeclare avec ses propres flags.
// DeclareExchange déclare (de façon idempotente) un exchange durable de type
// kind ("topic" pour "fleece" — voir doc de tête de fichier). Ouvre puis
// ferme un canal dédié, comme [Conn.Publish]/[Conn.Consume] : les
// déclarations de topologie sont rares (une fois au démarrage), le coût
// d'ouverture d'un canal y est négligeable.
func (c *Conn) DeclareExchange(name, kind string) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return ch.ExchangeDeclare(
		name,
		kind,
		true,  // durable
		false, // autoDelete
		false, // internal
		false, // noWait
		nil,   // args
	)
}

// DeclareQueue déclare (de façon idempotente) une queue durable, non
// exclusive, non auto-delete — le worker qui la consomme (core-processor,
// intelligence-processor) doit continuer à la trouver après un redémarrage du
// broker, et elle n'est jamais liée à une connexion/consumer en particulier.
func (c *Conn) DeclareQueue(name string) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		name,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	return err
}

// BindQueue lie queue à exchange pour chacune des routing keys fournies.
// Appelée après [Conn.DeclareQueue] (la queue doit exister avant d'être
// liée). keys vide est un no-op (aucun appel réseau).
func (c *Conn) BindQueue(queue, exchange string, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	for _, key := range keys {
		if err := ch.QueueBind(queue, key, exchange, false, nil); err != nil {
			return err
		}
	}
	return nil
}

// EnsureQueueBound est un raccourci pour DeclareQueue puis BindQueue,
// utilisé par les composition roots des workers (core-processor,
// intelligence-processor) qui possèdent chacun UNE queue à déclarer/lier.
// exchange DOIT avoir déjà été déclaré (voir [Conn.DeclareExchange]) — cette
// fonction ne le déclare pas elle-même : les trois composition roots
// partagent le MÊME appel DeclareExchange("fleece", "topic") pour garantir
// l'absence de divergence de paramètres (voir doc de tête de fichier).
func (c *Conn) EnsureQueueBound(queue, exchange string, keys ...string) error {
	if err := c.DeclareQueue(queue); err != nil {
		return err
	}
	return c.BindQueue(queue, exchange, keys...)
}
