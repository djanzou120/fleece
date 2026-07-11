// Command contact-intelligence est le point d'entree (composition root) du service
// Contact Intelligence.
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
//
// Port d'ecoute par defaut : 8086.
// Convention des ports : messaging=8081, wallet=8082, routing=8083, provider=8084,
//
//	webhook=8085, contact-intelligence=8086.
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
	adapterhttp "fleece/src/contact-intelligence/internal/adapters/http"
	"fleece/src/contact-intelligence/internal/adapters/messaging"
	"fleece/src/contact-intelligence/internal/adapters/persistence"
	"fleece/src/contact-intelligence/internal/application/usecases"
	"fleece/src/contact-intelligence/internal/infrastructure/config"
	"fleece/src/contact-intelligence/internal/infrastructure/httpserver"
	"fleece/src/contact-intelligence/internal/infrastructure/postgres"
	"fleece/src/contact-intelligence/internal/infrastructure/rabbitmq"
)

func main() {
	app.Bootstrap("contact-intelligence")

	// --- Couche 4 : config ---
	cfg := config.Load()

	// --- Couche 4 : PostgreSQL ---
	db, err := postgres.Open(cfg.PostgresDriver, cfg.PostgresDSN, cfg.PostgresSearchPath)
	if err != nil {
		log.Printf("contact-intelligence: postgres unavailable (%v) — continuing without persistence (dev mode)", err)
		db = nil
	}
	if db != nil {
		defer db.Close()
	}

	// --- Couche 4 : broker RabbitMQ (no-op par defaut) ---
	// TODO(amqp): remplacer NewNoopBroker par NewAMQPBroker(cfg.RabbitMQURL) quand disponible.
	broker := rabbitmq.NewNoopBroker()

	// --- Couche 3 : adapters driven (implementations des ports output) ---
	var contactRepo *persistence.ContactRepository
	var channelScoreRepo *persistence.ChannelScoreRepository
	var outcomeStore *persistence.OutcomeStore

	if db != nil {
		contactRepo = persistence.NewContactRepository(db.DB)
		channelScoreRepo = persistence.NewChannelScoreRepository(db.DB)
		outcomeStore = persistence.NewOutcomeStore(db.DB)
	} else {
		contactRepo = persistence.NewContactRepository(nil)
		channelScoreRepo = persistence.NewChannelScoreRepository(nil)
		outcomeStore = persistence.NewOutcomeStore(nil)
	}

	// --- Couche 2 : use cases (injection manuelle des ports) ---
	getScoreUC := &usecases.GetContactScore{
		Contacts:      contactRepo,
		ChannelScores: channelScoreRepo,
	}

	recordOutcomeUC := &usecases.RecordDeliveryOutcome{
		Contacts:      contactRepo,
		ChannelScores: channelScoreRepo,
		Store:         outcomeStore,
	}

	enrichUC := &usecases.EnrichRecipient{
		Contacts:      contactRepo,
		ChannelScores: channelScoreRepo,
	}
	_ = enrichUC // utilise par des consommateurs futurs ou extensions HTTP

	// --- Couche 3 : consumer DLR (driving) ---
	dlrConsumer := messaging.NewDLRConsumer(broker, cfg.QueueDLR, recordOutcomeUC)

	// --- Couche 3 : adapter HTTP (driving) ---
	handler := adapterhttp.NewContactIntelHandler(getScoreUC, recordOutcomeUC)

	// --- Couche 4 : serveur HTTP ---
	srv := httpserver.New(":" + cfg.Port)
	srv.HandleFunc("GET /contacts/{phone}/score", handler.GetScore)
	srv.HandleFunc("POST /outcomes", handler.RecordOutcome)
	srv.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Contexte d'arret gracieux.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Demarrer le consumer DLR en arriere-plan.
	go func() {
		if err := dlrConsumer.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("contact-intelligence: dlr consumer error: %v", err)
		}
	}()

	// Demarrer le serveur HTTP en arriere-plan.
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.Start()
	}()

	// Attendre SIGINT/SIGTERM ou une erreur du serveur.
	select {
	case <-ctx.Done():
		log.Printf("contact-intelligence: shutdown signal received")
	case err := <-srvErr:
		log.Printf("contact-intelligence: server error: %v", err)
	}

	// Arret gracieux du serveur HTTP (timeout 15 s).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("contact-intelligence: shutdown error: %v", err)
	}
}
