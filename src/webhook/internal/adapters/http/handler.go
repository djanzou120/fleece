package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"fleece/src/webhook/internal/application/ports/output"
	"fleece/src/webhook/internal/domain"
)

// registerEndpointUseCase est l'interface locale du use case RegisterEndpoint.
type registerEndpointUseCase interface {
	Execute(ctx context.Context, endpoint *domain.WebhookEndpoint) (*domain.WebhookEndpoint, error)
}

// WebhookHandler regroupe les handlers REST du service webhook.
type WebhookHandler struct {
	register  registerEndpointUseCase
	endpoints output.EndpointRepository
}

// NewWebhookHandler cree un WebhookHandler avec les dependances injectees.
func NewWebhookHandler(
	register registerEndpointUseCase,
	endpoints output.EndpointRepository,
) *WebhookHandler {
	return &WebhookHandler{
		register:  register,
		endpoints: endpoints,
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

// Register traite POST /endpoints.
//
// Mappage des erreurs en codes HTTP :
//   - validation (url vide, pas d'events) → 400
//   - autres                              → 500
func (h *WebhookHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	endpoint := &domain.WebhookEndpoint{
		ID:          newUUID(),
		WorkspaceID: req.WorkspaceID,
		URL:         req.URL,
		Secret:      req.Secret,
		Events:      req.Events,
	}

	created, err := h.register.Execute(r.Context(), endpoint)
	if err != nil {
		log.Printf("handler: Register endpoint erreur: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "erreur interne"})
		return
	}

	writeJSON(w, http.StatusCreated, toEndpointResponse(created))
}

// List traite GET /endpoints?workspace_id=<id>.
//
// Mappage des erreurs en codes HTTP :
//   - workspace_id absent → 400
//   - autres              → 500
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "workspace_id est requis"})
		return
	}

	endpoints, err := h.endpoints.ListByWorkspace(r.Context(), workspaceID)
	if err != nil {
		log.Printf("handler: List endpoints workspace=%s erreur: %v", workspaceID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "erreur interne"})
		return
	}

	resp := ListEndpointsResponse{
		Endpoints: make([]EndpointResponse, 0, len(endpoints)),
	}
	for _, ep := range endpoints {
		resp.Endpoints = append(resp.Endpoints, toEndpointResponse(ep))
	}
	writeJSON(w, http.StatusOK, resp)
}

// Delete traite DELETE /endpoints/{id}.
//
// Mappage des erreurs en codes HTTP :
//   - ErrEndpointNotFound → 404
//   - autres              → 500
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "id est requis"})
		return
	}

	if err := h.endpoints.Delete(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrEndpointNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
			return
		}
		log.Printf("handler: Delete endpoint %s erreur: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "erreur interne"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// toEndpointResponse convertit une entite domaine en DTO de sortie.
func toEndpointResponse(ep *domain.WebhookEndpoint) EndpointResponse {
	events := ep.Events
	if events == nil {
		events = []string{}
	}
	return EndpointResponse{
		ID:          ep.ID,
		WorkspaceID: ep.WorkspaceID,
		URL:         ep.URL,
		Secret:      ep.Secret,
		Events:      events,
		Active:      ep.Active,
	}
}
