// Package persistence implemente les ports output de routing via *sql.DB.
// Couche 3 (Interface Adapters, driven).
package persistence

import (
	"database/sql"
	"encoding/json"

	"fleece/src/routing/internal/domain"
)

// pricingRecord est le modele de la table routing.provider_pricing.
// La devise n'est pas stockee en base ; elle est injectee par l'adapter
// via defaultCurrency au moment de la construction de l'entite domaine.
type pricingRecord struct {
	ID       int64
	Provider string
	Channel  string
	Country  string
	Cost     int64
}

// toEntity convertit un pricingRecord en entite domaine.
// La devise est fournie par l'adapter (injected au constructeur du repository).
func (r pricingRecord) toEntity(currency string) domain.ProviderPricing {
	return domain.ProviderPricing{
		ID:         r.ID,
		ProviderID: r.Provider,
		Channel:    domain.Channel(r.Channel),
		Country:    r.Country,
		Cost:       domain.Money{Amount: r.Cost, Currency: currency},
	}
}

// scoreRecord est le modele de la table routing.provider_scores.
// Inclut les colonnes ajoutees par la migration 0013 :
//   - avg_latency_ms INTEGER NOT NULL DEFAULT 0
//   - country        VARCHAR(10) NOT NULL DEFAULT ”
type scoreRecord struct {
	Provider     string
	Channel      string
	Score        int
	AvgLatencyMs int
	Country      string
}

// toEntity convertit un scoreRecord en entite domaine.
func (r scoreRecord) toEntity() domain.ProviderScore {
	return domain.ProviderScore{
		ProviderID:   r.Provider,
		Channel:      domain.Channel(r.Channel),
		Score:        r.Score,
		AvgLatencyMs: r.AvgLatencyMs,
		Country:      r.Country,
	}
}

// ruleRecord est le modele de la table routing.routing_rules.
// Inclut les colonnes ajoutees par la migration 0013 :
//   - channel VARCHAR(50) NOT NULL DEFAULT ”
//   - config  JSONB (nullable)
type ruleRecord struct {
	WorkspaceID string
	Strategy    string
	Channel     string
	// ConfigRaw stocke la valeur JSONB nullable avant decodage.
	// sql.NullString permet de distinguer NULL de la chaine vide.
	ConfigRaw sql.NullString
}

// toEntity convertit un ruleRecord en entite domaine.
//
// Decodage de config JSONB :
//   - NULL (ConfigRaw.Valid == false) → Config nil dans le domaine.
//   - JSON valide → demappe en domain.RoutingConfig (map[string]any).
//   - JSON invalide (corruption) → Config nil, defensive programming (pas de panic).
//
// Si la strategie en base est invalide, retombe sur HighestDelivery.
func (r ruleRecord) toEntity() domain.RoutingRule {
	strategy, err := domain.ParseStrategy(r.Strategy)
	if err != nil {
		strategy = domain.StrategyHighestDelivery
	}

	var config domain.RoutingConfig
	if r.ConfigRaw.Valid && r.ConfigRaw.String != "" {
		// Decoder le JSONB en map[string]any ; erreur silencieuse (config nil).
		var m map[string]any
		if jsonErr := json.Unmarshal([]byte(r.ConfigRaw.String), &m); jsonErr == nil {
			config = domain.RoutingConfig(m)
		}
	}

	return domain.RoutingRule{
		WorkspaceID: r.WorkspaceID,
		Strategy:    strategy,
		Channel:     r.Channel,
		Config:      config,
	}
}
