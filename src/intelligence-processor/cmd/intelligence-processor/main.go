// Command intelligence-processor est le point d'entree (composition root) du
// worker intelligence-processor. Il ouvre les connexions d'infrastructure
// (PostgreSQL, RabbitMQ), declare la topologie AMQP dont il a besoin (D-M29),
// injecte les dependances manuellement dans intelligenceprocessor.Service,
// puis demarre le consumer AMQP et les deux tickers (scheduler de campagnes,
// refresh analytics) en goroutines, jusqu'a arret gracieux (SIGINT/SIGTERM).
//
// Variables d'environnement requises :
//
//	POSTGRES_DSN — DSN PostgreSQL (ex. postgres://user:pass@host:5432/fleece?sslmode=disable)
//	AMQP_DSN     — URL RabbitMQ    (ex. amqp://guest:guest@localhost:5672/)
//
// Variables d'environnement optionnelles :
//
//	INTELLIGENCE_PROCESSOR_QUEUE — nom de la queue RabbitMQ consommee
//	                               (defaut : "intelligence-processor"). DISTINCTE
//	                               de CORE_PROCESSOR_QUEUE (defaut
//	                               "core-processor") — voir consumer.go pour
//	                               le detail du double-binding sur le meme
//	                               exchange "fleece".
//	API_URL                      — URL de base du service HTTP interne src/api,
//	                               utilisee par on_campaign_run.go pour
//	                               POST /messages (defaut : "http://api:8080").
//	CAMPAIGN_SCHEDULER_INTERVAL  — intervalle du ticker campaign_scheduler.go,
//	                               format time.ParseDuration (defaut : "30s").
//	ANALYTICS_REFRESH_INTERVAL   — intervalle du ticker analytics_refresh.go,
//	                               format time.ParseDuration (defaut : "5m").
//
// Si une variable requise est absente, si une connexion echoue, si la
// topologie AMQP ne peut pas etre declaree, ou si la consommation se termine
// en erreur : le programme log en ERROR et quitte avec un code de sortie NON
// NUL (voir run(), E9 correctif Phase 3).
//
// Arret gracieux coordonne : le contexte racine (goapp.Context) est annule
// sur SIGINT/SIGTERM ; Consume() et les deux tickers surveillent CE MEME
// contexte. run() attend la fin des trois goroutines (WaitGroup) avant de
// rendre la main — aucune n'est coupee au milieu d'une transaction :
// Consume()/processDelivery() termine le message en cours avant de sortir de
// sa boucle ; chaque ticker termine son tick courant (runSchedulerTick /
// refreshTick) avant de sortir de sa boucle select.
package main

import (
	"os"
	"sync"
	"time"

	intelligenceprocessor "fleece/src/intelligence-processor"

	goamqp "fleece/src/go/amqp"
	goapp "fleece/src/go/app"
	golog "fleece/src/go/log"
	gosql "fleece/src/go/sql"
)

// fleeceExchange est le nom canonique de l'exchange AMQP topic durable
// partage par TOUS les composition roots Go (src/api, core-processor,
// intelligence-processor, D-M29) — declare a l'identique (memes flags,
// durable=true) par chacun pour eviter tout PRECONDITION_FAILED cote broker
// en cas de divergence de parametres entre binaires.
const fleeceExchange = "fleece"

func main() {
	os.Exit(run())
}

// run contient la logique de demarrage/execution de ce composition root et
// retourne le code de sortie du processus.
//
// ISOLEE de main() (E9, Phase 3) : os.Exit court-circuite TOUS les defer en
// attente — l'appeler directement depuis main() aurait empeche defer
// db.Close()/amqpConn.Close() de s'executer sur les chemins d'erreur. run()
// retourne un int ; main() n'appelle os.Exit qu'une seule fois, APRES que
// tous les defer de run() se soient normalement deroules a son retour.
//
// E9 (Phase 3, CAS PRINCIPAL DU CORRECTIF — "worker zombie") : AVANT ce
// correctif, si svc.Consume(ctx) echouait (ex. queue AMQP inexistante/
// inaccessible), le code loggait l'erreur puis "tombait" directement sur
// wg.Wait() : les deux tickers (scheduler de campagnes, refresh analytics)
// n'etant PAS arretes par cette erreur (ils surveillent uniquement ctx, lui-
// meme non annule), ils continuaient de tourner indefiniment — le processus
// restait vivant, actif (CPU non nul, tickers actifs), mais ZERO evenement
// AMQP n'etait jamais consomme. Sous Kubernetes, un tel processus ne crash
// jamais (exit code jamais atteint) : aucun redemarrage automatique n'est
// declenche, et le worker reste silencieusement inoperant. CORRECTIF :
// l'annulation EXPLICITE du contexte racine (cancel(), point d'arret commun
// aux trois goroutines) DES que Consume() retourne une erreur reelle,
// AVANT wg.Wait() — les deux tickers recoivent alors le meme signal d'arret
// qu'un SIGINT/SIGTERM et se terminent proprement ; run() retourne ensuite un
// code de sortie non nul.
func run() int {
	// Contexte racine : annule sur SIGINT/SIGTERM (et explicitement par ce
	// composition root si Consume() echoue — voir plus bas, E9).
	ctx, cancel := goapp.Context()
	defer cancel()

	// Logger structure. Utilise pour TOUS les logs de ce composition root —
	// jamais de log.Printf stdlib.
	logger := golog.Init("info", "text")
	logger.Info("intelligence-processor: starting", "version", goapp.Version)

	// PostgreSQL — DSN depuis l'environnement.
	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		logger.Error("intelligence-processor: POSTGRES_DSN is not set — cannot start")
		return 1
	}
	db, err := gosql.Open(postgresDSN)
	if err != nil {
		logger.Error("intelligence-processor: postgres unavailable", "err", err)
		return 1
	}
	defer db.Close()

	// RabbitMQ — DSN depuis l'environnement.
	amqpDSN := os.Getenv("AMQP_DSN")
	if amqpDSN == "" {
		logger.Error("intelligence-processor: AMQP_DSN is not set — cannot start")
		return 1
	}
	amqpConn := &goamqp.Conn{
		DSN:    amqpDSN,
		Logger: logger,
	}
	if err := amqpConn.Init(); err != nil {
		logger.Error("intelligence-processor: rabbitmq unavailable", "err", err)
		return 1
	}
	defer amqpConn.Close()

	// Configuration additionnelle — voir doc de tete de fichier pour les defauts.
	queue := os.Getenv("INTELLIGENCE_PROCESSOR_QUEUE")
	apiURL := os.Getenv("API_URL")
	schedulerInterval := parseDurationEnv(logger, "CAMPAIGN_SCHEDULER_INTERVAL", 30*time.Second)
	refreshInterval := parseDurationEnv(logger, "ANALYTICS_REFRESH_INTERVAL", 5*time.Minute)

	// Composition root : injection manuelle des dependances dans Service
	// (meme pattern que src/core-processor/cmd/core-processor/main.go et
	// src/api/cmd/api/main.go — pas de zconfig ici, DI manuelle explicite).
	svc := &intelligenceprocessor.Service{
		DB:     db,
		Logger: logger,
		AMQP:   amqpConn,
		Queue:  queue,
		APIURL: apiURL,
	}
	if err := svc.Init(); err != nil {
		logger.Error("intelligence-processor: service init failed", "err", err)
		return 1
	}

	// D-M29 (Phase 3) — DECLARATION IDEMPOTENTE DE LA TOPOLOGIE AMQP.
	//
	// AVANT ce correctif, aucun ExchangeDeclare/QueueDeclare/QueueBind
	// n'existait nulle part dans le depot (ni Go, ni deploy/). svc.Queue
	// (finalise par Init()) est BINDEE aux routing keys REELLEMENT consommees
	// par ce worker, derivees de intelligenceprocessor.BoundRoutingKeys()
	// (source unique : handlersByRoutingKey, consumer.go) — la topologie
	// declaree ne peut donc jamais diverger silencieusement du dispatch reel.
	// Meme exchange ("fleece", topic, durable) que src/core-processor et
	// src/api : parametres identiques par construction (voir goamqp/topology.go).
	if err := amqpConn.DeclareExchange(fleeceExchange, "topic"); err != nil {
		logger.Error("intelligence-processor: declaration exchange AMQP echouee", "exchange", fleeceExchange, "err", err)
		return 1
	}
	if err := amqpConn.EnsureQueueBound(svc.Queue, fleeceExchange, intelligenceprocessor.BoundRoutingKeys()...); err != nil {
		logger.Error("intelligence-processor: declaration/binding de la queue AMQP echouee",
			"queue", svc.Queue, "exchange", fleeceExchange, "err", err)
		return 1
	}

	logger.Info("intelligence-processor: ready",
		"queue", svc.Queue,
		"api_url", svc.APIURL,
		"campaign_scheduler_interval", schedulerInterval.String(),
		"analytics_refresh_interval", refreshInterval.String(),
	)

	// Demarrage des deux tickers en goroutines, avant le consumer (l'ordre
	// n'a pas d'importance fonctionnelle : les trois surveillent le meme ctx
	// et ne s'inter-bloquent jamais).
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		svc.RunCampaignScheduler(ctx, schedulerInterval)
	}()
	go func() {
		defer wg.Done()
		svc.RunAnalyticsRefresh(ctx, refreshInterval)
	}()

	// Consume bloque jusqu'a annulation de ctx (SIGINT/SIGTERM, retourne alors
	// nil) ou jusqu'a une erreur reelle (ex. queue AMQP inexistante/
	// inaccessible).
	exitCode := 0
	if err := svc.Consume(ctx); err != nil {
		logger.Error("intelligence-processor: consume error", "err", err)
		exitCode = 1
		// E9 (Phase 3) : annuler EXPLICITEMENT le contexte racine pour que les
		// deux tickers (qui ne surveillent QUE ctx, jamais l'erreur de
		// Consume) recoivent le meme signal d'arret et se terminent — sans
		// cela, wg.Wait() ci-dessous ne rendrait JAMAIS la main (tickers actifs
		// indefiniment) : le processus resterait un "zombie" vivant sans
		// jamais consommer un seul evenement AMQP, sans jamais atteindre
		// d'exit code (aucun redemarrage Kubernetes declenche).
		cancel()
	}

	// Arret gracieux coordonne : attendre la fin des deux tickers (qui
	// surveillent le meme ctx, desormais annule dans tous les cas — SIGINT/
	// SIGTERM ou l'annulation explicite ci-dessus) avant de sortir du
	// programme.
	wg.Wait()

	if exitCode != 0 {
		logger.Error("intelligence-processor: shutdown apres erreur (consume)", "exit_code", exitCode)
		return exitCode
	}

	logger.Info("intelligence-processor: shutdown complete")
	return 0
}

// parseDurationEnv lit une variable d'environnement au format
// time.ParseDuration ; retourne def si absente ou invalide (avec un log WARN
// dans ce dernier cas — une valeur mal formee ne doit jamais empecher le
// worker de demarrer).
func parseDurationEnv(logger *golog.Logger, key string, def time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		logger.Warn("intelligence-processor: valeur invalide pour variable d'environnement, defaut utilise",
			"key", key, "value", raw, "err", err)
		return def
	}
	return d
}
