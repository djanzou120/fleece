package domain

import "time"

// Contact est l'entite racine representant la joignabilite d'un destinataire.
// phone est la cle primaire (numero E.164).
type Contact struct {
	Phone         string
	Country       string
	KnownChannels []Channel
	DeliveryScore DeliveryScore
	LastChannel   Channel
	LastSuccessAt *time.Time
	SuccessCount  int
	FailureCount  int
	UpdatedAt     time.Time
}

// NewContact cree un Contact avec les valeurs initiales.
// Retourne ErrEmptyPhone si phone est vide.
func NewContact(phone, country string) (*Contact, error) {
	if phone == "" {
		return nil, ErrEmptyPhone
	}
	return &Contact{
		Phone:         phone,
		Country:       country,
		KnownChannels: []Channel{},
		DeliveryScore: NewDeliveryScore(50), // prior neutre
		UpdatedAt:     time.Now().UTC(),
	}, nil
}

// AddKnownChannel ajoute channel a la liste des canaux connus si absent.
// Retourne ErrInvalidChannel si le canal est inconnu.
func (c *Contact) AddKnownChannel(ch Channel) error {
	if !ch.IsValid() {
		return ErrInvalidChannel
	}
	for _, existing := range c.KnownChannels {
		if existing == ch {
			return nil // deja present
		}
	}
	c.KnownChannels = append(c.KnownChannels, ch)
	return nil
}

// RecordOutcome met a jour les compteurs agregats du contact et recalcule
// le score global. Il ajoute aussi le canal aux canaux connus si succes.
// Retourne ErrInvalidChannel si ch est inconnu.
func (c *Contact) RecordOutcome(ch Channel, success bool, at time.Time) error {
	if !ch.IsValid() {
		return ErrInvalidChannel
	}
	if success {
		c.SuccessCount++
		c.LastChannel = ch
		t := at.UTC()
		c.LastSuccessAt = &t
		_ = c.AddKnownChannel(ch)
	} else {
		c.FailureCount++
	}
	c.DeliveryScore = ComputeScore(c.SuccessCount, c.FailureCount)
	c.UpdatedAt = at.UTC()
	return nil
}
