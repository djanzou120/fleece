// Package goamqp fournit une connexion RabbitMQ partagée entre les services Go.
// Il enveloppe [github.com/rabbitmq/amqp091-go] et est compatible avec le
// système de configuration par struct tags (key:"" / inject:"").
// Cette lib est transverse : elle ne contient AUCUNE règle métier.
package goamqp

import (
	"context"

	golog "fleece/src/go/log"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// Conn est une connexion RabbitMQ partagée, compatible zconfig (struct tags).
// Les champs exportés sont renseignés par injection de dépendances avant
// l'appel à [Conn.Init].
type Conn struct {
	// DSN est l'URL de connexion RabbitMQ au format amqp://user:pass@host:port/vhost.
	DSN string `key:"dsn"`
	// Logger est injecté par le conteneur DI. Peut être nil.
	Logger *golog.Logger `inject:"logger"`

	conn *amqp091.Connection
}

// Init établit la connexion AMQP en utilisant [Conn.DSN].
// Si un Logger est disponible, il est enrichi du module "amqp".
// Retourne une erreur si la connexion échoue.
func (c *Conn) Init() error {
	if c.Logger != nil {
		c.Logger = c.Logger.With("module", "amqp")
	}

	conn, err := amqp091.Dial(c.DSN)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// Channel ouvre et retourne un nouveau canal AMQP depuis la connexion active.
func (c *Conn) Channel() (*amqp091.Channel, error) {
	return c.conn.Channel()
}

// Publish publie un message JSON sur l'exchange donné avec la routing key
// indiquée. Un canal est ouvert puis fermé à chaque appel.
// Les flags mandatory et immediate sont positionnés à false.
func (c *Conn) Publish(ctx context.Context, exchange, key string, body []byte) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return ch.PublishWithContext(
		ctx,
		exchange,
		key,
		false, // mandatory
		false, // immediate
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// Consume enregistre un consumer sur la queue indiquée et retourne le canal
// de delivery. Le canal AMQP reste ouvert pour la durée de vie du flux.
// autoAck est positionné à false : les messages doivent être acquittés
// manuellement par le consommateur.
func (c *Conn) Consume(queue string) (<-chan amqp091.Delivery, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}

	return ch.Consume(
		queue,
		"",    // consumer tag (vide = auto-généré)
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
}

// Close ferme la connexion AMQP sous-jacente si elle est active.
func (c *Conn) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
