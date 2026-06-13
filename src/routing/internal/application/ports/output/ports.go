// Package output — ports pilotes : interfaces requises par les use cases (couche 2).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package output

import (
	"context"

	"fleece/src/routing/internal/domain"
)

// ProviderPricingRepository donne acces aux tarifs des providers.
// Implemente en couche 3 (Postgres, schema "routing").
type ProviderPricingRepository interface {
	// ListByChannelCountry retourne tous les tarifs disponibles pour un canal et un pays.
	// Retourne une slice vide (sans erreur) si aucun tarif n'est configure.
	ListByChannelCountry(ctx context.Context, channel, country string) ([]domain.ProviderPricing, error)
}

// ProviderScoreRepository donne acces aux scores de delivrabilite des providers.
// Implemente en couche 3 (Postgres, schema "routing").
type ProviderScoreRepository interface {
	// ListByChannel retourne tous les scores pour un canal donne.
	// Retourne une slice vide (sans erreur) si aucun score n'est enregistre.
	ListByChannel(ctx context.Context, channel string) ([]domain.ProviderScore, error)

	// Upsert insere ou met a jour le score d'un provider sur un canal.
	// Utilise ON CONFLICT (provider, channel) DO UPDATE.
	Upsert(ctx context.Context, score domain.ProviderScore) error
}

// RoutingRuleRepository donne acces aux regles de routage des workspaces.
// Implemente en couche 3 (Postgres, schema "routing").
type RoutingRuleRepository interface {
	// GetByWorkspace retourne la regle de routage d'un workspace.
	// Retourne domain.ErrRuleNotFound si aucune regle n'est configuree pour ce workspace.
	GetByWorkspace(ctx context.Context, workspaceID string) (domain.RoutingRule, error)
}
