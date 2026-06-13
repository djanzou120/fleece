package usecases

import (
	"context"
	"errors"
	"fmt"

	"fleece/src/routing/internal/application/ports/output"
	"fleece/src/routing/internal/domain"
)

// GetRoutingDecision orchestre la selection du meilleur provider pour un workspace.
// Il ne depend que des ports (interfaces) : aucune dependance vers un framework.
type GetRoutingDecision struct {
	Pricing output.ProviderPricingRepository
	Scores  output.ProviderScoreRepository
	Rules   output.RoutingRuleRepository
}

// Execute charge la regle du workspace (fallback HighestDelivery si absente),
// les tarifs pour le canal+pays, les scores pour le canal, puis appelle SelectProvider.
//
// recipientCount : l'EstimatedCost de la RoutingDecision est le cout unitaire
// (par message unique). Multiplier par recipientCount cote appelant si necessaire.
// Ce choix est documente ici : retourner le cout unitaire preserve la flexibilite
// (le Messaging Service connait le nombre reel de destinataires apres expansion).
func (uc GetRoutingDecision) Execute(ctx context.Context, workspaceID, channel, country string, recipientCount int) (domain.RoutingDecision, error) {
	// 1. Charger la strategie du workspace (fallback sur HighestDelivery si absente).
	strategy := domain.StrategyHighestDelivery
	rule, err := uc.Rules.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		if !errors.Is(err, domain.ErrRuleNotFound) {
			return domain.RoutingDecision{}, fmt.Errorf("get_routing: charger regle workspace %s: %w", workspaceID, err)
		}
		// ErrRuleNotFound : comportement par defaut (HighestDelivery), pas d'erreur.
	} else {
		strategy = rule.Strategy
	}

	// 2. Charger les tarifs pour le canal et le pays.
	pricing, err := uc.Pricing.ListByChannelCountry(ctx, channel, country)
	if err != nil {
		return domain.RoutingDecision{}, fmt.Errorf("get_routing: charger tarifs canal=%s pays=%s: %w", channel, country, err)
	}

	// 3. Charger les scores de delivrabilite pour le canal.
	scores, err := uc.Scores.ListByChannel(ctx, channel)
	if err != nil {
		return domain.RoutingDecision{}, fmt.Errorf("get_routing: charger scores canal=%s: %w", channel, err)
	}

	// 4. Appeler la logique pure du domaine.
	decision, err := domain.SelectProvider(pricing, scores, strategy)
	if err != nil {
		return domain.RoutingDecision{}, fmt.Errorf("get_routing: selection provider: %w", err)
	}

	return decision, nil
}
