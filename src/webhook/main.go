// Command webhook est le point d'entree (composition root) du Webhook Service.
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
	adapterhttp "fleece/src/webhook/internal/adapters/http"
	"fleece/src/webhook/internal/adapters/consumer"
	"fleece/src/webhook/internal/adapters/dispatcher"
	"fleece/src/webhook/internal/adapters/persistence"
	"fleece/src/webhook/internal/application/usecases"
	"fleece/src/webhook/internal/infrastructure/config"
	"fleece/src/webhook/internal/infrastructure/httpserver"
	"fleece/src/webhook/internal/infrastructure/postgres"
	"fleece/src/webhook/internal/infrastructure/rabbitmq"
)

func main() {
	app.Bootstrap("webhook")

	// --- Couche 4 : config ---
	cfg := config.Load()

	// --- Couche 4 : PostgreSQL ---
	db, err := postgres.Open(cfg.PostgresDriver, cfg.PostgresDSN, cfg.PostgresSearchPath)
	if err != nil {
		log.Printf("webhook: postgres unavailable (%v) — continuing without persistence (dev mode)", err)
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
	var endpointRepo *persistence.EndpointRepository
	var deliveryRepo *persistence.DeliveryRepository
	if db != nil {
		endpointRepo = persistence.NewEndpointRepository(db.DB)
		deliveryRepo = persistence.NewDeliveryRepository(db.DB)
	} else {
		endpointRepo = persistence.NewEndpointRepository(nil)
		deliveryRepo = persistence.NewDeliveryRepository(nil)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	httpDisp := dispatcher.New(httpClient)

	// --- Couche 2 : use cases (injection manuelle des ports) ---
	registerUC := &usecases.RegisterEndpoint{
		Endpoints: endpointRepo,
	}
	deliverUC := &usecases.DeliverEvent{
		Endpoints:  endpointRepo,
		Deliveries: deliveryRepo,
		Dispatcher: httpDisp,
	}
	retryUC := &usecases.RetryDelivery{
		Deliveries: deliveryRepo,
		Endpoints:  endpointRepo,
		Dispatcher: httpDisp,
	}

	// --- Couche 3 : consumer d'evenements (driving) ---
	eventConsumer := consumer.NewEventConsumer(broker, deliverUC)

	// --- Couche 3 : adapter HTTP (driving) ---
	handler := adapterhttp.NewWebhookHandler(registerUC, endpointRepo)

	// --- Couche 4 : serveur HTTP ---
	srv := httpserver.New(":" + cfg.Port)
	srv.HandleFunc("POST /endpoints", handler.Register)
	srv.HandleFunc("GET /endpoints", handler.List)
	srv.HandleFunc("DELETE /endpoints/{id}", handler.Delete)
	// Route de sante minimale.
	srv.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Contexte d'arret gracieux.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Demarrer le consumer d'evenements en arriere-plan.
	go func() {
		if err := eventConsumer.Start(ctx); err != nil {
			log.Printf("webhook: consumer error: %v", err)
		}
	}()

	// Scheduler de retry : toutes les 60 s, relance les livraisons en echec dont
	// next_retry_at est echu. Si la base est indisponible (db==nil), le repo
	// retourne une liste vide sans paniquer.
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				pending, err := deliveryRepo.FindPendingRetries(ctx, t)
				if err != nil {
					log.Printf("webhook: retry scheduler: FindPendingRetries: %v", err)
					continue
				}
				for _, d := range pending {
					if err := retryUC.Execute(ctx, d.ID); err != nil {
						log.Printf("webhook: retry scheduler: delivery %d: %v", d.ID, err)
					}
				}
			}
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
		log.Printf("webhook: shutdown signal received")
	case err := <-srvErr:
		log.Printf("webhook: server error: %v", err)
	}

	// Arret gracieux du serveur HTTP (timeout 15 s).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("webhook: shutdown error: %v", err)
	}
}
