// Package persistence implemente les ports output de routing via *sql.DB.
// Couche 3 (Interface Adapters, driven).
package persistence

import "fleece/src/routing/internal/domain"

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
type scoreRecord struct {
	Provider string
	Channel  string
	Score    int
}

// toEntity convertit un scoreRecord en entite domaine.
func (r scoreRecord) toEntity() domain.ProviderScore {
	return domain.ProviderScore{
		ProviderID: r.Provider,
		Channel:    domain.Channel(r.Channel),
		Score:      r.Score,
	}
}

// ruleRecord est le modele de la table routing.routing_rules.
type ruleRecord struct {
	WorkspaceID string
	Strategy    string
}

// toEntity convertit un ruleRecord en entite domaine.
// Si la strategie en base est invalide, retombe sur HighestDelivery (defensive programming).
func (r ruleRecord) toEntity() domain.RoutingRule {
	strategy, err := domain.ParseStrategy(r.Strategy)
	if err != nil {
		strategy = domain.StrategyHighestDelivery
	}
	return domain.RoutingRule{
		WorkspaceID: r.WorkspaceID,
		Strategy:    strategy,
	}
}
