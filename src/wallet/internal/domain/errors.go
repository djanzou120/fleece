// Package domain — entites, value objects, erreurs metier (couche 1).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package domain

import "errors"

// Erreurs metier du domaine wallet.
var (
	// ErrInsufficientFunds est retournee quand le solde du wallet est inferieur
	// au montant demande lors d'un debit.
	ErrInsufficientFunds = errors.New("insufficient_funds")

	// ErrWalletNotFound est retournee quand le wallet d'un workspace est introuvable.
	ErrWalletNotFound = errors.New("wallet_not_found")

	// ErrInvalidAmount est retournee quand un montant negatif est utilise ou quand
	// les devises ne correspondent pas lors d'une operation arithmetique.
	ErrInvalidAmount = errors.New("invalid_amount")
)
