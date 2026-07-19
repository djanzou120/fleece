// Command api est le point d'entrée (composition root) du service HTTP unifié Fleece.
// Il ouvre les connexions d'infrastructure (PostgreSQL, RabbitMQ), injecte les
// dépendances manuellement dans Service, puis démarre le serveur HTTP sur :8080.
//
// Variables d'environnement requises :
//
//	POSTGRES_DSN  — DSN PostgreSQL (ex. postgres://user:pass@host:5432/fleece?sslmode=disable)
//	AMQP_DSN      — URL RabbitMQ    (ex. amqp://guest:guest@localhost:5672/)
//
// Si une variable est absente ou si la connexion échoue, le programme log et retourne.
// En production, envisager os.Exit(1) à la place.
package main

import (
	"log"
	"net/http"
	"os"

	"fleece/src/api"
	"fleece/src/api/providers"
	goamqp "fleece/src/go/amqp"
	goapp "fleece/src/go/app"
	golog "fleece/src/go/log"
	gosql "fleece/src/go/sql"
)

func main() {
	// Contexte racine : annulé sur SIGINT/SIGTERM.
	ctx, cancel := goapp.Context()
	defer cancel()
	_ = ctx

	// Logger structuré (JSON en production, text en dev).
	logger := golog.Init("info", "text")
	logger.Info("api: starting", "version", goapp.Version)

	// PostgreSQL — DSN depuis l'environnement.
	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		log.Printf("api: POSTGRES_DSN is not set — cannot start")
		return
	}
	db, err := gosql.Open(postgresDSN)
	if err != nil {
		log.Printf("api: postgres unavailable: %v", err)
		return
	}
	defer db.Close()

	// RabbitMQ — DSN depuis l'environnement.
	amqpDSN := os.Getenv("AMQP_DSN")
	if amqpDSN == "" {
		log.Printf("api: AMQP_DSN is not set — cannot start")
		return
	}
	amqpConn := &goamqp.Conn{
		DSN:    amqpDSN,
		Logger: logger,
	}
	if err := amqpConn.Init(); err != nil {
		log.Printf("api: rabbitmq unavailable: %v", err)
		return
	}
	defer amqpConn.Close()

	// Providers : construction du registry depuis les variables d'environnement.
	// Les valeurs vides sont acceptées (stub — aucun appel réel en dev).
	// TODO(production): valider que les credentials sont présents au démarrage.
	providerCfg := providers.ProviderConfig{
		TwilioBaseURL:           os.Getenv("TWILIO_BASE_URL"),
		TwilioAccountSID:        os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:         os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioFromNumber:        os.Getenv("TWILIO_FROM_NUMBER"),
		WhatsAppMetaBaseURL:     os.Getenv("WHATSAPP_META_BASE_URL"),
		WhatsAppMetaToken:       os.Getenv("WHATSAPP_META_TOKEN"),
		WhatsAppMetaPhoneNumber: os.Getenv("WHATSAPP_META_PHONE_NUMBER_ID"),
		TelegramBaseURL:         os.Getenv("TELEGRAM_BASE_URL"),
		TelegramBotToken:        os.Getenv("TELEGRAM_BOT_TOKEN"),
	}

	// Composition root : injection manuelle des dépendances dans Service.
	svc := &api.Service{
		DB:        db,
		Logger:    logger,
		AMQP:      amqpConn,
		Providers: providers.BuildRegistry(providerCfg),
	}
	if err := svc.Init(); err != nil {
		log.Printf("api: service init failed: %v", err)
		return
	}

	addr := ":8080"
	logger.Info("api: listening", "addr", addr)
	if err := http.ListenAndServe(addr, svc); err != nil {
		log.Printf("api: server error: %v", err)
	}
}
