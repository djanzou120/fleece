package usecases

import (
	"context"
	"fmt"

	"fleece/src/campaign/internal/application/ports/output"
	"fleece/src/campaign/internal/domain"
)

// PauseCampaign suspend une campagne en cours (running → paused).
type PauseCampaign struct {
	Campaigns output.CampaignRepository
}

// Execute suspend la campagne.
func (uc *PauseCampaign) Execute(ctx context.Context, campaignID string) error {
	campaign, err := uc.Campaigns.Get(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("pause_campaign: charger campagne %s: %w", campaignID, err)
	}

	if err := campaign.Pause(); err != nil {
		return err
	}

	if err := uc.Campaigns.Save(ctx, campaign); err != nil {
		return fmt.Errorf("pause_campaign: persister %s: %w", campaignID, err)
	}

	return nil
}

// CancelCampaign annule une campagne (draft|scheduled|paused → cancelled).
type CancelCampaign struct {
	Campaigns output.CampaignRepository
}

// Execute annule la campagne.
func (uc *CancelCampaign) Execute(ctx context.Context, campaignID string) error {
	campaign, err := uc.Campaigns.Get(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("cancel_campaign: charger campagne %s: %w", campaignID, err)
	}

	if err := campaign.Cancel(); err != nil {
		return err
	}

	if err := uc.Campaigns.Save(ctx, campaign); err != nil {
		return fmt.Errorf("cancel_campaign: persister %s: %w", campaignID, err)
	}

	return nil
}

// GetCampaignStatus retourne l'etat courant d'une campagne.
type GetCampaignStatus struct {
	Campaigns output.CampaignRepository
}

// Execute retourne la campagne avec son statut courant.
func (uc *GetCampaignStatus) Execute(ctx context.Context, campaignID string) (*domain.Campaign, error) {
	campaign, err := uc.Campaigns.Get(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("get_campaign_status: %w", err)
	}
	return campaign, nil
}
