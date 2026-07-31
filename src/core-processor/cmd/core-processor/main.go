// Command core-processor est le point d'entree (composition root) du worker
// core-processor. Il ouvre les connexions d'infrastructure (PostgreSQL,
// RabbitMQ), declare la topologie AMQP dont il a besoin (D-M29), injecte les
// dependances manuellement dans coreprocessor.Service, puis consomme la
// queue configuree jusqu'a arret gracieux (SIGINT/SIGTERM).
//
// Variables d'environnement requises :
//
//	POSTGRES_DSN — DSN PostgreSQL (ex. postgres://user:pass@host:5432/fleece?sslmode=disable)
//	AMQP_DSN     — URL RabbitMQ    (ex. amqp://guest:guest@localhost:5672/)
//
// Variables d'environnement optionnelles :
//
//	CORE_PROCESSOR_QUEUE      — nom de la queue RabbitMQ consommee (defaut : "core-processor").
//	WEBHOOK_RETRY_INTERVAL    — intervalle du ticker webhook_retry_scheduler.go
//	                            (D-M43, format time.ParseDuration, defaut : "30s").
//
// Si une variable requise est absente, si une connexion echoue, si la
// topologie AMQP ne peut pas etre declaree, ou si la consommation se termine
// en erreur : le programme log en ERROR et quitte avec un code de sortie NON
// NUL (voir run(), E9 correctif Phase 3 — avant ce correctif, ces chemins
// retournaient un exit code 0 implicite, indiscernable d'un arret volontaire).
//
// Arret gracieux COORDONNE (D-M43 — avant ce correctif, Consume() etait la
// SEULE goroutine de ce composition root ; le scheduler de retry webhook en
// ajoute une seconde, sur le modele de cmd/intelligence-processor/main.go) :
// le contexte racine (goapp.Context) est annule sur SIGINT/SIGTERM ; Consume()
// ET RunWebhookRetryScheduler surveillent CE MEME contexte. run() attend la
// fin des deux goroutines (WaitGroup) avant de rendre la main — si Consume()
// retourne une erreur reelle, le contexte racine est annule EXPLICITEMENT
// (meme motif que intelligence-processor, E9) pour que le ticker de retry ne
// reste jamais actif seul (worker "zombie" qui ne consommerait plus mais
// continuerait de retenter des webhooks indefiniment).
package main

import (
	"os"
	"sync"
	"time"

	coreprocessor "fleece/src/core-processor"
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
func run() int {
	// Contexte racine : annule sur SIGINT/SIGTERM.
	ctx, cancel := goapp.Context()
	defer cancel()

	// Logger structure (JSON en production, text en dev). Utilise pour TOUS
	// les logs de ce composition root — jamais de log.Printf stdlib.
	logger := golog.Init("info", "text")
	logger.Info("core-processor: starting", "version", goapp.Version)

	// PostgreSQL — DSN depuis l'environnement.
	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		logger.Error("core-processor: POSTGRES_DSN is not set — cannot start")
		return 1
	}
	db, err := gosql.Open(postgresDSN)
	if err != nil {
		logger.Error("core-processor: postgres unavailable", "err", err)
		return 1
	}
	defer db.Close()

	// RabbitMQ — DSN depuis l'environnement.
	amqpDSN := os.Getenv("AMQP_DSN")
	if amqpDSN == "" {
		logger.Error("core-processor: AMQP_DSN is not set — cannot start")
		return 1
	}
	amqpConn := &goamqp.Conn{
		DSN:    amqpDSN,
		Logger: logger,
	}
	if err := amqpConn.Init(); err != nil {
		logger.Error("core-processor: rabbitmq unavailable", "err", err)
		return 1
	}
	defer amqpConn.Close()

	// Queue consommee — configurable, defaut "core-processor" (voir Service.Init).
	queue := os.Getenv("CORE_PROCESSOR_QUEUE")

	// Intervalle du scheduler de retry webhook (D-M43) — defaut 30s.
	webhookRetryInterval := parseDurationEnv(logger, "WEBHOOK_RETRY_INTERVAL", 30*time.Second)

	// Composition root : injection manuelle des dependances dans Service
	// (meme pattern que src/api/cmd/api/main.go — pas de zconfig ici, DI
	// manuelle explicite).
	svc := &coreprocessor.Service{
		DB:     db,
		Logger: logger,
		AMQP:   amqpConn,
		Queue:  queue,
	}
	if err := svc.Init(); err != nil {
		logger.Error("core-processor: service init failed", "err", err)
		return 1
	}

	// D-M29 (Phase 3) — DECLARATION IDEMPOTENTE DE LA TOPOLOGIE AMQP.
	//
	// AVANT ce correctif, aucun ExchangeDeclare/QueueDeclare/QueueBind
	// n'existait nulle part dans le depot (ni Go, ni deploy/) : l'exchange
	// "fleece" et la queue "core-processor" n'existaient qu'en theorie —
	// Consume() aurait echoue (ou pire, un Publish producteur aurait
	// silencieusement echoue, voir goamqp.Conn.Publish) contre une topologie
	// jamais creee. svc.Queue (finalise par Init(), plus jamais la variable
	// "queue" brute qui peut etre vide) est BINDEE aux routing keys REELLEMENT
	// consommees par ce worker, derivees de coreprocessor.BoundRoutingKeys()
	// (source unique : handlersByRoutingKey, consumer.go) — la topologie
	// declaree ne peut donc jamais diverger silencieusement du dispatch reel.
	//
	// Une erreur ici est FATALE (E9) : consommer une queue jamais declaree/liee
	// echouerait de toute facon des l'appel a Consume ci-dessous.
	if err := amqpConn.DeclareExchange(fleeceExchange, "topic"); err != nil {
		logger.Error("core-processor: declaration exchange AMQP echouee", "exchange", fleeceExchange, "err", err)
		return 1
	}
	if err := amqpConn.EnsureQueueBound(svc.Queue, fleeceExchange, coreprocessor.BoundRoutingKeys()...); err != nil {
		logger.Error("core-processor: declaration/binding de la queue AMQP echouee",
			"queue", svc.Queue, "exchange", fleeceExchange, "err", err)
		return 1
	}

	logger.Info("core-processor: ready", "queue", svc.Queue, "webhook_retry_interval", webhookRetryInterval.String())

	// D-M43 — demarrage du ticker de retry webhook en goroutine, AVANT
	// Consume() (l'ordre n'a pas d'importance fonctionnelle : les deux
	// surveillent le meme ctx et ne s'inter-bloquent jamais). Meme motif que
	// cmd/intelligence-processor/main.go (RunCampaignScheduler/
	// RunAnalyticsRefresh) : arret gracieux coordonne via WaitGroup.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		svc.RunWebhookRetryScheduler(ctx, webhookRetryInterval)
	}()

	// Consume bloque jusqu'a annulation de ctx (SIGINT/SIGTERM, retourne alors
	// nil — arret gracieux volontaire) ou jusqu'a une erreur reelle (ex. queue
	// AMQP inexistante/inaccessible au moment de l'appel).
	//
	// E9 (Phase 3) — AVANT ce correctif : cette erreur etait logguee puis
	// main() se terminait avec un exit code 0 implicite (aucun appel a
	// os.Exit ni retour d'erreur au runtime) : indiscernable d'un arret
	// volontaire pour Kubernetes (restartPolicy) ou tout operateur lisant
	// `echo $?`. Le worker mourait "proprement en apparence" alors qu'il
	// n'avait jamais traite un seul message. CORRECTIF : exit code 1.
	exitCode := 0
	if err := svc.Consume(ctx); err != nil {
		logger.Error("core-processor: consume error", "err", err)
		exitCode = 1
		// D-M43 : annuler EXPLICITEMENT le contexte racine pour que le ticker
		// de retry webhook (qui ne surveille QUE ctx, jamais l'erreur de
		// Consume) recoive le meme signal d'arret et se termine — sans cela,
		// wg.Wait() ci-dessous ne rendrait jamais la main (meme "worker
		// zombie" que E9 sur intelligence-processor : processus vivant, mais
		// plus aucun message AMQP consomme).
		cancel()
	}

	// Arret gracieux coordonne : attendre la fin du ticker de retry (qui
	// surveille le meme ctx, desormais annule dans tous les cas — SIGINT/
	// SIGTERM ou l'annulation explicite ci-dessus) avant de sortir.
	wg.Wait()

	if exitCode != 0 {
		logger.Error("core-processor: shutdown apres erreur (consume)", "exit_code", exitCode)
		return exitCode
	}

	logger.Info("core-processor: shutdown complete")
	return 0
}

// parseDurationEnv lit une variable d'environnement au format
// time.ParseDuration ; retourne def si absente ou invalide (avec un log WARN
// dans ce dernier cas — une valeur mal formee ne doit jamais empecher le
// worker de demarrer). Copie de cmd/intelligence-processor/main.go (meme
// motif, deux binaires distincts — pas de lib partagee pour ~10 lignes).
func parseDurationEnv(logger *golog.Logger, key string, def time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		logger.Warn("core-processor: valeur invalide pour variable d'environnement, defaut utilise",
			"key", key, "value", raw, "err", err)
		return def
	}
	return d
}
