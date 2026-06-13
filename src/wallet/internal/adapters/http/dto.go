// Package http expose les use cases Wallet via une API REST (net/http).
// Couche 3 (Interface Adapters, driving).
package http

import "fmt"

// DebitRequest est le DTO d'entree de l'endpoint POST /debit.
type DebitRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Amount      int64  `json:"amount"`
	MessageID   string `json:"message_id"`
}

// validate verifie que les champs obligatoires sont presents.
func (r *DebitRequest) validate() error {
	if r.WorkspaceID == "" {
		return fmt.Errorf("workspace_id est requis")
	}
	if r.Amount <= 0 {
		return fmt.Errorf("amount doit etre strictement positif")
	}
	if r.MessageID == "" {
		return fmt.Errorf("message_id est requis")
	}
	return nil
}

// CreditRequest est le DTO d'entree de l'endpoint POST /credit.
type CreditRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
}

// validate verifie que les champs obligatoires sont presents.
func (r *CreditRequest) validate() error {
	if r.WorkspaceID == "" {
		return fmt.Errorf("workspace_id est requis")
	}
	if r.Amount <= 0 {
		return fmt.Errorf("amount doit etre strictement positif")
	}
	if r.Currency == "" {
		return fmt.Errorf("currency est requis")
	}
	return nil
}

// RefundRequest est le DTO d'entree de l'endpoint POST /refund.
type RefundRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Amount      int64  `json:"amount"`
	MessageID   string `json:"message_id"`
}

// validate verifie que les champs obligatoires sont presents.
func (r *RefundRequest) validate() error {
	if r.WorkspaceID == "" {
		return fmt.Errorf("workspace_id est requis")
	}
	if r.Amount <= 0 {
		return fmt.Errorf("amount doit etre strictement positif")
	}
	if r.MessageID == "" {
		return fmt.Errorf("message_id est requis")
	}
	return nil
}

// TransactionResponse est le DTO de sortie des operations de mutation (debit/credit/refund).
type TransactionResponse struct {
	ID          int64  `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Kind        string `json:"kind"`
	Amount      int64  `json:"amount"`
	MessageID   string `json:"message_id,omitempty"`
}

// BalanceResponse est le DTO de sortie de l'endpoint GET /balance.
type BalanceResponse struct {
	WorkspaceID string `json:"workspace_id"`
	Balance     int64  `json:"balance"`
	Currency    string `json:"currency"`
}
