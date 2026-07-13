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

// ContactScoreClient permet d'obtenir le score de joignabilite d'un contact sur un canal.
// Implemente en couche 3 (client REST vers le service contact-intelligence).
//
// Semantique du score retourne :
//   - score [0,100] : joignabilite du destinataire sur ce canal (Laplace α=1).
//   - found=false : contact inconnu → score=50 (prior neutre, jamais d'erreur 404).
//   - Si le canal demande est absent des scores par canal du contact, le score global
//     de delivraison (delivery_score) est retourne en repli.
//
// Le use case n'appelle ce port QUE pour la strategie highest_delivery.
// En cas d'erreur (timeout, service indisponible, JSON invalide), le use case
// degrade gracieusement vers les provider_scores seuls (pas d'erreur propagee).
//
// IMPORTANT (isolation inter-services) : cette interface utilise uniquement des types
// primitifs Go (int, bool, string). L'implementation concrete (couche 3) parle HTTP —
// elle n'importe jamais fleece/src/contact-intelligence/... (D11, D19).
type ContactScoreClient interface {
	// GetChannelScore retourne le score de joignabilite du destinataire phone sur channel.
	// Si le contact est inconnu (found=false), score=50 (prior neutre Laplace).
	// Si le canal n'est pas dans les scores par canal, retourne le score global de delivraison.
	// N'est jamais 404 : contact inconnu → found=false, score=50, err=nil.
	GetChannelScore(ctx context.Context, phone, channel string) (score int, found bool, err error)
}
