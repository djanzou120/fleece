package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"fleece/src/wallet/internal/domain"
)

// debitWalletUseCase est l'interface locale du use case DebitWallet.
type debitWalletUseCase interface {
	Execute(ctx context.Context, workspaceID string, amount int64, messageID string) (*domain.WalletTransaction, error)
}

// creditWalletUseCase est l'interface locale du use case CreditWallet.
type creditWalletUseCase interface {
	Execute(ctx context.Context, workspaceID string, amount int64, currency string) (*domain.WalletTransaction, error)
}

// refundUseCase est l'interface locale du use case Refund.
type refundUseCase interface {
	Execute(ctx context.Context, workspaceID string, amount int64, messageID string) (*domain.WalletTransaction, error)
}

// getBalanceUseCase est l'interface locale du use case GetBalance.
type getBalanceUseCase interface {
	Execute(ctx context.Context, workspaceID string) (*domain.Wallet, error)
}

// WalletHandler regroupe les handlers REST du service wallet.
type WalletHandler struct {
	debit      debitWalletUseCase
	credit     creditWalletUseCase
	refund     refundUseCase
	getBalance getBalanceUseCase
}

// NewWalletHandler cree un WalletHandler avec les use cases injectes.
func NewWalletHandler(
	debit debitWalletUseCase,
	credit creditWalletUseCase,
	refund refundUseCase,
	getBalance getBalanceUseCase,
) *WalletHandler {
	return &WalletHandler{
		debit:      debit,
		credit:     credit,
		refund:     refund,
		getBalance: getBalance,
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

// Debit traite POST /debit.
//
// Mappage des erreurs metier en codes HTTP :
//   - validation          → 400
//   - ErrInvalidAmount    → 400
//   - ErrWalletNotFound   → 404
//   - ErrInsufficientFunds → 402
//   - autres              → 500
func (h *WalletHandler) Debit(w http.ResponseWriter, r *http.Request) {
	var req DebitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	txn, err := h.debit.Execute(r.Context(), req.WorkspaceID, req.Amount, req.MessageID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, TransactionResponse{
		ID:          txn.ID,
		WorkspaceID: txn.WorkspaceID,
		Kind:        string(txn.Kind),
		Amount:      txn.Amount,
		MessageID:   txn.MessageID,
	})
}

// Credit traite POST /credit.
//
// Mappage des erreurs metier en codes HTTP :
//   - validation       → 400
//   - ErrInvalidAmount → 400
//   - autres           → 500
func (h *WalletHandler) Credit(w http.ResponseWriter, r *http.Request) {
	var req CreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	txn, err := h.credit.Execute(r.Context(), req.WorkspaceID, req.Amount, req.Currency)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, TransactionResponse{
		ID:          txn.ID,
		WorkspaceID: txn.WorkspaceID,
		Kind:        string(txn.Kind),
		Amount:      txn.Amount,
	})
}

// RefundHandler traite POST /refund.
//
// Mappage des erreurs metier en codes HTTP :
//   - validation       → 400
//   - ErrInvalidAmount → 400
//   - ErrWalletNotFound → 404
//   - autres           → 500
func (h *WalletHandler) RefundHandler(w http.ResponseWriter, r *http.Request) {
	var req RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corps de requete invalide"})
		return
	}
	if err := req.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	txn, err := h.refund.Execute(r.Context(), req.WorkspaceID, req.Amount, req.MessageID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, TransactionResponse{
		ID:          txn.ID,
		WorkspaceID: txn.WorkspaceID,
		Kind:        string(txn.Kind),
		Amount:      txn.Amount,
		MessageID:   txn.MessageID,
	})
}

// Balance traite GET /balance?workspace_id=<id>.
//
// Mappage des erreurs metier en codes HTTP :
//   - workspace_id absent → 400
//   - ErrWalletNotFound  → 404
//   - autres             → 500
func (h *WalletHandler) Balance(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "workspace_id est requis"})
		return
	}

	wallet, err := h.getBalance.Execute(r.Context(), workspaceID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, BalanceResponse{
		WorkspaceID: wallet.WorkspaceID,
		Balance:     wallet.Balance.Amount,
		Currency:    wallet.Balance.Currency,
	})
}

// handleError centralise le mappage des erreurs domaine en codes HTTP.
func (h *WalletHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeJSON(w, http.StatusPaymentRequired, errorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrWalletNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInvalidAmount):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	default:
		log.Printf("handler: erreur interne: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "erreur interne"})
	}
}
