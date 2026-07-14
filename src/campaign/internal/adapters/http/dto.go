// Package http expose les use cases Campaign via une API REST (net/http).
// Couche 3 (Interface Adapters, driving).
package http

import (
	"fmt"
	"time"
)

// ─── Request DTOs ─────────────────────────────────────────────────────────────

// CreateCampaignRequest est le DTO d'entree de POST /campaigns.
type CreateCampaignRequest struct {
	WorkspaceID     string `json:"workspace_id"`
	Name            string `json:"name"`
	ChannelStrategy string `json:"channel_strategy"`
	MessageBody     string `json:"message_body"`
	RateLimit       int    `json:"rate_limit"`
}

func (r *CreateCampaignRequest) validate() error {
	if r.WorkspaceID == "" {
		return fmt.Errorf("workspace_id est requis")
	}
	if r.Name == "" {
		return fmt.Errorf("name est requis")
	}
	return nil
}

// ImportRecipientsRequest est le DTO d'entree de POST /campaigns/{id}/recipients.
type ImportRecipientsRequest struct {
	Recipients []string `json:"recipients"`
}

func (r *ImportRecipientsRequest) validate() error {
	if len(r.Recipients) == 0 {
		return fmt.Errorf("recipients ne peut pas etre vide")
	}
	return nil
}

// EstimateCostRequest est le DTO d'entree de POST /campaigns/{id}/estimate.
type EstimateCostRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Country     string `json:"country"`
}

func (r *EstimateCostRequest) validate() error {
	if r.WorkspaceID == "" {
		return fmt.Errorf("workspace_id est requis")
	}
	return nil
}

// ScheduleCampaignRequest est le DTO d'entree de POST /campaigns/{id}/schedule.
type ScheduleCampaignRequest struct {
	MessageBody string    `json:"message_body"`
	ScheduledAt time.Time `json:"scheduled_at"`
}

func (r *ScheduleCampaignRequest) validate() error {
	if r.ScheduledAt.IsZero() {
		return fmt.Errorf("scheduled_at est requis")
	}
	return nil
}

// ─── Response DTOs ────────────────────────────────────────────────────────────

// CampaignResponse est le DTO de sortie d'une campagne.
type CampaignResponse struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	ScheduledAt     *time.Time `json:"scheduled_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	MessageBody     string     `json:"message_body,omitempty"`
	ChannelStrategy string     `json:"channel_strategy"`
	EstimatedCost   int64      `json:"estimated_cost"`
	Currency        string     `json:"currency"`
	RateLimit       int        `json:"rate_limit"`
}

// ImportRecipientsResponse est le DTO de sortie de POST /campaigns/{id}/recipients.
type ImportRecipientsResponse struct {
	Inserted int64 `json:"inserted"`
}

// EstimateCostResponse est le DTO de sortie de POST /campaigns/{id}/estimate.
type EstimateCostResponse struct {
	EstimatedCost int64  `json:"estimated_cost"`
	Currency      string `json:"currency"`
}

// ErrorResponse est le format JSON des erreurs.
type ErrorResponse struct {
	Error string `json:"error"`
}
