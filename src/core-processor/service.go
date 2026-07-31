// Package coreprocessor est le worker Go qui consomme les evenements DLR
// (Delivery Report) de messagerie ("message.delivered", "message.failed")
// publies par src/api sur RabbitMQ, et applique leurs effets sur la base
// PostgreSQL partagee : transition du statut du message et, le cas echeant,
// remboursement du wallet.
//
// Architecture : meme pattern que src/api — structure plate, accès DB direct
// (s.DB.Select/Get/Exec/ExecRows/Begin), pas de repository/port/use case.
// Contrainte Go "1 repertoire = 1 package" (precedent D-M09, Phase 1) : le
// composition root vit dans src/core-processor/cmd/core-processor/main.go
// (package main), le reste ici en package coreprocessor.
package coreprocessor

import (
	"net/http"

	goamqp "fleece/src/go/amqp"
	golog "fleece/src/go/log"
	gosql "fleece/src/go/sql"
)

// Service est la racine du worker core-processor.
// Les champs exportes sont injectes manuellement par le composition root
// (cmd/core-processor/main.go), a l'identique du pattern src/api/service.go.
type Service struct {
	// DB est la connexion PostgreSQL partagee (memes schemas que src/api :
	// messaging, wallet — voir D-M01, prefixer chaque table).
	DB *gosql.DB
	// Logger est le logger structure partage.
	Logger *golog.Logger
	// AMQP est la connexion RabbitMQ partagee, utilisee uniquement en
	// consommation ici (Consume) — ce worker NE publie AUCUN evenement
	// (decision PM, voir on_message_failed.go : pas de "wallet.refund").
	AMQP *goamqp.Conn

	// Queue est le nom de la queue RabbitMQ consommee. Configurable via la
	// variable d'environnement CORE_PROCESSOR_QUEUE (voir cmd/core-processor/main.go).
	// Defaut : "core-processor".
	Queue string

	// WebhookHTTPClient est le client HTTP utilise pour livrer les webhooks
	// sortants (D-M43, webhook_dispatch.go/webhook_retry_scheduler.go).
	// Defaut (Init()) : timeout 10s, voir defaultWebhookHTTPClient()
	// (webhook_dispatch.go). Injectable explicitement par les tests pour
	// exercer un timeout court sans attendre 10 secondes reelles.
	WebhookHTTPClient *http.Client
}

// Init verifie que les dependances obligatoires sont presentes et applique
// les valeurs par defaut. Retourne une erreur si une dependance requise est
// absente — appele explicitement par le composition root avant Run.
func (s *Service) Init() error {
	if s.Queue == "" {
		s.Queue = "core-processor"
	}
	if s.WebhookHTTPClient == nil {
		s.WebhookHTTPClient = defaultWebhookHTTPClient()
	}
	return nil
}
