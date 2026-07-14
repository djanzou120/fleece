package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"fleece/src/campaign/internal/application/ports/input"
	"fleece/src/campaign/internal/domain"
)

// ─── Interfaces locales des use cases (evite l'import des ports input) ────────

type createCampaignUC interface {
	Execute(ctx context.Context, cmd input.CreateCampaignCommand) (*domain.Campaign, error)
}

type importRecipientsUC interface {
	Execute(ctx context.Context, cmd input.ImportRecipientsCommand) (int64, error)
}

type estimateCostUC interface {
	Execute(ctx context.Context, cmd input.EstimateCostCommand) (*domain.Money, error)
}

type scheduleCampaignUC interface {
	Execute(ctx context.Context, cmd input.ScheduleCampaignCommand) (*domain.Campaign, error)
}

type runCampaignUC interface {
	Execute(ctx context.Context, campaignID string) error
}

type pauseCampaignUC interface {
	Execute(ctx context.Context, campaignID string) error
}

type cancelCampaignUC interface {
	Execute(ctx context.Context, campaignID string) error
}

type getCampaignStatusUC interface {
	Execute(ctx context.Context, campaignID string) (*domain.Campaign, error)
}

// ─── Handler ──────────────────────────────────────────────────────────────────

// CampaignHandler regroupe les handlers REST du service campaign.
// Il utilise r.PathValue() (net/http stdlib >= 1.22, conforme au projet).
type CampaignHandler struct {
	create        createCampaignUC
	importRecip   importRecipientsUC
	estimateCost  estimateCostUC
	schedule      scheduleCampaignUC
	run           runCampaignUC
	pause         pauseCampaignUC
	cancel        cancelCampaignUC
	getStatus     getCampaignStatusUC
}

// NewCampaignHandler cree un CampaignHandler avec tous les use cases injectes.
func NewCampaignHandler(
	create createCampaignUC,
	importRecip importRecipientsUC,
	estimateCost estimateCostUC,
	schedule scheduleCampaignUC,
	run runCampaignUC,
	pause pauseCampaignUC,
	cancel cancelCampaignUC,
	getStatus getCampaignStatusUC,
) *CampaignHandler {
	return &CampaignHandler{
		create:       create,
		importRecip:  importRecip,
		estimateCost: estimateCost,
		schedule:     schedule,
		run:          run,
		pause:        pause,
		cancel:       cancel,
		getStatus:    getStatus,
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[campaign:handler] writeJSON encode error: %v", err)
	}
}

// mapError mappe les erreurs domaine en codes HTTP.
func mapError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrCampaignNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInvalidTransition):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrMissingMessageBody):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrMissingSchedule):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	default:
		log.Printf("[campaign:handler] erreur interne: %v", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "erreur interne"})
	}
}

// campaignToDTO convertit une entite domaine en DTO de reponse.
func campaignToDTO(c *domain.Campaign) CampaignResponse {
	cur := c.EstimatedCost.Currency
	if cur == "" {
		cur = "XAF"
	}
	return CampaignResponse{
		ID:              c.ID,
		WorkspaceID:     c.WorkspaceID,
		Name:            c.Name,
		Status:          string(c.Status),
		ScheduledAt:     c.ScheduledAt,
		CreatedAt:       c.CreatedAt,
		MessageBody:     c.MessageBody,
		ChannelStrategy: c.ChannelStrategy,
		EstimatedCost:   c.EstimatedCost.Amount,
		Currency:        cur,
		RateLimit:       c.RateLimit,
	}
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// CreateCampaign traite POST /campaigns.
func (h *CampaignHandler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req CreateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c, err := h.create.Execute(r.Context(), input.CreateCampaignCommand{
		WorkspaceID:     req.WorkspaceID,
		Name:            req.Name,
		ChannelStrategy: req.ChannelStrategy,
		MessageBody:     req.MessageBody,
		RateLimit:       req.RateLimit,
	})
	if err != nil {
		mapError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, campaignToDTO(c))
}

// ImportRecipients traite POST /campaigns/{id}/recipients.
func (h *CampaignHandler) ImportRecipients(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("id")
	if campaignID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "id est requis dans le chemin"})
		return
	}

	var req ImportRecipientsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	inserted, err := h.importRecip.Execute(r.Context(), input.ImportRecipientsCommand{
		CampaignID: campaignID,
		Recipients: req.Recipients,
	})
	if err != nil {
		mapError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, ImportRecipientsResponse{Inserted: inserted})
}

// EstimateCost traite POST /campaigns/{id}/estimate.
func (h *CampaignHandler) EstimateCost(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("id")
	if campaignID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "id est requis dans le chemin"})
		return
	}

	var req EstimateCostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	cost, err := h.estimateCost.Execute(r.Context(), input.EstimateCostCommand{
		CampaignID:  campaignID,
		WorkspaceID: req.WorkspaceID,
		Country:     req.Country,
	})
	if err != nil {
		mapError(w, err)
		return
	}

	cur := cost.Currency
	if cur == "" {
		cur = "XAF"
	}
	writeJSON(w, http.StatusOK, EstimateCostResponse{
		EstimatedCost: cost.Amount,
		Currency:      cur,
	})
}

// ScheduleCampaign traite POST /campaigns/{id}/schedule.
func (h *CampaignHandler) ScheduleCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("id")
	if campaignID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "id est requis dans le chemin"})
		return
	}

	var req ScheduleCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c, err := h.schedule.Execute(r.Context(), input.ScheduleCampaignCommand{
		CampaignID:  campaignID,
		MessageBody: req.MessageBody,
		ScheduledAt: req.ScheduledAt,
	})
	if err != nil {
		mapError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, campaignToDTO(c))
}

// RunCampaign traite POST /campaigns/{id}/run.
func (h *CampaignHandler) RunCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("id")
	if campaignID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "id est requis dans le chemin"})
		return
	}

	if err := h.run.Execute(r.Context(), campaignID); err != nil {
		mapError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// PauseCampaign traite POST /campaigns/{id}/pause.
func (h *CampaignHandler) PauseCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("id")
	if campaignID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "id est requis dans le chemin"})
		return
	}

	if err := h.pause.Execute(r.Context(), campaignID); err != nil {
		mapError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CancelCampaign traite POST /campaigns/{id}/cancel.
func (h *CampaignHandler) CancelCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("id")
	if campaignID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "id est requis dans le chemin"})
		return
	}

	if err := h.cancel.Execute(r.Context(), campaignID); err != nil {
		mapError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetCampaignStatus traite GET /campaigns/{id}.
func (h *CampaignHandler) GetCampaignStatus(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("id")
	if campaignID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "id est requis dans le chemin"})
		return
	}

	c, err := h.getStatus.Execute(r.Context(), campaignID)
	if err != nil {
		mapError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, campaignToDTO(c))
}

