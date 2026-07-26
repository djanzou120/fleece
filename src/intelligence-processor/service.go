// Package intelligenceprocessor est le worker Go qui consomme les evenements
// de messagerie ("message.sent", "message.delivered", "message.failed")
// publies par src/api sur RabbitMQ, ainsi que l'evenement interne
// "campaign.run" qu'il publie lui-meme (voir campaign_scheduler.go), et
// applique leurs effets d'ENRICHISSEMENT sur la base PostgreSQL partagee :
// scoring de joignabilite contact (contact_intel), agregats analytics
// (analytics.message_daily), correlation destinataires de campagne
// (campaign.campaign_recipients), et l'envoi effectif des messages d'un run
// de campagne via l'API HTTP interne (src/api).
//
// Ce worker est le JUMEAU de src/core-processor (meme evenements
// message.delivered/message.failed, QUEUE DISTINCTE — voir consumer.go), mais
// n'a AUCUNE responsabilite sur messaging.messages.status ni sur
// wallet.wallets/wallet.wallet_transactions : ces deux domaines restent
// EXCLUSIVEMENT geres par src/core-processor (Decision C, PM M-021) — deux
// workers qui crediteraient le wallet doubleraient tout remboursement.
//
// Architecture : meme pattern que src/api et src/core-processor — structure
// plate, acces DB direct (s.DB.Select/Get/Exec/Begin), pas de
// repository/port/use case. Contrainte Go "1 repertoire = 1 package"
// (precedent D-M09, Phase 1) : le composition root vit dans
// src/intelligence-processor/cmd/intelligence-processor/main.go (package
// main), le reste ici en package intelligenceprocessor.
package intelligenceprocessor

import (
	"net/http"

	goamqp "fleece/src/go/amqp"
	golog "fleece/src/go/log"
	gosql "fleece/src/go/sql"
)

// Service est la racine du worker intelligence-processor. Les champs
// exportes sont injectes manuellement par le composition root
// (cmd/intelligence-processor/main.go), a l'identique du pattern
// src/core-processor/service.go.
type Service struct {
	// DB est la connexion PostgreSQL partagee (memes schemas que src/api :
	// messaging (lecture seule), contact_intel, analytics, campaign — voir
	// D-M01, prefixer chaque table).
	DB *gosql.DB
	// Logger est le logger structure partage.
	Logger *golog.Logger
	// AMQP est la connexion RabbitMQ partagee, utilisee en consommation
	// (Consume, consumer.go) ET en publication (campaign_scheduler.go publie
	// "campaign.run" — seul evenement emis par ce worker).
	AMQP *goamqp.Conn

	// Queue est le nom de la queue RabbitMQ consommee. Configurable via la
	// variable d'environnement INTELLIGENCE_PROCESSOR_QUEUE (voir
	// cmd/intelligence-processor/main.go). Defaut : "intelligence-processor".
	//
	// DISTINCTE de la queue de src/core-processor (defaut "core-processor") :
	// les deux queues sont bindees (infrastructure RabbitMQ, hors perimetre Go
	// de cette tache) aux MEMES routing keys sur le MEME exchange "fleece" —
	// chaque queue recoit sa PROPRE COPIE de chaque evenement publie (topic
	// exchange RabbitMQ : routage vers TOUTES les queues dont un binding
	// matche, pas une seule). Voir consumer.go pour le detail.
	Queue string

	// APIURL est l'URL de base du service HTTP interne src/api (ex.
	// "http://api:8080"), utilisee par on_campaign_run.go pour POST /messages.
	// Configurable via la variable d'environnement API_URL. Defaut :
	// "http://api:8080".
	APIURL string

	// HTTPClient est le client HTTP utilise pour appeler src/api. Optionnel :
	// si nil, un *http.Client avec un timeout par defaut est cree a la volee
	// (voir on_campaign_run.go, httpClient()). Champ expose pour permettre aux
	// tests d'injecter un client pointant vers un httptest.Server.
	HTTPClient *http.Client
}

// Init verifie que les dependances obligatoires sont presentes et applique
// les valeurs par defaut. Retourne une erreur si une dependance requise est
// absente — appele explicitement par le composition root avant Run.
func (s *Service) Init() error {
	if s.Queue == "" {
		s.Queue = "intelligence-processor"
	}
	if s.APIURL == "" {
		s.APIURL = "http://api:8080"
	}
	return nil
}
