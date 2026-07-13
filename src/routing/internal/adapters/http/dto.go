// Package http expose les use cases Routing via une API REST (net/http).
// Couche 3 (Interface Adapters, driving).
package http

import "fmt"

// RouteRequest est le DTO d'entree de l'endpoint POST /route.
//
// Champ optionnel ajoute en T-016 :
//   - Recipient : numero de telephone du destinataire (format E.164, ex. "+237612345678").
//     Si present et non vide, le use case GetRoutingDecision l'utilise UNIQUEMENT pour
//     la strategie highest_delivery afin d'enrichir le score provider avec le score de
//     joignabilite du contact (service contact-intelligence).
//     Si absent (vide ou omis), l'enrichissement est simplement desactive : comportement
//     inchange pour les appelants existants (retro-compatible).
type RouteRequest struct {
	WorkspaceID    string `json:"workspace_id"`
	Channel        string `json:"channel"`
	Country        string `json:"country"`
	RecipientCount int    `json:"recipient_count"`
	// Recipient est le numero de telephone du destinataire (optionnel).
	// Utilise uniquement pour la strategie highest_delivery (enrichissement contact T-016).
	// Laisser vide pour desactiver l'enrichissement (retrocompatible).
	Recipient string `json:"recipient,omitempty"`
}

// validate verifie que les champs obligatoires sont presents et valides.
func (r *RouteRequest) validate() error {
	if r.WorkspaceID == "" {
		return fmt.Errorf("workspace_id est requis")
	}
	if r.Channel == "" {
		return fmt.Errorf("channel est requis")
	}
	if r.Country == "" {
		return fmt.Errorf("country est requis")
	}
	if r.RecipientCount < 0 {
		return fmt.Errorf("recipient_count doit etre positif ou nul")
	}
	return nil
}

// RouteResponse est le DTO de sortie de l'endpoint POST /route.
type RouteResponse struct {
	ProviderID    string   `json:"provider_id"`
	Channel       string   `json:"channel"`
	EstimatedCost int64    `json:"estimated_cost"`
	Currency      string   `json:"currency"`
	Strategy      string   `json:"strategy"`
	FallbackChain []string `json:"fallback_chain"`
}

// ScoreFeedbackRequest est le DTO d'entree de l'endpoint POST /scores.
type ScoreFeedbackRequest struct {
	ProviderID string `json:"provider_id"`
	Channel    string `json:"channel"`
	Score      int    `json:"score"`
}

// validate verifie que les champs obligatoires sont presents et valides.
func (r *ScoreFeedbackRequest) validate() error {
	if r.ProviderID == "" {
		return fmt.Errorf("provider_id est requis")
	}
	if r.Channel == "" {
		return fmt.Errorf("channel est requis")
	}
	if r.Score < 0 || r.Score > 100 {
		return fmt.Errorf("score doit etre compris entre 0 et 100")
	}
	return nil
}
