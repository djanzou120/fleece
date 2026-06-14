// Package http expose les use cases Provider via une API REST (net/http).
// Couche 3 (Interface Adapters, driving).
package http

import "fmt"

// DispatchRequest est le DTO d'entree de l'endpoint POST /dispatch.
type DispatchRequest struct {
	WorkspaceID string            `json:"workspace_id"`
	ProviderID  string            `json:"provider_id"`
	MessageID   string            `json:"message_id"`
	Channel     string            `json:"channel"`
	Recipient   string            `json:"recipient"`
	Content     string            `json:"content"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// validate verifie que les champs obligatoires sont presents.
func (r *DispatchRequest) validate() error {
	if r.WorkspaceID == "" {
		return fmt.Errorf("workspace_id est requis")
	}
	if r.ProviderID == "" {
		return fmt.Errorf("provider_id est requis")
	}
	if r.MessageID == "" {
		return fmt.Errorf("message_id est requis")
	}
	if r.Channel == "" {
		return fmt.Errorf("channel est requis")
	}
	if r.Recipient == "" {
		return fmt.Errorf("recipient est requis")
	}
	if r.Content == "" {
		return fmt.Errorf("content est requis")
	}
	return nil
}

// DispatchResponse est le DTO de sortie de l'endpoint POST /dispatch.
type DispatchResponse struct {
	ExternalID string `json:"external_id"`
}

// DLRRequest est le DTO d'entree de l'endpoint POST /dlr (delivery receipt entrant).
type DLRRequest struct {
	ExternalID string `json:"external_id"`
	Status     string `json:"status"`
}

// validate verifie que les champs obligatoires sont presents.
func (r *DLRRequest) validate() error {
	if r.ExternalID == "" {
		return fmt.Errorf("external_id est requis")
	}
	if r.Status == "" {
		return fmt.Errorf("status est requis")
	}
	return nil
}
