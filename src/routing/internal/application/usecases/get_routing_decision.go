package usecases

import (
	"context"
	"errors"
	"fmt"
	"log"

	"fleece/src/routing/internal/application/ports/output"
	"fleece/src/routing/internal/domain"
)

// GetRoutingDecision orchestre la selection du meilleur provider pour un workspace.
// Il ne depend que des ports (interfaces) : aucune dependance vers un framework.
//
// Le champ ContactScore est optionnel (peut etre nil). Quand il est nil ou quand
// le service contact-intelligence est indisponible, le use case degrade gracieusement
// vers les provider_scores seuls (comportement V1 inchange).
type GetRoutingDecision struct {
	Pricing      output.ProviderPricingRepository
	Scores       output.ProviderScoreRepository
	Rules        output.RoutingRuleRepository
	ContactScore output.ContactScoreClient // optionnel : nil = pas d'enrichissement contact
}

// Execute charge la regle du workspace (fallback HighestDelivery si absente),
// les tarifs pour le canal+pays, les scores pour le canal, puis appelle SelectProvider.
//
// recipient : numero de telephone du destinataire (E.164). Utilise UNIQUEMENT quand
// la strategie est highest_delivery pour enrichir le score provider avec le score de
// joignabilite du contact (appel REST vers contact-intelligence). Si recipient est vide
// ou si ContactScore est nil, l'enrichissement est saute silencieusement.
//
// Fallback gracieux contact-intelligence :
//   - Si ContactScore est nil → pas d'appel, pas d'erreur.
//   - Si GetChannelScore retourne une erreur → log warning, degrade vers provider_scores seuls.
//   - Le routage n'echoue JAMAIS a cause du service contact-intelligence.
//
// recipientCount : l'EstimatedCost de la RoutingDecision est le cout unitaire
// (par message unique). Multiplier par recipientCount cote appelant si necessaire.
// Ce choix est documente ici : retourner le cout unitaire preserve la flexibilite
// (le Messaging Service connait le nombre reel de destinataires apres expansion).
func (uc GetRoutingDecision) Execute(ctx context.Context, workspaceID, channel, country, recipient string, recipientCount int) (domain.RoutingDecision, error) {
	// 1. Charger la regle du workspace (fallback sur HighestDelivery si absente).
	// La regle complete (avec Config) est transmise au domaine pour que la strategie
	// Custom puisse appliquer la configuration personnalisee.
	activeRule := domain.RoutingRule{Strategy: domain.StrategyHighestDelivery}
	rule, err := uc.Rules.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		if !errors.Is(err, domain.ErrRuleNotFound) {
			return domain.RoutingDecision{}, fmt.Errorf("get_routing: charger regle workspace %s: %w", workspaceID, err)
		}
		// ErrRuleNotFound : comportement par defaut (HighestDelivery), pas d'erreur.
	} else {
		activeRule = rule
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

	// 4. Enrichissement contact (highest_delivery uniquement).
	//
	// Le score de joignabilite du contact module le score provider via CombineScore.
	// Le contactScore s'applique au CANAL (pas au provider individuel) : tous les providers
	// du meme canal recoivent le meme bonus/malus contact, mais le tri provider reste
	// discrimine par le provider_score. Formule : combined = round(0.5*ps + 0.5*cs).
	//
	// Conditions pour activer l'enrichissement :
	//   - la strategie est highest_delivery
	//   - le port ContactScore est cable (non nil)
	//   - le numero de telephone du destinataire est fourni (non vide)
	if activeRule.Strategy == domain.StrategyHighestDelivery && uc.ContactScore != nil && recipient != "" {
		contactScore, _, contactErr := uc.ContactScore.GetChannelScore(ctx, recipient, channel)
		if contactErr != nil {
			// Fallback gracieux : log warning, continue sans enrichissement.
			// Le routage ne doit JAMAIS echouer a cause de contact-intelligence.
			log.Printf("get_routing: contact-intelligence indisponible pour %s/%s, degrade vers provider_scores seuls: %v", recipient, channel, contactErr)
		} else {
			// Combiner le score contact avec chaque score provider.
			// La modification est locale (nouvelle slice) : scores originaux inchanges.
			enriched := make([]domain.ProviderScore, len(scores))
			copy(enriched, scores)
			for i := range enriched {
				enriched[i].Score = domain.CombineScore(enriched[i].Score, contactScore)
			}
			scores = enriched
		}
	}

	// 5. Appeler la logique pure du domaine avec la regle complete.
	decision, err := domain.SelectProvider(pricing, scores, activeRule)
	if err != nil {
		return domain.RoutingDecision{}, fmt.Errorf("get_routing: selection provider: %w", err)
	}

	return decision, nil
}
