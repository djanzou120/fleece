package usecases

import (
	"context"
	"fmt"

	"fleece/src/campaign/internal/application/ports/input"
	"fleece/src/campaign/internal/application/ports/output"
	"fleece/src/campaign/internal/domain"
)

// EstimateCost estime le cout total d'une campagne.
//
// Algorithme :
//  1. Charger la campagne pour avoir la strategie de canal et le message_body.
//  2. Charger les destinataires en etat pending pour compter.
//  3. Appeler GetRoutingDecision sur le premier destinataire pour obtenir le cout unitaire.
//     (Le premier destinataire est representatif ; l'estimation n'est pas garantie exacte.)
//  4. Multiplier le cout unitaire par le nombre de destinataires.
//  5. Persister l'estimation dans estimated_cost (champ de la campagne).
//
// Fallback gracieux : si le RoutingClient echoue, retourne une estimation a 0 sans erreur.
type EstimateCost struct {
	Campaigns  output.CampaignRepository
	Recipients output.RecipientRepository
	Routing    output.RoutingClient
}

// Execute calcule et retourne l'estimation de cout de la campagne.
func (uc *EstimateCost) Execute(ctx context.Context, cmd input.EstimateCostCommand) (*domain.Money, error) {
	if cmd.CampaignID == "" {
		return nil, fmt.Errorf("estimate_cost: campaign_id requis")
	}

	campaign, err := uc.Campaigns.Get(ctx, cmd.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("estimate_cost: charger campagne: %w", err)
	}

	recipients, err := uc.Recipients.ListPendingByCampaign(ctx, cmd.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("estimate_cost: charger destinataires: %w", err)
	}

	if len(recipients) == 0 {
		zero, _ := domain.NewMoney(0, "XAF")
		return &zero, nil
	}

	// Obtenir le cout unitaire via le service routing.
	// On utilise le premier destinataire comme representatif pour l'estimation.
	// Le recipient est passe pour beneficier de l'enrichissement contact-intelligence (D28).
	firstRecipient := recipients[0].Recipient
	country := cmd.Country
	if country == "" {
		country = "CM" // defaut Cameroun si pays absent
	}

	_, _, costAmount, currency, routingErr := uc.Routing.GetRoutingDecision(
		ctx, cmd.WorkspaceID, campaign.ChannelStrategy, country, firstRecipient,
	)
	if routingErr != nil {
		// Fallback gracieux : cout inconnu, on retourne 0 sans bloquer.
		zero, _ := domain.NewMoney(0, "XAF")
		return &zero, nil
	}

	if currency == "" {
		currency = "XAF"
	}

	unitCost, err := domain.NewMoney(costAmount, currency)
	if err != nil {
		zero, _ := domain.NewMoney(0, currency)
		return &zero, nil
	}

	// Estimation = cout unitaire x nombre de destinataires pending.
	totalCost, err := unitCost.Multiply(int64(len(recipients)))
	if err != nil {
		return nil, fmt.Errorf("estimate_cost: multiplier: %w", err)
	}

	// Persister l'estimation dans la campagne.
	campaign.EstimatedCost = totalCost
	if saveErr := uc.Campaigns.Save(ctx, campaign); saveErr != nil {
		// Non bloquant : retourner quand meme l'estimation.
		fmt.Printf("estimate_cost: persister estimation: %v\n", saveErr)
	}

	return &totalCost, nil
}
