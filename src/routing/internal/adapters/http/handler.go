package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"fleece/src/routing/internal/domain"
)

// getRoutingDecisionUseCase est l'interface locale du use case GetRoutingDecision.
// Definie ici (couche 3) pour eviter une dependance vers les ports input (couche 2)
// et permettre des mocks faciles dans les tests du handler.
type getRoutingDecisionUseCase interface {
	Execute(ctx context.Context, workspaceID, channel, country string, recipientCount int) (domain.RoutingDecision, error)
}

// updateProviderScoreUseCase est l'interface locale du use case UpdateProviderScore.
type updateProviderScoreUseCase interface {
	Execute(ctx context.Context, providerID, channel string, score int) error
}

// RoutingHandler regroupe les handlers REST du service routing.
type RoutingHandler struct {
	getDecision  getRoutingDecisionUseCase
	updateScore  updateProviderScoreUseCase
}

// NewRoutingHandler cree un RoutingHandler avec les use cases injectes.
func NewRoutingHandler(getDecision getRoutingDecisionUseCase, updateScore updateProviderScoreUseCase) *RoutingHandler {
	return &RoutingHandler{
		getDecision: getDecision,
		updateScore: updateScore,
	}
}

// errorResponse est le format JSON des erreurs retournees par l'API.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON serialise v en JSON et l'ecrit dans la reponse avec le code HTTP donne.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("handler: writeJSON encode error: %v", err)
	}
}

// Route traite POST /route.
//
// Mappage des erreurs metier en codes HTTP :
//   - validation                → 400
//   - ErrInvalidStrategy        → 400
//   - ErrNoProviderAvailable    → 422
//   - autres                    → 500
func (h *RoutingHandler) Route(w http.ResponseWriter, r *http.Request) {
	var req RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	decision, err := h.getDecision.Execute(r.Context(), req.WorkspaceID, req.Channel, req.Country, req.RecipientCount)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Construire la liste des providers de repli (juste les IDs).
	fallback := make([]string, 0, len(decision.FallbackChain))
	for _, ref := range decision.FallbackChain {
		fallback = append(fallback, ref.ProviderID)
	}

	writeJSON(w, http.StatusOK, RouteResponse{
		ProviderID:    decision.ProviderID,
		Channel:       string(decision.Channel),
		EstimatedCost: decision.EstimatedCost.Amount,
		Currency:      decision.EstimatedCost.Currency,
		Strategy:      string(decision.Strategy),
		FallbackChain: fallback,
	})
}

// ScoreFeedback traite POST /scores.
//
// Mappage des erreurs metier en codes HTTP :
//   - validation → 400
//   - autres     → 500
func (h *RoutingHandler) ScoreFeedback(w http.ResponseWriter, r *http.Request) {
	var req ScoreFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := h.updateScore.Execute(r.Context(), req.ProviderID, req.Channel, req.Score); err != nil {
		h.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleError centralise le mappage des erreurs domaine en codes HTTP.
func (h *RoutingHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNoProviderAvailable):
		// 422 Unprocessable Entity : la requete est valide mais aucun provider ne peut la traiter.
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInvalidStrategy):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	default:
		log.Printf("handler: erreur interne: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "erreur interne"})
	}
}
