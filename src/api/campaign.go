package api

import (
	"errors"
	"fmt"
	"time"
)

// CampaignStatus represente l'etat courant d'une campagne (M-009).
type CampaignStatus string

const (
	// CampaignDraft : campagne en cours de creation, non planifiee.
	CampaignDraft CampaignStatus = "draft"
	// CampaignScheduled : campagne planifiee, en attente d'execution.
	CampaignScheduled CampaignStatus = "scheduled"
	// CampaignRunning : campagne en cours d'envoi.
	CampaignRunning CampaignStatus = "running"
	// CampaignCompleted : campagne terminee avec succes.
	CampaignCompleted CampaignStatus = "completed"
	// CampaignFailed : campagne terminee en erreur.
	CampaignFailed CampaignStatus = "failed"
	// CampaignCancelled : campagne annulee.
	CampaignCancelled CampaignStatus = "cancelled"
	// CampaignPaused : campagne suspendue (depuis running).
	CampaignPaused CampaignStatus = "paused"
)

// ErrMissingMessageBody est retournee quand on tente de planifier une campagne
// sans corps de message (message_body vide).
var ErrMissingMessageBody = errors.New("campaign: le corps du message est requis pour planifier")

// campaignTransitions definit les transitions autorisees pour la machine a etats Campaign.
//
// Machine a etats :
//
//	draft      → scheduled (garde : MessageBody != ""), cancelled
//	scheduled  → running, cancelled
//	running    → completed, failed, paused
//	paused     → running, cancelled
//	completed  → (terminal)
//	failed     → (terminal)
//	cancelled  → (terminal)
var campaignTransitions = map[CampaignStatus][]CampaignStatus{
	CampaignDraft:     {CampaignScheduled, CampaignCancelled},
	CampaignScheduled: {CampaignRunning, CampaignCancelled},
	CampaignRunning:   {CampaignCompleted, CampaignFailed, CampaignPaused},
	CampaignPaused:    {CampaignRunning, CampaignCancelled},
	CampaignCompleted: {},
	CampaignFailed:    {},
	CampaignCancelled: {},
}

// Campaign est l'entite representant une campagne d'envoi en masse.
type Campaign struct {
	ID              string
	WorkspaceID     string
	Name            string
	Status          CampaignStatus
	ScheduledAt     *time.Time
	CreatedAt       time.Time
	MessageBody     string
	ChannelStrategy string
	EstimatedCost   int64 // en centimes (coherent avec le service wallet, D8)
	RateLimit       int   // messages/min ; 0 = illimite
}

// NewCampaign cree une campagne a l'etat initial draft.
func NewCampaign(id, workspaceID, name, channelStrategy string) *Campaign {
	return &Campaign{
		ID:              id,
		WorkspaceID:     workspaceID,
		Name:            name,
		Status:          CampaignDraft,
		ChannelStrategy: channelStrategy,
		CreatedAt:       time.Now().UTC(),
	}
}

// Transition applique une transition d'etat. Retourne ErrInvalidTransition si
// la transition est interdite depuis l'etat courant.
//
// Note : pour la transition draft→scheduled, utiliser la methode Schedule() qui
// verifie en outre la garde metier (MessageBody non vide).
func (c *Campaign) Transition(next CampaignStatus) error {
	for _, allowed := range campaignTransitions[c.Status] {
		if allowed == next {
			c.Status = next
			return nil
		}
	}
	return fmt.Errorf("campaign %s (status=%s → %s): %w", c.ID, c.Status, next, ErrInvalidTransition)
}

// Schedule planifie la campagne (draft → scheduled).
//
// Gardes :
//   - MessageBody doit etre non vide (ErrMissingMessageBody).
//   - La campagne doit etre en etat draft (ErrInvalidTransition via Transition).
//
// scheduledAt est stocke dans ScheduledAt avant la transition.
func (c *Campaign) Schedule(scheduledAt time.Time) error {
	if c.MessageBody == "" {
		return ErrMissingMessageBody
	}
	c.ScheduledAt = &scheduledAt
	if err := c.Transition(CampaignScheduled); err != nil {
		// rollback : on retire la date si la transition echoue
		c.ScheduledAt = nil
		return err
	}
	return nil
}
