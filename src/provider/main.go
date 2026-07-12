// Command provider est le point d'entree (composition root) du Provider Service.
//
// Il cable l'infrastructure et les adapters dans les use cases en respectant
// la regle de dependance de la Clean Architecture (voir .ia/ARCHITECTURE.md).
//
// NOTE DRIVER : ce binaire n'importe pas de driver PostgreSQL concret.
// Pour un deploiement reel, ajouter dans ce fichier :
//
//	import _ "github.com/lib/pq"   // ou tout autre driver compatible database/sql
//
// apres avoir initialise go.sum avec la dependance correspondante.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fleece/src/go/app"
	adapterhttp "fleece/src/provider/internal/adapters/http"
	"fleece/src/provider/internal/adapters/persistence"
	"fleece/src/provider/internal/adapters/providers"
	"fleece/src/provider/internal/adapters/publisher"
	"fleece/src/provider/internal/application/ports/output"
	"fleece/src/provider/internal/application/usecases"
	"fleece/src/provider/internal/infrastructure/config"
	"fleece/src/provider/internal/infrastructure/httpserver"
	"fleece/src/provider/internal/infrastructure/postgres"
	"fleece/src/provider/internal/infrastructure/rabbitmq"
)

func main() {
	app.Bootstrap("provider")

	// --- Couche 4 : config ---
	cfg := config.Load()

	// --- Couche 4 : PostgreSQL ---
	db, err := postgres.Open(cfg.PostgresDriver, cfg.PostgresDSN, cfg.PostgresSearchPath)
	if err != nil {
		log.Printf("provider: postgres unavailable (%v) — continuing without persistence (dev mode)", err)
		// En dev, on continue sans base pour ne pas bloquer le demarrage.
		// En prod, supprimer ce fallback et faire os.Exit(1).
		db = nil
	}
	if db != nil {
		defer db.Close()
	}

	// --- Couche 4 : broker RabbitMQ (no-op par defaut) ---
	// TODO(amqp): remplacer NewNoopBroker par NewAMQPBroker(cfg.RabbitMQURL) quand disponible.
	broker := rabbitmq.NewNoopBroker()

	// --- Couche 3 : adapters driven ---

	// Repository de persistance.
	var providerRepo *persistence.ProviderRepository
	if db != nil {
		providerRepo = persistence.NewProviderRepository(db.DB)
	} else {
		providerRepo = persistence.NewProviderRepository(nil)
	}

	// Publisher d'evenements.
	eventPub := publisher.NewEventPublisher(broker)

	// Adapters fournisseurs (registry par providerId).
	// Le providerId est la cle utilisee par le service Routing lors de la selection.
	whatsappAdapter := providers.NewWhatsAppMeta(
		cfg.WhatsAppMetaBaseURL,
		cfg.WhatsAppMetaToken,
		cfg.WhatsAppMetaPhoneNumber,
	)
	smsAdapter := providers.NewSMSTwilio(
		cfg.TwilioBaseURL,
		cfg.TwilioAccountSID,
		cfg.TwilioAuthToken,
		cfg.TwilioFromNumber,
	)
	// Adapter Telegram Bot API.
	// Le bot token est charge depuis la config (env TELEGRAM_BOT_TOKEN).
	// TODO(production) D27 : migrer vers provider_credentials (AES-GCM, T-024 DevOps).
	telegramAdapter := providers.NewTelegramBot(
		cfg.TelegramBaseURL,
		cfg.TelegramBotToken,
	)

	// Registry : mappe chaque providerId a son implementation concrete.
	// Les clefs correspondent aux valeurs de provider.providers.id dans la base.
	// "telegram-bot" est seede dans migrations/0015_provider_telegram.sql (T-013).
	registry := map[string]output.Provider{
		"whatsapp-meta": whatsappAdapter,
		"sms-twilio":    smsAdapter,
		"telegram-bot":  telegramAdapter,
	}

	// --- Couche 2 : use cases (injection manuelle des ports) ---
	dispatchUC := &usecases.Dispatch{
		Registry:  registry,
		Repo:      providerRepo,
		Publisher: eventPub,
	}
	handleDLRUC := &usecases.HandleDLR{
		Publisher: eventPub,
		Repo:      providerRepo,
	}

	// --- Couche 3 : adapter HTTP (driving) ---
	handler := adapterhttp.NewProviderHandler(dispatchUC, handleDLRUC)

	// --- Couche 4 : serveur HTTP ---
	srv := httpserver.New(":" + cfg.Port)
	srv.HandleFunc("POST /dispatch", handler.Dispatch)
	srv.HandleFunc("POST /dlr", handler.DLR)
	// Route de sante minimale.
	srv.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Contexte d'arret gracieux.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Demarrer le serveur HTTP en arriere-plan.
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.Start()
	}()

	// Attendre SIGINT/SIGTERM ou une erreur du serveur.
	select {
	case <-ctx.Done():
		log.Printf("provider: shutdown signal received")
	case err := <-srvErr:
		log.Printf("provider: server error: %v", err)
	}

	// Arret gracieux du serveur HTTP (timeout 15 s).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("provider: shutdown error: %v", err)
	}
}
