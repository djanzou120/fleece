// Command wallet est le point d'entree (composition root) du Wallet Service.
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
	adapterhttp "fleece/src/wallet/internal/adapters/http"
	"fleece/src/wallet/internal/adapters/persistence"
	"fleece/src/wallet/internal/adapters/publisher"
	"fleece/src/wallet/internal/application/usecases"
	"fleece/src/wallet/internal/infrastructure/config"
	"fleece/src/wallet/internal/infrastructure/httpserver"
	"fleece/src/wallet/internal/infrastructure/postgres"
	"fleece/src/wallet/internal/infrastructure/rabbitmq"
)

func main() {
	app.Bootstrap("wallet")

	// --- Couche 4 : config ---
	cfg := config.Load()

	// --- Couche 4 : PostgreSQL ---
	db, err := postgres.Open(cfg.PostgresDriver, cfg.PostgresDSN, cfg.PostgresSearchPath)
	if err != nil {
		log.Printf("wallet: postgres unavailable (%v) — continuing without persistence (dev mode)", err)
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
	var walletRepo *persistence.WalletRepository
	var txnRepo *persistence.TransactionRepository
	if db != nil {
		walletRepo = persistence.NewWalletRepository(db.DB)
		txnRepo = persistence.NewTransactionRepository(db.DB)
	} else {
		walletRepo = persistence.NewWalletRepository(nil)
		txnRepo = persistence.NewTransactionRepository(nil)
	}

	_ = &http.Client{Timeout: 10 * time.Second} // conserve pour extension future (clients inter-services)

	eventPub := publisher.NewEventPublisher(broker)

	// --- Couche 2 : use cases (injection manuelle des ports) ---
	debitUC := &usecases.DebitWallet{
		Wallets:   walletRepo,
		Txns:      txnRepo,
		Publisher: eventPub,
	}
	creditUC := &usecases.CreditWallet{
		Wallets:   walletRepo,
		Txns:      txnRepo,
		Publisher: eventPub,
	}
	refundUC := &usecases.Refund{
		Wallets:   walletRepo,
		Txns:      txnRepo,
		Publisher: eventPub,
	}
	getBalanceUC := &usecases.GetBalance{
		Wallets: walletRepo,
	}

	// --- Couche 3 : adapter HTTP (driving) ---
	handler := adapterhttp.NewWalletHandler(debitUC, creditUC, refundUC, getBalanceUC)

	// --- Couche 4 : serveur HTTP ---
	srv := httpserver.New(":" + cfg.Port)
	srv.HandleFunc("GET /balance", handler.Balance)
	srv.HandleFunc("POST /debit", handler.Debit)
	srv.HandleFunc("POST /credit", handler.Credit)
	srv.HandleFunc("POST /refund", handler.RefundHandler)
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
		log.Printf("wallet: shutdown signal received")
	case err := <-srvErr:
		log.Printf("wallet: server error: %v", err)
	}

	// Arret gracieux du serveur HTTP (timeout 15 s).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("wallet: shutdown error: %v", err)
	}
}
