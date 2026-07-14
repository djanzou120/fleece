package domain

import "fmt"

// Money est un value object representant un montant monetaire en centimes
// associe a une devise. Immuable : toutes les operations retournent une nouvelle valeur.
// Coherent avec le service wallet (D8 — centimes).
type Money struct {
	// Amount est le montant en centimes (la plus petite unite de la devise).
	Amount int64
	// Currency est le code ISO 4217 de la devise (ex. "XAF", "EUR").
	Currency string
}

// NewMoney cree un Money en validant que le montant n'est pas negatif.
// Retourne ErrInvalidAmount si amount < 0.
func NewMoney(amount int64, currency string) (Money, error) {
	if amount < 0 {
		return Money{}, fmt.Errorf("money: montant negatif (%d): %w", amount, ErrInvalidAmount)
	}
	return Money{Amount: amount, Currency: currency}, nil
}

// Multiply multiplie le montant par un facteur entier positif.
// Utilise pour calculer le cout total d'une campagne (cout unitaire x nb destinataires).
// Retourne ErrInvalidAmount si factor < 0.
func (m Money) Multiply(factor int64) (Money, error) {
	if factor < 0 {
		return Money{}, fmt.Errorf("money: facteur negatif (%d): %w", factor, ErrInvalidAmount)
	}
	return Money{Amount: m.Amount * factor, Currency: m.Currency}, nil
}

// IsZero retourne true si le montant est zero.
func (m Money) IsZero() bool {
	return m.Amount == 0
}
