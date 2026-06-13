package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"fleece/src/messaging/internal/application/usecases"
	"fleece/src/messaging/internal/domain"
)

// sendMessageUseCase est l'interface locale du use case SendMessage.
// Elle permet de ne dépendre que du contrat, pas du type concret.
type sendMessageUseCase interface {
	Execute(ctx context.Context, m *domain.Message) error
}

// MessagingHandler regroupe les handlers REST du service messaging.
type MessagingHandler struct {
	sendMessage sendMessageUseCase
}

// NewMessagingHandler crée un MessagingHandler avec le use case injecté.
func NewMessagingHandler(uc sendMessageUseCase) *MessagingHandler {
	return &MessagingHandler{sendMessage: uc}
}

// errorResponse est le format JSON des erreurs retournées par l'API.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON sérialise v en JSON et l'écrit dans la réponse avec le code HTTP donné.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("handler: writeJSON encode error: %v", err)
	}
}

// SendMessage traite POST /messages.
//
// Mappage des erreurs métier en codes HTTP :
//   - validation → 400
//   - ErrInsufficientFunds → 402
//   - domain.ErrNoChannel → 422
//   - autres → 500
func (h *MessagingHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corps de requête invalide"})
		return
	}

	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	msg, err := req.toMessage()
	if err != nil {
		log.Printf("handler: toMessage: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "erreur interne"})
		return
	}

	if err := h.sendMessage.Execute(r.Context(), msg); err != nil {
		switch {
		case errors.Is(err, usecases.ErrInsufficientFunds):
			writeJSON(w, http.StatusPaymentRequired, errorResponse{Error: err.Error()})
		case errors.Is(err, domain.ErrNoChannel):
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: err.Error()})
		default:
			log.Printf("handler: SendMessage: %v", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "erreur interne"})
		}
		return
	}

	writeJSON(w, http.StatusAccepted, SendMessageResponse{
		ID:     msg.ID,
		Status: string(msg.Status),
	})
}
