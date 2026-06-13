// Command messaging est le point d'entree (composition root) du Messaging Service.
//
// Il cable l'infrastructure et les adapters dans les use cases en respectant
// la regle de dependance de la Clean Architecture (voir .ia/ARCHITECTURE.md).
//
// NOTE DRIVER : ce binaire n'importe pas de driver PostgreSQL concret.
// Pour un deploiement reel, ajouter dans ce fichier :
//   import _ "github.com/lib/pq"   // ou tout autre driver compatible database/sql
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
	"fleece/src/messaging/internal/adapters/consumer"
	adapterhttp "fleece/src/messaging/internal/adapters/http"
	"fleece/src/messaging/internal/adapters/clients"
	"fleece/src/messaging/internal/adapters/persistence"
	"fleece/src/messaging/internal/adapters/publisher"
	"fleece/src/messaging/internal/application/usecases"
	"fleece/src/messaging/internal/infrastructure/config"
	"fleece/src/messaging/internal/infrastructure/httpserver"
	"fleece/src/messaging/internal/infrastructure/postgres"
	"fleece/src/messaging/internal/infrastructure/rabbitmq"
)

func main() {
	app.Bootstrap("messaging")

	// --- Couche 4 : config ---
	cfg := config.Load()

	// --- Couche 4 : PostgreSQL ---
	db, err := postgres.Open(cfg.PostgresDriver, cfg.PostgresDSN, cfg.PostgresSearchPath)
	if err != nil {
		log.Printf("messaging: postgres unavailable (%v) — continuing without persistence (dev mode)", err)
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

	// --- Couche 3 : adapters driven (implementations des ports output) ---
	var repo *persistence.MessageRepository
	if db != nil {
		repo = persistence.NewMessageRepository(db.DB)
	} else {
		repo = persistence.NewMessageRepository(nil)
	}

	httpClientShared := &http.Client{Timeout: 10 * time.Second}

	routingGW := clients.NewRoutingClient(cfg.RoutingURL, httpClientShared)
	walletGW := clients.NewWalletClient(cfg.WalletURL, httpClientShared)
	providerGW := clients.NewProviderClient(cfg.ProviderURL, httpClientShared)
	eventPub := publisher.NewRabbitMQPublisher(broker)

	// --- Couche 2 : use case (injection manuelle des ports) ---
	sendMsgUC := &usecases.SendMessage{
		Repo:      repo,
		Routing:   routingGW,
		Wallet:    walletGW,
		Provider:  providerGW,
		Publisher: eventPub,
	}

	// --- Couche 3 : adapter HTTP (driving) ---
	handler := adapterhttp.NewMessagingHandler(sendMsgUC)

	// --- Couche 4 : serveur HTTP ---
	srv := httpserver.New(":" + cfg.Port)
	srv.HandleFunc("POST /messages", handler.SendMessage)
	// Route de sante minimale.
	srv.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// --- Couche 3 : consumer (driving) ---
	sendWorker := consumer.NewSendWorker(broker, sendMsgUC)

	// Contexte d'arret gracieux.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Demarrer le consumer en arriere-plan.
	go func() {
		if err := sendWorker.Start(ctx); err != nil {
			log.Printf("messaging: send_worker stopped: %v", err)
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
		log.Printf("messaging: shutdown signal received")
	case err := <-srvErr:
		log.Printf("messaging: server error: %v", err)
	}

	// Arret gracieux du serveur HTTP (timeout 15 s).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("messaging: shutdown error: %v", err)
	}
}
