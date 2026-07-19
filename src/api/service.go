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

	// TODO(Phase 2): routes messages   — M-012
	// TODO(Phase 2): routes routing    — M-013
	// TODO(Phase 2): routes contacts   — M-014
	// TODO(Phase 2): routes campaigns  — M-015
	// TODO(Phase 2): routes analytics  — M-016
	// TODO(Phase 2): routes wallet     — M-017
	// TODO(Phase 2): routes webhooks   — M-018
}

// ServeHTTP implémente http.Handler en déléguant au mux interne.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Close libère les ressources du service.
// La fermeture de DB et AMQP est gérée par app.Cleanup dans main.go.
func (s *Service) Close() error {
	// TODO(Phase ultérieure): fermeture gracieuse supplémentaire si nécessaire.
	return nil
}
