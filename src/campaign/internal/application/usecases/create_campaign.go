package usecases

import (
	"context"
	"fmt"
	"time"

	"fleece/src/campaign/internal/application/ports/input"
	"fleece/src/campaign/internal/application/ports/output"
	"fleece/src/campaign/internal/domain"
)

// CreateCampaign cree une campagne en etat draft.
// Genere un ID temporel (UUID serait preferable, utilise ici une string horodatee
// pour rester stdlib-only sans dependance uuid).
type CreateCampaign struct {
	Campaigns output.CampaignRepository
	// IDGenerator permet de surcharger la generation d'ID dans les tests.
	// Si nil, utilise generateID() par defaut.
	IDGenerator func() string
}

// Execute cree et persiste une campagne en etat draft.
func (uc *CreateCampaign) Execute(ctx context.Context, cmd input.CreateCampaignCommand) (*domain.Campaign, error) {
	if cmd.WorkspaceID == "" {
		return nil, fmt.Errorf("create_campaign: workspace_id requis")
	}
	if cmd.Name == "" {
		return nil, fmt.Errorf("create_campaign: name requis")
	}

	strategy := cmd.ChannelStrategy
	if strategy == "" {
		strategy = "lowest_cost"
	}

	idGen := uc.IDGenerator
	if idGen == nil {
		idGen = generateID
	}

	c := domain.NewCampaign(idGen(), cmd.WorkspaceID, cmd.Name, strategy)
	c.MessageBody = cmd.MessageBody
	c.RateLimit = cmd.RateLimit

	if err := uc.Campaigns.Save(ctx, c); err != nil {
		return nil, fmt.Errorf("create_campaign: persister: %w", err)
	}
	return c, nil
}

// generateID produit un identifiant simple base sur l'horodatage nanoseconde.
// En production, remplacer par un UUID v4 (go.mod inchange oblige).
func generateID() string {
	return fmt.Sprintf("cmp-%d", time.Now().UnixNano())
}
