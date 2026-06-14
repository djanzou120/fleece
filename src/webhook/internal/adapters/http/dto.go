// Package http expose les use cases webhook via une API REST (net/http).
// Couche 3 (Interface Adapters, driving).
package http

import "fmt"

// RegisterEndpointRequest est le DTO d'entree de POST /endpoints.
type RegisterEndpointRequest struct {
	WorkspaceID string   `json:"workspace_id"`
	URL         string   `json:"url"`
	Secret      string   `json:"secret,omitempty"` // optionnel : genere si absent
	Events      []string `json:"events"`
}

// validate verifie que les champs obligatoires sont presents.
func (r *RegisterEndpointRequest) validate() error {
	if r.WorkspaceID == "" {
		return fmt.Errorf("workspace_id est requis")
	}
	if r.URL == "" {
		return fmt.Errorf("url est requise")
	}
	if len(r.Events) == 0 {
		return fmt.Errorf("au moins un evenement est requis")
	}
	return nil
}

// EndpointResponse est le DTO de sortie representant un endpoint webhook.
type EndpointResponse struct {
	ID          string   `json:"id"`
	WorkspaceID string   `json:"workspace_id"`
	URL         string   `json:"url"`
	Secret      string   `json:"secret"`
	Events      []string `json:"events"`
	Active      bool     `json:"active"`
}

// ListEndpointsResponse est le DTO de sortie de GET /endpoints.
type ListEndpointsResponse struct {
	Endpoints []EndpointResponse `json:"endpoints"`
}
