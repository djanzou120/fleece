package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"fleece/src/contact-intelligence/internal/application/usecases"
	"fleece/src/contact-intelligence/internal/domain"
)

// getContactScoreUseCase est l'interface locale du use case GetContactScore.
type getContactScoreUseCase interface {
	Execute(ctx context.Context, phone string) (*usecases.ContactScoreResult, error)
}

// recordDeliveryOutcomeUseCase est l'interface locale du use case RecordDeliveryOutcome.
type recordDeliveryOutcomeUseCase interface {
	Execute(ctx context.Context, phone string, ch domain.Channel, success bool) error
}

// ContactIntelHandler regroupe les handlers REST du service contact-intelligence.
type ContactIntelHandler struct {
	getScore      getContactScoreUseCase
	recordOutcome recordDeliveryOutcomeUseCase
}

// NewContactIntelHandler cree un ContactIntelHandler.
func NewContactIntelHandler(
	getScore getContactScoreUseCase,
	recordOutcome recordDeliveryOutcomeUseCase,
) *ContactIntelHandler {
	return &ContactIntelHandler{
		getScore:      getScore,
		recordOutcome: recordOutcome,
	}
}

// errorResponse est le format JSON des erreurs.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON serialise v en JSON et l'ecrit dans la reponse avec le code HTTP donne.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[contact_intel:handler] writeJSON encode error: %v", err)
	}
}

// GetScore traite GET /contacts/{phone}/score.
//
// Retourne 200 + ContactScoreResponse (found=true ou false).
// Retourne 400 si phone est absent du chemin.
// Ne retourne jamais 404 : un contact inconnu → found=false + score neutre 50.
func (h *ContactIntelHandler) GetScore(w http.ResponseWriter, r *http.Request) {
	phone := r.PathValue("phone")
	if phone == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "phone est requis dans le chemin"})
		return
	}

	result, err := h.getScore.Execute(r.Context(), phone)
	if err != nil {
		if errors.Is(err, domain.ErrEmptyPhone) {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		log.Printf("[contact_intel:handler] GetScore phone=%s: %v", phone, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "erreur interne"})
		return
	}

	channelScores := make([]ChannelScoreDTO, 0, len(result.ChannelScores))
	for _, cs := range result.ChannelScores {
		channelScores = append(channelScores, ChannelScoreDTO{
			Channel:       string(cs.Channel),
			Score:         cs.Score.Value(),
			SampleSize:    cs.SampleSize,
			LastSuccessAt: cs.LastSuccessAt,
		})
	}

	writeJSON(w, http.StatusOK, ContactScoreResponse{
		Phone:         result.Phone,
		DeliveryScore: result.DeliveryScore.Value(),
		Found:         result.Found,
		ChannelScores: channelScores,
	})
}

// RecordOutcome traite POST /outcomes.
//
// Body JSON : {"phone": "...", "channel": "...", "success": true/false}.
// Retourne 204 si OK, 400 si payload invalide.
func (h *ContactIntelHandler) RecordOutcome(w http.ResponseWriter, r *http.Request) {
	var req RecordOutcomeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	ch := domain.Channel(req.Channel)
	if err := h.recordOutcome.Execute(r.Context(), req.Phone, ch, req.Success); err != nil {
		switch {
		case errors.Is(err, domain.ErrEmptyPhone):
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		case errors.Is(err, domain.ErrInvalidChannel):
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		default:
			log.Printf("[contact_intel:handler] RecordOutcome phone=%s channel=%s: %v", req.Phone, req.Channel, err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "erreur interne"})
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
