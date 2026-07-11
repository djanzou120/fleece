package domain

import "time"

// DeliveryOutcome represente un resultat de livraison (DLR) d'un message.
// C'est la commande d'entree du use case RecordDeliveryOutcome.
type DeliveryOutcome struct {
	Phone     string
	Channel   Channel
	Success   bool
	OccurredAt time.Time
}

// NewDeliveryOutcome cree un DeliveryOutcome valide.
// Retourne ErrEmptyPhone ou ErrInvalidChannel si les donnees sont invalides.
func NewDeliveryOutcome(phone string, ch Channel, success bool, occurredAt time.Time) (*DeliveryOutcome, error) {
	if phone == "" {
		return nil, ErrEmptyPhone
	}
	if !ch.IsValid() {
		return nil, ErrInvalidChannel
	}
	t := occurredAt.UTC()
	return &DeliveryOutcome{
		Phone:      phone,
		Channel:    ch,
		Success:    success,
		OccurredAt: t,
	}, nil
}
