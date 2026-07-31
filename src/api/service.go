// Package api est le point d'entrée du service HTTP unifié Fleece.
// Il agrège tous les endpoints internes (messages, routing, contacts, campaigns,
// analytics, wallet, webhooks) dans un seul binaire Go.
//
// Architecture : structure plate inspirée du pattern winmarket —
// un fichier par endpoint, accès DB direct, zconfig pour la DI.
// Les abstractions (interface Provider, machines à états) sont conservées
// uniquement là où elles sont justifiées.
package api

import (
	"net/http"

	goamqp "fleece/src/go/amqp"
	golog "fleece/src/go/log"
	gosql "fleece/src/go/sql"
)

// Service est la racine du service HTTP unifié.
// Les champs exportés sont injectés par zconfig (tags inject/key) ou manuellement
// dans main.go.
type Service struct {
	// DB est la connexion PostgreSQL partagée, injectée par zconfig.
	DB *gosql.DB `inject:""`
	// Logger est le logger structuré partagé, injecté par zconfig.
	Logger *golog.Logger `inject:""`
	// AMQP est la connexion RabbitMQ partagée, injectée par zconfig.
	AMQP *goamqp.Conn `inject:""`

	// Providers est le registre des adapters fournisseurs, indexé par providerId.
	// Peuplé dans main.go via providers.BuildRegistry avant l'appel à Init().
	// Clés canoniques : "sms-twilio", "whatsapp-meta", "telegram-bot".
	Providers map[string]Provider

	// OMWebhookSecret est la clé HMAC-SHA256 utilisée pour valider les callbacks
	// Orange Money (POST /webhooks/om, header X-OM-Signature). Convention zconfig :
	// variable d'environnement OMWEBHOOKSECRET (préfixe = nom du champ struct, D-M02).
	// Injectée manuellement dans le composition root (cmd/api/main.go) depuis
	// os.Getenv("OM_WEBHOOK_SECRET"). Vide = secret non configuré : voir la politique
	// de validation documentée dans webhooks_om.go (log WARN, callback accepté sans
	// vérification — NE PAS utiliser cette configuration en production).
	OMWebhookSecret string `key:"om_webhook_secret"`
	// MTNWebhookSecret est l'équivalent de OMWebhookSecret pour MTN Mobile Money
	// (POST /webhooks/mtn, header X-MTN-Signature). Voir webhooks_mtn.go.
	MTNWebhookSecret string `key:"mtn_webhook_secret"`

	// Env identifie l'environnement d'exécution (variable d'environnement
	// FLEECE_ENV, injectée manuellement dans le composition root,
	// cmd/api/main.go). E5 (revue architecture Phase 3 / D-M43,
	// webhook_endpoints_create.go) : seule une valeur explicitement
	// "development"/"dev"/"test"/"local" (isNonProductionEnv) autorise
	// l'exemption localhost/127.0.0.1 en HTTP de validateWebhookURL — AVANT
	// ce correctif l'exemption s'appliquait INCONDITIONNELLEMENT, y compris
	// en production. Vide/absent = PRODUCTION (comportement le plus sûr par
	// défaut, voir isNonProductionEnv).
	Env string `key:"env"`

	mux *http.ServeMux
}

// Init initialise le mux HTTP et enregistre toutes les routes.
// Appelé directement depuis le composition root (cmd/api/main.go) après injection
// des dépendances. (Compatible avec app.Bootstrap via réflexion si branché ultérieurement.)
func (s *Service) Init() error {
	s.mux = http.NewServeMux()
	s.registerRoutes()
	if s.Providers == nil {
		s.Providers = make(map[string]Provider)
	}
	return nil
}

// registerRoutes enregistre toutes les routes HTTP du service.
// Les routes métier sont ajoutées en Phase 2 (M-012..M-018).
func (s *Service) registerRoutes() {
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Routes messages (M-012) — handlers definis dans messages_send.go, messages_get.go, messages_list.go.
	s.mux.HandleFunc("POST /messages", s.HandleSendMessage)
	s.mux.HandleFunc("GET /messages/{id}", s.HandleGetMessage)
	s.mux.HandleFunc("GET /messages", s.HandleListMessages)

	// Routes routing (M-013) — handlers definis dans routing_select.go, routing_estimate.go.
	s.mux.HandleFunc("POST /routing/select", s.HandleRoutingSelect)
	s.mux.HandleFunc("POST /routing/estimate", s.HandleRoutingEstimate)

	// Routes contacts (M-014) — handlers definis dans contacts_get_score.go, contacts_record_outcome.go.
	s.mux.HandleFunc("GET /contacts/{phone}/score", s.HandleGetContactScore)
	s.mux.HandleFunc("POST /contacts/outcomes", s.HandleRecordOutcome)

	// Routes wallet (M-017) — handlers definis dans wallet_get.go, wallet_transactions.go,
	// wallet_deduct.go, wallet_topup.go.
	s.mux.HandleFunc("GET /wallet/{workspaceId}", s.HandleGetWallet)
	s.mux.HandleFunc("GET /wallet/{workspaceId}/transactions", s.HandleListWalletTransactions)
	s.mux.HandleFunc("POST /wallet/{workspaceId}/deduct", s.HandleDeductWallet)
	s.mux.HandleFunc("POST /wallet/{workspaceId}/topup", s.HandleTopupWallet)

	// Routes campaigns (M-015) — handlers definis dans campaigns_create.go,
	// campaigns_schedule.go, campaigns_cancel.go, campaigns_import.go,
	// campaigns_get.go, campaigns_list.go.
	s.mux.HandleFunc("POST /campaigns", s.HandleCreateCampaign)
	s.mux.HandleFunc("PATCH /campaigns/{id}/schedule", s.HandleScheduleCampaign)
	s.mux.HandleFunc("PATCH /campaigns/{id}/cancel", s.HandleCancelCampaign)
	s.mux.HandleFunc("POST /campaigns/{id}/recipients", s.HandleImportRecipients)
	s.mux.HandleFunc("GET /campaigns/{id}", s.HandleGetCampaign)
	s.mux.HandleFunc("GET /campaigns", s.HandleListCampaigns)

	// Routes analytics (M-016) — handlers definis dans analytics_get_kpis.go, analytics_get_timeseries.go.
	s.mux.HandleFunc("GET /analytics/kpis", s.HandleGetKpis)
	s.mux.HandleFunc("GET /analytics/timeseries", s.HandleGetTimeseries)

	// Routes webhooks (M-018) — handlers definis dans webhooks_om.go, webhooks_mtn.go,
	// webhooks_telegram.go.
	s.mux.HandleFunc("POST /webhooks/om", s.HandleWebhookOM)
	s.mux.HandleFunc("POST /webhooks/mtn", s.HandleWebhookMTN)
	s.mux.HandleFunc("POST /webhooks/telegram", s.HandleWebhookTelegram)

	// Routes webhook-endpoints (D-M40 — CRUD des webhooks SORTANTS, T-005 porté
	// tardivement) — handlers definis dans webhook_endpoints_create.go,
	// webhook_endpoints_list.go, webhook_endpoints_delete.go. Distinct de
	// /webhooks/* ci-dessus (callbacks ENTRANTS des providers de paiement/messagerie).
	s.mux.HandleFunc("POST /webhook-endpoints", s.HandleCreateWebhookEndpoint)
	s.mux.HandleFunc("GET /webhook-endpoints", s.HandleListWebhookEndpoints)
	s.mux.HandleFunc("DELETE /webhook-endpoints/{id}", s.HandleDeleteWebhookEndpoint)
}

// ServeHTTP implémente http.Handler en déléguant au mux interne, enveloppé par
// recoverMiddleware : toute panique survenue dans un handler (ex. newUUID sur
// épuisement d'entropie) est convertie en 500 JSON plutôt que de crasher le
// processus (écart C4 de la revue d'architecture Phase 2).
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	recoverMiddleware(s.Logger, s.mux).ServeHTTP(w, r)
}

// recoverMiddleware enveloppe next et récupère toute panique survenue durant le
// traitement de la requête, transformée en réponse 500 JSON via writeError au
// lieu de faire crasher le processus. logger peut être nil (aucun log, réponse
// 500 quand même) — utile pour les tests qui construisent un Service minimal.
// Fonction indépendante des routes réelles, testable isolément (voir
// helpers_test.go).
func recoverMiddleware(logger *golog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if logger != nil {
					logger.Error("api: panic recovered", "panic", rec, "path", r.URL.Path)
				}
				writeError(w, http.StatusInternalServerError, "erreur interne")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Close libère les ressources du service.
// La fermeture de DB et AMQP est gérée par app.Cleanup dans main.go.
func (s *Service) Close() error {
	// TODO(Phase ultérieure): fermeture gracieuse supplémentaire si nécessaire.
	return nil
}
