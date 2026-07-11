// Package http contient les handlers REST et les DTOs du service Contact Intelligence.
// Couche 3 (Interface Adapters).
package http

import "time"

// ─── DTOs de reponse ─────────────────────────────────────────────────────────

// ChannelScoreDTO represente le score de delivrabilite d'un canal dans la reponse JSON.
type ChannelScoreDTO struct {
	Channel       string     `json:"channel"`
	Score         int        `json:"score"`
	SampleSize    int        `json:"sample_size"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
}

// ContactScoreResponse est la reponse de GET /contacts/{phone}/score.
//
// Contrat JSON consomme par le Routing Service (T-016) :
//
//	{
//	  "phone":          "+237690000000",
//	  "delivery_score": 72,
//	  "found":          true,
//	  "channel_scores": [
//	    {"channel": "whatsapp", "score": 85, "sample_size": 20, "last_success_at": "..."},
//	    {"channel": "sms",      "score": 60, "sample_size": 5}
//	  ]
//	}
//
// Si found=false (contact inconnu), delivery_score=50 (prior neutre) et
// channel_scores est un tableau vide.
type ContactScoreResponse struct {
	Phone         string            `json:"phone"`
	DeliveryScore int               `json:"delivery_score"`
	Found         bool              `json:"found"`
	ChannelScores []ChannelScoreDTO `json:"channel_scores"`
}

// ─── DTOs de requete ─────────────────────────────────────────────────────────

// RecordOutcomeRequest est le body de POST /outcomes.
type RecordOutcomeRequest struct {
	Phone   string `json:"phone"`
	Channel string `json:"channel"`
	Success bool   `json:"success"`
}

// validate verifie que les champs obligatoires sont presents.
func (r *RecordOutcomeRequest) validate() error {
	if r.Phone == "" {
		return errMissingField("phone")
	}
	if r.Channel == "" {
		return errMissingField("channel")
	}
	return nil
}

// ─── Erreur de validation ────────────────────────────────────────────────────

type validationError struct{ msg string }

func (e validationError) Error() string { return e.msg }

func errMissingField(field string) error {
	return validationError{msg: field + " est requis"}
}
