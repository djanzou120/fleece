// Command campaign est le point d'entree (composition root) du Campaign Service.
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
// Port d'ecoute par defaut : 8087.
// Convention des ports : messaging=8081, wallet=8082, routing=8083, provider=8084,
//
//	webhook=8085, contact-intelligence=8086, campaign=8087.
//
// Scheduler :
// Le scheduler est une goroutine ticker (intervalle defaut 30s).
// Il appelle ListScheduledDue puis UpdateStatusAtomic (anti-double-run) pour chaque campagne due.
// Si rowsAffected == 0, un autre pod a deja pris la campagne → skip silencieux.
// S'arrete sur ctx.Done(), erreurs loggees sans panic.
//
// Rate limiting :
// Le rate limit est par run, base sur campaigns.rate_limit (messages/min).
// 0 = illimite. Implemente par time.Sleep dans RunCampaign (token bucket simple stdlib).
// Voir RunCampaign.Execute pour le detail.
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

	adapterhttp "fleece/src/campaign/internal/adapters/http"
	"fleece/src/campaign/internal/adapters/clients"
	"fleece/src/campaign/internal/adapters/messaging"
	"fleece/src/campaign/internal/adapters/persistence"
	"fleece/src/campaign/internal/application/usecases"
	"fleece/src/campaign/internal/infrastructure/config"
	"fleece/src/campaign/internal/infrastructure/httpserver"
	"fleece/src/campaign/internal/infrastructure/postgres"
	"fleece/src/campaign/internal/infrastructure/rabbitmq"
)

func main() {
	app.Bootstrap("campaign")

	// --- Couche 4 : config ---
	cfg := config.Load()

	// --- Couche 4 : PostgreSQL ---
	db, err := postgres.Open(cfg.PostgresDriver, cfg.PostgresDSN, cfg.PostgresSearchPath)
	if err != nil {
		log.Printf("campaign: postgres unavailable (%v) — continuing without persistence (dev mode)", err)
		db = nil
	}
	if db != nil {
		defer db.Close()
	}

	// --- Couche 4 : broker RabbitMQ (no-op par defaut) ---
	// TODO(amqp): remplacer NewNoopBroker par NewAMQPBroker(cfg.RabbitMQURL) quand disponible.
	broker := rabbitmq.NewNoopBroker()

	// --- Couche 3 : adapters driven (implementations des ports output) ---
	var sqlDB interface{ Close() error }
	var campaignRepo *persistence.CampaignRepository
	var recipientRepo *persistence.RecipientRepository
	var runRepo *persistence.RunRepository

	if db != nil {
		sqlDB = db
		campaignRepo = persistence.NewCampaignRepository(db.DB)
		recipientRepo = persistence.NewRecipientRepository(db.DB)
		runRepo = persistence.NewRunRepository(db.DB)
	} else {
		campaignRepo = persistence.NewCampaignRepository(nil)
		recipientRepo = persistence.NewRecipientRepository(nil)
		runRepo = persistence.NewRunRepository(nil)
	}
	if sqlDB != nil {
		defer sqlDB.Close()
	}

	// --- Couche 3 : clients REST inter-services (driven) ---
	// Si ROUTING_API_URL est vide, on injecte nil et le use case estime a 0.
	var routingCl *clients.RoutingClient
	if cfg.RoutingAPIURL != "" {
		routingCl = clients.NewRoutingClient(cfg.RoutingAPIURL)
		log.Printf("campaign: routing active sur %s", cfg.RoutingAPIURL)
	} else {
		log.Printf("campaign: ROUTING_API_URL non configure — estimation de cout desactivee")
	}

	// Si MESSAGING_API_URL est vide, l'envoi de messages sera non operationnel.
	var messagingCl *clients.MessagingClient
	if cfg.MessagingAPIURL != "" {
		messagingCl = clients.NewMessagingClient(cfg.MessagingAPIURL)
		log.Printf("campaign: messaging active sur %s", cfg.MessagingAPIURL)
	} else {
		log.Printf("campaign: MESSAGING_API_URL non configure — envoi de messages non operationnel")
	}

	// --- Couche 2 : use cases (injection manuelle des ports) ---
	createCampaignUC := &usecases.CreateCampaign{
		Campaigns: campaignRepo,
	}
	importRecipientsUC := &usecases.ImportRecipients{
		Recipients: recipientRepo,
	}
	estimateCostUC := &usecases.EstimateCost{
		Campaigns:  campaignRepo,
		Recipients: recipientRepo,
		Routing:    routingCl,
	}
	scheduleCampaignUC := &usecases.ScheduleCampaign{
		Campaigns: campaignRepo,
	}
	runCampaignUC := &usecases.RunCampaign{
		Campaigns:  campaignRepo,
		Recipients: recipientRepo,
		Runs:       runRepo,
		Messaging:  messagingCl,
	}
	pauseCampaignUC := &usecases.PauseCampaign{
		Campaigns: campaignRepo,
	}
	cancelCampaignUC := &usecases.CancelCampaign{
		Campaigns: campaignRepo,
	}
	getCampaignStatusUC := &usecases.GetCampaignStatus{
		Campaigns: campaignRepo,
	}
	recordDeliveryOutcomeUC := &usecases.RecordDeliveryOutcome{
		Recipients: recipientRepo,
	}

	// --- Couche 3 : consumer DLR (driving) ---
	dlrConsumer := messaging.NewDLRConsumer(broker, cfg.QueueDLR, recordDeliveryOutcomeUC)

	// --- Couche 3 : adapter HTTP (driving) ---
	handler := adapterhttp.NewCampaignHandler(
		createCampaignUC,
		importRecipientsUC,
		estimateCostUC,
		scheduleCampaignUC,
		runCampaignUC,
		pauseCampaignUC,
		cancelCampaignUC,
		getCampaignStatusUC,
	)

	// --- Couche 4 : serveur HTTP ---
	srv := httpserver.New(":" + cfg.Port)
	srv.HandleFunc("POST /campaigns", handler.CreateCampaign)
	srv.HandleFunc("POST /campaigns/{id}/recipients", handler.ImportRecipients)
	srv.HandleFunc("POST /campaigns/{id}/estimate", handler.EstimateCost)
	srv.HandleFunc("POST /campaigns/{id}/schedule", handler.ScheduleCampaign)
	srv.HandleFunc("POST /campaigns/{id}/run", handler.RunCampaign)
	srv.HandleFunc("POST /campaigns/{id}/pause", handler.PauseCampaign)
	srv.HandleFunc("POST /campaigns/{id}/cancel", handler.CancelCampaign)
	srv.HandleFunc("GET /campaigns/{id}", handler.GetCampaignStatus)
	srv.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Contexte d'arret gracieux.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Demarrer le consumer DLR en arriere-plan.
	go func() {
		if err := dlrConsumer.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("campaign: dlr consumer error: %v", err)
		}
	}()

	// Demarrer le scheduler en arriere-plan.
	// Le scheduler interroge ListScheduledDue toutes les <SchedulerInterval> secondes.
	// Anti-double-run : UpdateStatusAtomic(scheduled → running) retourne 0 si deja pris.
	go runScheduler(ctx, cfg.SchedulerInterval, campaignRepo, runCampaignUC)

	// Demarrer le serveur HTTP en arriere-plan.
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.Start()
	}()

	// Attendre SIGINT/SIGTERM ou une erreur du serveur.
	select {
	case <-ctx.Done():
		log.Printf("campaign: shutdown signal received")
	case err := <-srvErr:
		log.Printf("campaign: server error: %v", err)
	}

	// Arret gracieux du serveur HTTP (timeout 15 s).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("campaign: shutdown error: %v", err)
	}
}

// runScheduler est la goroutine du scheduler de campagnes.
//
// Toutes les <intervalSec> secondes :
//  1. Appelle ListScheduledDue (status='scheduled' AND scheduled_at <= now()).
//  2. Pour chaque campagne due, appelle RunCampaign.Execute.
//     RunCampaign fait UpdateStatusAtomic(scheduled → running) — si rowsAffected==0,
//     un autre pod a deja pris la campagne (anti-double-run).
//  3. S'arrete proprement sur ctx.Done().
//  4. Erreurs loggees sans panic, no-panic si db==nil (repo devmode retourne nil).
func runScheduler(ctx context.Context, intervalSec int, campaigns *persistence.CampaignRepository, runUC *usecases.RunCampaign) {
	if intervalSec <= 0 {
		intervalSec = 30
	}
	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	log.Printf("campaign: scheduler demarre (intervalle=%ds)", intervalSec)

	for {
		select {
		case <-ctx.Done():
			log.Printf("campaign: scheduler arrete")
			return
		case <-ticker.C:
			schedulerTick(ctx, campaigns, runUC)
		}
	}
}

// schedulerTick execute un tick du scheduler.
func schedulerTick(ctx context.Context, campaigns *persistence.CampaignRepository, runUC *usecases.RunCampaign) {
	due, err := campaigns.ListScheduledDue(ctx)
	if err != nil {
		log.Printf("campaign: scheduler ListScheduledDue: %v", err)
		return
	}

	for _, c := range due {
		c := c // capture de boucle
		// Executer chaque campagne dans une goroutine dediee pour ne pas bloquer le scheduler.
		go func() {
			if err := runUC.Execute(ctx, c.ID); err != nil && ctx.Err() == nil {
				log.Printf("campaign: scheduler RunCampaign %s: %v", c.ID, err)
			}
		}()
	}
}
