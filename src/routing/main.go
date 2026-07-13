// Command routing est le point d'entree (composition root) du Routing Service.
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
	"fleece/src/routing/internal/adapters/clients"
	adapterhttp "fleece/src/routing/internal/adapters/http"
	"fleece/src/routing/internal/adapters/persistence"
	"fleece/src/routing/internal/application/ports/output"
	"fleece/src/routing/internal/application/usecases"
	"fleece/src/routing/internal/infrastructure/config"
	"fleece/src/routing/internal/infrastructure/httpserver"
	"fleece/src/routing/internal/infrastructure/postgres"
)

func main() {
	app.Bootstrap("routing")

	// --- Couche 4 : config ---
	cfg := config.Load()

	// --- Couche 4 : PostgreSQL ---
	db, err := postgres.Open(cfg.PostgresDriver, cfg.PostgresDSN, cfg.PostgresSearchPath)
	if err != nil {
		log.Printf("routing: postgres unavailable (%v) — continuing without persistence (dev mode)", err)
		// En dev, on continue sans base pour ne pas bloquer le demarrage.
		// En prod, supprimer ce fallback et faire os.Exit(1).
		db = nil
	}
	if db != nil {
		defer db.Close()
	}

	// --- Couche 3 : adapters driven (implementations des ports output) ---
	var pricingRepo *persistence.PricingRepository
	var scoreRepo *persistence.ScoreRepository
	var ruleRepo *persistence.RuleRepository

	var sqlDB interface{ Close() error }
	if db != nil {
		sqlDB = db
		pricingRepo = persistence.NewPricingRepository(db.DB, cfg.DefaultCurrency)
		scoreRepo = persistence.NewScoreRepository(db.DB)
		ruleRepo = persistence.NewRuleRepository(db.DB)
	} else {
		pricingRepo = persistence.NewPricingRepository(nil, cfg.DefaultCurrency)
		scoreRepo = persistence.NewScoreRepository(nil)
		ruleRepo = persistence.NewRuleRepository(nil)
	}
	if sqlDB != nil {
		defer sqlDB.Close()
	}

	// --- Couche 3 : client REST contact-intelligence (driven) ---
	// Si CONTACT_INTEL_API_URL est vide, on injecte nil : le use case degrade
	// gracieusement vers provider_scores seuls (aucun enrichissement contact).
	var contactScoreClient output.ContactScoreClient
	if cfg.ContactIntelAPIURL != "" {
		contactScoreClient = clients.NewContactIntelClient(cfg.ContactIntelAPIURL)
		log.Printf("routing: contact-intelligence active sur %s", cfg.ContactIntelAPIURL)
	} else {
		log.Printf("routing: CONTACT_INTEL_API_URL non configure — enrichissement contact desactive")
	}

	// --- Couche 2 : use cases (injection manuelle des ports) ---
	getDecisionUC := &usecases.GetRoutingDecision{
		Pricing:      pricingRepo,
		Scores:       scoreRepo,
		Rules:        ruleRepo,
		ContactScore: contactScoreClient, // nil si CONTACT_INTEL_API_URL non configure
	}
	updateScoreUC := &usecases.UpdateProviderScore{
		Scores: scoreRepo,
	}

	// --- Couche 3 : adapter HTTP (driving) ---
	handler := adapterhttp.NewRoutingHandler(getDecisionUC, updateScoreUC)

	// --- Couche 4 : serveur HTTP ---
	srv := httpserver.New(":" + cfg.Port)
	srv.HandleFunc("POST /route", handler.Route)
	srv.HandleFunc("POST /scores", handler.ScoreFeedback)
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
		log.Printf("routing: shutdown signal received")
	case err := <-srvErr:
		log.Printf("routing: server error: %v", err)
	}

	// Arret gracieux du serveur HTTP (timeout 15 s).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("routing: shutdown error: %v", err)
	}
}
