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
// Variable d'environnement optionnelle :
//
//	CORE_PROCESSOR_QUEUE — nom de la queue RabbitMQ consommee (defaut : "core-processor").
//
// Si une variable requise est absente, si une connexion echoue, si la
// topologie AMQP ne peut pas etre declaree, ou si la consommation se termine
// en erreur : le programme log en ERROR et quitte avec un code de sortie NON
// NUL (voir run(), E9 correctif Phase 3 — avant ce correctif, ces chemins
// retournaient un exit code 0 implicite, indiscernable d'un arret volontaire).
//
// Arret gracieux : le contexte racine (goapp.Context) est annule sur
// SIGINT/SIGTERM ; Consume() surveille ce meme contexte (select sur ctx.Done())
// et retourne des que le message AMQP en cours de traitement est termine —
// contrairement au serveur HTTP de src/api (D-M07, qui utilise
// http.Server.Shutdown avec un delai de drain explicite pour laisser des
// requetes CONCURRENTES en cours se terminer), ce worker traite les
// deliveries de facon strictement sequentielle (une seule goroutine de
// consommation) : il n'y a jamais qu'un seul message "en vol", donc aucun
// delai de drain supplementaire n'est necessaire — la boucle s'arrete des que
// le message courant (s'il y en a un) est acquitte/rejete.
package main

import (
	"os"

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

	logger.Info("core-processor: ready", "queue", svc.Queue)

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
	if err := svc.Consume(ctx); err != nil {
		logger.Error("core-processor: consume error", "err", err)
		return 1
	}

	logger.Info("core-processor: shutdown complete")
	return 0
}
