package domain

import "fmt"

// Wallet est l'entite racine du domaine Wallet.
// Elle contient le solde courant d'un workspace sous forme de Money.
type Wallet struct {
	// WorkspaceID est l'identifiant du workspace proprietaire de ce wallet.
	WorkspaceID string
	// Balance est le solde courant (en centimes, avec sa devise).
	Balance Money
}

// NewWallet cree un Wallet avec un solde nul pour le workspace et la devise donnes.
func NewWallet(workspaceID, currency string) *Wallet {
	return &Wallet{
		WorkspaceID: workspaceID,
		Balance:     Money{Amount: 0, Currency: currency},
	}
}

// Debit soustrait amount du solde.
// Retourne ErrInsufficientFunds si le solde resultant serait negatif.
// Retourne ErrInvalidAmount si les devises different.
func (w *Wallet) Debit(amount Money) error {
	result, err := w.Balance.Sub(amount)
	if err != nil {
		return err
	}
	if result.IsNegative() {
		return fmt.Errorf("wallet %s: %w", w.WorkspaceID, ErrInsufficientFunds)
	}
	w.Balance = result
	return nil
}

// Credit ajoute amount au solde.
// Retourne ErrInvalidAmount si les devises different.
func (w *Wallet) Credit(amount Money) error {
	result, err := w.Balance.Add(amount)
	if err != nil {
		return err
	}
	w.Balance = result
	return nil
}
