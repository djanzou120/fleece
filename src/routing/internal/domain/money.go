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
// Retourne ErrInvalidAmount si amount < 0.
func NewMoney(amount int64, currency string) (Money, error) {
	if amount < 0 {
		return Money{}, fmt.Errorf("money: montant negatif (%d): %w", amount, ErrInvalidAmount)
	}
	return Money{Amount: amount, Currency: currency}, nil
}

// Add additionne deux Money de meme devise.
// Retourne ErrInvalidAmount si les devises different.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("money: devises incompatibles (%s vs %s): %w", m.Currency, other.Currency, ErrInvalidAmount)
	}
	return Money{Amount: m.Amount + other.Amount, Currency: m.Currency}, nil
}

// Sub soustrait other de m. Les deux Money doivent avoir la meme devise.
// Retourne ErrInvalidAmount si les devises different.
func (m Money) Sub(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("money: devises incompatibles (%s vs %s): %w", m.Currency, other.Currency, ErrInvalidAmount)
	}
	return Money{Amount: m.Amount - other.Amount, Currency: m.Currency}, nil
}

// IsNegative retourne true si le montant est strictement negatif.
func (m Money) IsNegative() bool {
	return m.Amount < 0
}
