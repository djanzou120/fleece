package domain

import "fmt"

// Money est un value object representant un montant monetaire (en centimes)
// associe a une devise. Immuable : toutes les operations retournent une nouvelle valeur.
type Money struct {
	// Amount est le montant en centimes (la plus petite unite de la devise).
	Amount int64
	// Currency est le code ISO 4217 de la devise (ex. "XAF", "EUR").
	Currency string
}

// NewMoney cree un Money en validant que le montant n'est pas negatif.
// Retourne une erreur si amount < 0.
func NewMoney(amount int64, currency string) (Money, error) {
	if amount < 0 {
		return Money{}, fmt.Errorf("money: montant negatif (%d)", amount)
	}
	return Money{Amount: amount, Currency: currency}, nil
}

// IsZero retourne true si le montant est nul.
func (m Money) IsZero() bool {
	return m.Amount == 0
}
