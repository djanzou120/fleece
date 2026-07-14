package domain

import (
	"fmt"
	"time"
)

// CampaignStatus represente l'etat courant d'une campagne marketing.
type CampaignStatus string

const (
	// StatusDraft : campagne en cours de creation, non planifiee.
	StatusDraft CampaignStatus = "draft"
	// StatusScheduled : campagne planifiee, en attente d'execution.
	StatusScheduled CampaignStatus = "scheduled"
	// StatusRunning : campagne en cours d'envoi.
	StatusRunning CampaignStatus = "running"
	// StatusCompleted : campagne terminee avec succes.
	StatusCompleted CampaignStatus = "completed"
	// StatusFailed : campagne terminee en erreur.
	StatusFailed CampaignStatus = "failed"
	// StatusPaused : campagne suspendue (depuis running).
	StatusPaused CampaignStatus = "paused"
	// StatusCancelled : campagne annulee (depuis draft, scheduled ou paused).
	StatusCancelled CampaignStatus = "cancelled"
)

// Campaign est l'entite racine du domaine Campaign.
// Elle encapsule la machine a etats et les invariants metier.
//
// Machine a etats :
//
//	draft → scheduled  (ScheduleCampaign : message_body != "" ET scheduled_at != nil)
//	draft → cancelled
//	scheduled → running    (StartRun)
//	scheduled → cancelled
//	running → completed    (Complete)
//	running → failed       (Fail)
//	running → paused       (Pause)
//	paused  → running      (Resume)
//	paused  → cancelled
type Campaign struct {
	ID              string
	WorkspaceID     string
	Name            string
	Status          CampaignStatus
	ScheduledAt     *time.Time
	CreatedAt       time.Time
	MessageBody     string
	ChannelStrategy string
	EstimatedCost   Money
	RateLimit       int // messages/min ; 0 = illimite
}

// NewCampaign cree une campagne en etat draft.
func NewCampaign(id, workspaceID, name, channelStrategy string) *Campaign {
	return &Campaign{
		ID:              id,
		WorkspaceID:     workspaceID,
		Name:            name,
		Status:          StatusDraft,
		ChannelStrategy: channelStrategy,
		CreatedAt:       time.Now().UTC(),
	}
}

// Schedule planifie la campagne (draft → scheduled).
//
// Gardes :
//   - message_body doit etre non vide (ErrMissingMessageBody).
//   - scheduledAt doit etre non nil (ErrMissingSchedule).
//   - La campagne doit etre en etat draft (ErrInvalidTransition).
func (c *Campaign) Schedule(scheduledAt time.Time) error {
	if c.Status != StatusDraft {
		return fmt.Errorf("campaign %s (status=%s): %w", c.ID, c.Status, ErrInvalidTransition)
	}
	if c.MessageBody == "" {
		return ErrMissingMessageBody
	}
	c.ScheduledAt = &scheduledAt
	c.Status = StatusScheduled
	return nil
}

// StartRun demarre l'execution de la campagne (scheduled → running).
// Retourne ErrInvalidTransition si la campagne n'est pas en etat scheduled.
func (c *Campaign) StartRun() error {
	if c.Status != StatusScheduled {
		return fmt.Errorf("campaign %s (status=%s): %w", c.ID, c.Status, ErrInvalidTransition)
	}
	c.Status = StatusRunning
	return nil
}

// Complete marque la campagne comme terminee (running → completed).
// Retourne ErrInvalidTransition si la campagne n'est pas en etat running.
func (c *Campaign) Complete() error {
	if c.Status != StatusRunning {
		return fmt.Errorf("campaign %s (status=%s): %w", c.ID, c.Status, ErrInvalidTransition)
	}
	c.Status = StatusCompleted
	return nil
}

// Fail marque la campagne comme echouee (running → failed).
// Retourne ErrInvalidTransition si la campagne n'est pas en etat running.
func (c *Campaign) Fail() error {
	if c.Status != StatusRunning {
		return fmt.Errorf("campaign %s (status=%s): %w", c.ID, c.Status, ErrInvalidTransition)
	}
	c.Status = StatusFailed
	return nil
}

// Pause suspend la campagne (running → paused).
// Retourne ErrInvalidTransition si la campagne n'est pas en etat running.
func (c *Campaign) Pause() error {
	if c.Status != StatusRunning {
		return fmt.Errorf("campaign %s (status=%s): %w", c.ID, c.Status, ErrInvalidTransition)
	}
	c.Status = StatusPaused
	return nil
}

// Resume reprend la campagne suspendue (paused → running).
// Retourne ErrInvalidTransition si la campagne n'est pas en etat paused.
func (c *Campaign) Resume() error {
	if c.Status != StatusPaused {
		return fmt.Errorf("campaign %s (status=%s): %w", c.ID, c.Status, ErrInvalidTransition)
	}
	c.Status = StatusRunning
	return nil
}

// Cancel annule la campagne (draft|scheduled|paused → cancelled).
// Retourne ErrInvalidTransition pour les autres etats.
func (c *Campaign) Cancel() error {
	switch c.Status {
	case StatusDraft, StatusScheduled, StatusPaused:
		c.Status = StatusCancelled
		return nil
	default:
		return fmt.Errorf("campaign %s (status=%s): %w", c.ID, c.Status, ErrInvalidTransition)
	}
}
