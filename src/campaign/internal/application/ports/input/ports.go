// Package input contient les ports pilotants (driving) du service Campaign.
// Ce sont les interfaces que les handlers HTTP et les consumers RabbitMQ utilisent
// pour piloter les use cases.
// Les implementations concretes vivent dans application/usecases/.
package input

import (
	"context"
	"time"

	"fleece/src/campaign/internal/domain"
)

// CreateCampaignUseCase cree une campagne en etat draft.
type CreateCampaignUseCase interface {
	Execute(ctx context.Context, cmd CreateCampaignCommand) (*domain.Campaign, error)
}

// CreateCampaignCommand contient les parametres de creation d'une campagne.
type CreateCampaignCommand struct {
	WorkspaceID     string
	Name            string
	ChannelStrategy string // ex. "lowest_cost", "highest_delivery"
	MessageBody     string
	RateLimit       int
}

// ImportRecipientsUseCase importe les destinataires d'une campagne (dedup).
type ImportRecipientsUseCase interface {
	Execute(ctx context.Context, cmd ImportRecipientsCommand) (inserted int64, err error)
}

// ImportRecipientsCommand contient les parametres d'import de destinataires.
type ImportRecipientsCommand struct {
	CampaignID string
	Recipients []string
}

// EstimateCostUseCase estime le cout total d'une campagne
// (cout unitaire via routing x nombre de destinataires pending).
type EstimateCostUseCase interface {
	Execute(ctx context.Context, cmd EstimateCostCommand) (*domain.Money, error)
}

// EstimateCostCommand contient les parametres d'estimation de cout.
type EstimateCostCommand struct {
	CampaignID  string
	WorkspaceID string
	Country     string // pays de destination (pour la tarification)
}

// ScheduleCampaignUseCase planifie une campagne (draft → scheduled).
type ScheduleCampaignUseCase interface {
	Execute(ctx context.Context, cmd ScheduleCampaignCommand) (*domain.Campaign, error)
}

// ScheduleCampaignCommand contient les parametres de planification.
type ScheduleCampaignCommand struct {
	CampaignID  string
	MessageBody string
	ScheduledAt time.Time
}

// RunCampaignUseCase execute une campagne planifiee.
// Appele par le scheduler ou par un endpoint HTTP.
type RunCampaignUseCase interface {
	Execute(ctx context.Context, campaignID string) error
}

// PauseCampaignUseCase suspend une campagne en cours.
type PauseCampaignUseCase interface {
	Execute(ctx context.Context, campaignID string) error
}

// CancelCampaignUseCase annule une campagne.
type CancelCampaignUseCase interface {
	Execute(ctx context.Context, campaignID string) error
}

// GetCampaignStatusUseCase retourne l'etat courant d'une campagne.
type GetCampaignStatusUseCase interface {
	Execute(ctx context.Context, campaignID string) (*domain.Campaign, error)
}

// RecordDeliveryOutcomeUseCase met a jour le statut d'un destinataire sur reception d'un DLR.
type RecordDeliveryOutcomeUseCase interface {
	Execute(ctx context.Context, cmd RecordDeliveryOutcomeCommand) error
}

// RecordDeliveryOutcomeCommand contient les parametres d'un DLR.
type RecordDeliveryOutcomeCommand struct {
	// MessageID est l'identifiant du message (peut etre vide si absent du DLR).
	// Si vide, l'outcome est ignore (log warning, pas d'erreur).
	MessageID string
	// Success indique si le message a ete livre (true) ou a echoue (false).
	Success bool
}
