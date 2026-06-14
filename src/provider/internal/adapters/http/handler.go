package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"fleece/src/provider/internal/domain"
)

// dispatchUseCase est l'interface locale du use case Dispatch.
type dispatchUseCase interface {
	Execute(ctx context.Context, msg domain.OutboundMessage) (externalId string, err error)
}

// handleDLRUseCase est l'interface locale du use case HandleDLR.
type handleDLRUseCase interface {
	Execute(ctx context.Context, externalId string, status domain.DeliveryStatus) error
}

// ProviderHandler regroupe les handlers REST du service provider.
type ProviderHandler struct {
	dispatch  dispatchUseCase
	handleDLR handleDLRUseCase
}

// NewProviderHandler cree un ProviderHandler avec les use cases injectes.
func NewProviderHandler(dispatch dispatchUseCase, handleDLR handleDLRUseCase) *ProviderHandler {
	return &ProviderHandler{
		dispatch:  dispatch,
		handleDLR: handleDLR,
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

// Dispatch traite POST /dispatch.
//
// Mappage des erreurs metier en codes HTTP :
//   - validation           → 400
//   - ErrProviderNotFound  → 404
//   - ErrProviderUnavailable → 503
//   - ErrSendFailed        → 502
//   - autres               → 500
func (h *ProviderHandler) Dispatch(w http.ResponseWriter, r *http.Request) {
	var req DispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	msg := domain.OutboundMessage{
		Id:         req.MessageID,
		ProviderId: req.ProviderID,
		Channel:    req.Channel,
		Recipient:  req.Recipient,
		Content:    req.Content,
		Metadata:   req.Metadata,
	}

	externalId, err := h.dispatch.Execute(r.Context(), msg)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, DispatchResponse{ExternalID: externalId})
}

// DLR traite POST /dlr (delivery receipt entrant depuis un fournisseur).
//
// Mappage des erreurs metier en codes HTTP :
//   - validation         → 400
//   - statut DLR invalide → 400
//   - autres             → 500
func (h *ProviderHandler) DLR(w http.ResponseWriter, r *http.Request) {
	var req DLRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	status := domain.DeliveryStatus(req.Status)
	if err := h.handleDLR.Execute(r.Context(), req.ExternalID, status); err != nil {
		h.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleError centralise le mappage des erreurs domaine en codes HTTP.
func (h *ProviderHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrProviderNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrProviderUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrSendFailed):
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
	default:
		log.Printf("handler: erreur interne: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "erreur interne"})
	}
}
