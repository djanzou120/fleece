// Package input — ports pilotants : interfaces exposees par le service (couche 2).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package input

import (
	"context"

	"fleece/src/routing/internal/domain"
)

// GetRoutingDecisionUseCase selectionne le meilleur provider pour un workspace,
// un canal et un pays donnes.
type GetRoutingDecisionUseCase interface {
	// Execute charge la regle du workspace, les tarifs et les scores, puis
	// retourne la RoutingDecision optimisee selon la strategie configuree.
	//
	// Si aucune regle n'est configuree pour le workspace, la strategie par defaut
	// HighestDelivery est appliquee.
	//
	// recipientCount est le nombre de destinataires ; l'EstimatedCost retourne
	// est le cout unitaire (par message). Documente ici pour evolution future.
	//
	// Retourne domain.ErrNoProviderAvailable si aucun provider n'est disponible.
	Execute(ctx context.Context, workspaceID, channel, country string, recipientCount int) (domain.RoutingDecision, error)
}

// UpdateProviderScoreUseCase met a jour le score de delivrabilite d'un provider
// en retour de DLR (Delivery Receipt).
type UpdateProviderScoreUseCase interface {
	// Execute met a jour le score du provider providerID sur le canal channel.
	// score doit etre compris entre 0 et 100.
	Execute(ctx context.Context, providerID, channel string, score int) error
}
