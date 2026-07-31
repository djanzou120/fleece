// Command api est le point d'entrée (composition root) du service HTTP unifié Fleece.
// Il ouvre les connexions d'infrastructure (PostgreSQL, RabbitMQ), injecte les
// dépendances manuellement dans Service, puis démarre le serveur HTTP sur :8080.
//
// Variables d'environnement requises :
//
//	POSTGRES_DSN  — DSN PostgreSQL (ex. postgres://user:pass@host:5432/fleece?sslmode=disable)
//	AMQP_DSN      — URL RabbitMQ    (ex. amqp://guest:guest@localhost:5672/)
//
// Variables d'environnement optionnelles (webhooks paiement — voir webhooks_om.go/
// webhooks_mtn.go pour la politique de validation appliquée si elles sont vides) :
//
//	OM_WEBHOOK_SECRET  — clé HMAC-SHA256 des callbacks Orange Money.
//	MTN_WEBHOOK_SECRET — clé HMAC-SHA256 des callbacks MTN Mobile Money.
//	FLEECE_ENV         — environnement d'exécution (E5, webhook_endpoints_create.go) :
//	                     seule une valeur "development"/"dev"/"test"/"local" autorise
//	                     l'exemption localhost/127.0.0.1 en HTTP à la création d'un
//	                     webhook-endpoint. Absente/vide = PRODUCTION (exemption désactivée,
//	                     comportement le plus sûr par défaut).
//
// Si une variable requise est absente ou si la connexion échoue, le programme log et
// retourne. En production, envisager os.Exit(1) à la place.
//
// Arrêt gracieux : le contexte racine (goapp.Context) est annulé sur SIGINT/SIGTERM ;
// le serveur HTTP est alors arrêté via http.Server.Shutdown avec un délai de drain
// (shutdownTimeout) pour laisser les requêtes en cours se terminer (D-M07 soldée).
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"fleece/src/api"
	"fleece/src/api/providers"
	goamqp "fleece/src/go/amqp"
	goapp "fleece/src/go/app"
	golog "fleece/src/go/log"
	gosql "fleece/src/go/sql"
)

// shutdownTimeout est le délai de drain accordé au serveur HTTP pour terminer les
// requêtes en cours lors d'un arrêt gracieux (SIGINT/SIGTERM).
const shutdownTimeout = 10 * time.Second

// fleeceExchange est le nom canonique de l'exchange AMQP topic durable
// partagé par TOUS les composition roots Go (src/api, core-processor,
// intelligence-processor, D-M29) — déclaré à l'identique (mêmes flags,
// durable=true) par chacun pour éviter tout PRECONDITION_FAILED côté broker
// en cas de divergence de paramètres entre binaires.
const fleeceExchange = "fleece"

func main() {
	// Contexte racine : annulé sur SIGINT/SIGTERM.
	ctx, cancel := goapp.Context()
	defer cancel()

	// Logger structuré (JSON en production, text en dev). Utilisé pour TOUS les
	// logs de ce composition root — plus de log.Printf stdlib (D-M07).
	logger := golog.Init("info", "text")
	logger.Info("api: starting", "version", goapp.Version)

	// PostgreSQL — DSN depuis l'environnement.
	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		logger.Error("api: POSTGRES_DSN is not set — cannot start")
		return
	}
	db, err := gosql.Open(postgresDSN)
	if err != nil {
		logger.Error("api: postgres unavailable", "err", err)
		return
	}
	defer db.Close()

	// RabbitMQ — DSN depuis l'environnement.
	amqpDSN := os.Getenv("AMQP_DSN")
	if amqpDSN == "" {
		logger.Error("api: AMQP_DSN is not set — cannot start")
		return
	}
	amqpConn := &goamqp.Conn{
		DSN:    amqpDSN,
		Logger: logger,
	}
	if err := amqpConn.Init(); err != nil {
		logger.Error("api: rabbitmq unavailable", "err", err)
		return
	}
	defer amqpConn.Close()

	// D-M29 (Phase 3) : declaration idempotente de l'exchange AMQP "fleece"
	// (topic, durable) — src/api est un PRODUCTEUR pur (messages_send.go,
	// webhooks_telegram.go publient dessus) mais ne consomme jamais de queue.
	// AVANT ce correctif, aucun ExchangeDeclare n'existait nulle part dans le
	// depot : si src/api demarrait avant tout worker (core-processor/
	// intelligence-processor, qui declarent aussi cet exchange, voir
	// goamqp/topology.go), il publiait dans un exchange inexistant — perte
	// silencieuse de tout evenement (voir aussi la fiabilisation de
	// goamqp.Conn.Publish, meme correctif D-M29). Memes parametres EXACTS
	// (topic, durable=true) que core-processor/intelligence-processor : toute
	// divergence provoquerait un PRECONDITION_FAILED cote broker.
	if err := amqpConn.DeclareExchange(fleeceExchange, "topic"); err != nil {
		logger.Error("api: declaration exchange AMQP echouee", "exchange", fleeceExchange, "err", err)
		return
	}

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
		// D-M17 (Phase 3) : remplace les anciens log.Printf stdlib des adapters
		// providers par le logger structure partage.
		Logger: logger,
	}

	// Secrets HMAC des webhooks paiement (écart B2 — plus de constante codée en dur).
	// Vides = validation désactivée avec log WARN par le handler (voir webhooks_om.go/
	// webhooks_mtn.go) : acceptable en dev/test uniquement, jamais en production.
	omWebhookSecret := os.Getenv("OM_WEBHOOK_SECRET")
	if omWebhookSecret == "" {
		logger.Warn("api: OM_WEBHOOK_SECRET non défini — callbacks Orange Money acceptés sans vérification (dev/test uniquement)")
	}
	mtnWebhookSecret := os.Getenv("MTN_WEBHOOK_SECRET")
	if mtnWebhookSecret == "" {
		logger.Warn("api: MTN_WEBHOOK_SECRET non défini — callbacks MTN MoMo acceptés sans vérification (dev/test uniquement)")
	}

	// FLEECE_ENV (E5) — voir doc de tête de fichier et Service.Env : absent/vide
	// = production (exemption localhost/127.0.0.1 désactivée par défaut).
	env := os.Getenv("FLEECE_ENV")

	// Composition root : injection manuelle des dépendances dans Service.
	svc := &api.Service{
		DB:               db,
		Logger:           logger,
		AMQP:             amqpConn,
		Providers:        providers.BuildRegistry(providerCfg),
		OMWebhookSecret:  omWebhookSecret,
		MTNWebhookSecret: mtnWebhookSecret,
		Env:              env,
	}
	if err := svc.Init(); err != nil {
		logger.Error("api: service init failed", "err", err)
		return
	}

	addr := ":8080"
	srv := &http.Server{
		Addr:    addr,
		Handler: svc,
	}

	// Démarrage du serveur HTTP dans une goroutine ; ListenAndServe bloque jusqu'à
	// Shutdown() (retourne alors http.ErrServerClosed) ou une erreur réseau fatale.
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("api: listening", "addr", addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api: server error", "err", err)
		}
	case <-ctx.Done():
		// SIGINT/SIGTERM : arrêt gracieux avec délai de drain (D-M07 soldée).
		logger.Info("api: shutdown signal received, draining connections", "timeout", shutdownTimeout)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("api: graceful shutdown failed", "err", err)
		} else {
			logger.Info("api: shutdown complete")
		}
	}
}
