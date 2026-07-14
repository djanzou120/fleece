package domain

import "time"

// CampaignRun represente une execution d'une campagne.
// Chaque campagne peut avoir plusieurs runs (ex. reprise apres pause).
// Stocke les compteurs d'avancement : total, sent, delivered, failed.
type CampaignRun struct {
	// ID est la cle primaire bigserial.
	ID int64
	// CampaignID est la cle etrangere vers la campagne.
	CampaignID string
	// StartedAt est le debut de l'execution.
	StartedAt time.Time
	// FinishedAt est la fin de l'execution (nil si en cours).
	FinishedAt *time.Time
	// TotalCount est le nombre total de destinataires a traiter.
	TotalCount int
	// SentCount est le nombre de messages soumis au provider.
	SentCount int
	// DeliveredCount est le nombre de messages livres (DLR confirme).
	DeliveredCount int
	// FailedCount est le nombre de messages en echec.
	FailedCount int
}

// NewCampaignRun cree un CampaignRun au debut d'une execution.
func NewCampaignRun(campaignID string, totalCount int) *CampaignRun {
	return &CampaignRun{
		CampaignID: campaignID,
		StartedAt:  time.Now().UTC(),
		TotalCount: totalCount,
	}
}

// Finish marque le run comme termine.
func (r *CampaignRun) Finish() {
	now := time.Now().UTC()
	r.FinishedAt = &now
}
