package usecases

import (
	"context"
	"fmt"

	"fleece/src/contact-intelligence/internal/application/ports/output"
	"fleece/src/contact-intelligence/internal/domain"
)

// RecipientProfile est le profil de joignabilite retourne par EnrichRecipient.
type RecipientProfile struct {
	Phone          string
	Country        string
	KnownChannels  []domain.Channel
	GlobalScore    domain.DeliveryScore
	ChannelScores  []*domain.ChannelScore
	BestChannel    domain.Channel
}

// EnrichRecipient retourne le profil de joignabilite complet d'un destinataire :
// canaux connus, meilleurs scores, canal recommande.
//
// Si le contact est inconnu, retourne un profil vide (non found) sans erreur dure.
type EnrichRecipient struct {
	Contacts      output.ContactRepository
	ChannelScores output.ChannelScoreRepository
}

// Execute retourne le profil de joignabilite de phone.
func (uc EnrichRecipient) Execute(ctx context.Context, phone string) (*RecipientProfile, error) {
	if phone == "" {
		return nil, domain.ErrEmptyPhone
	}

	contact, err := uc.Contacts.Get(ctx, phone)
	if err != nil {
		if isNotFound(err) {
			return &RecipientProfile{
				Phone:         phone,
				KnownChannels: nil,
				ChannelScores: nil,
				GlobalScore:   domain.NewDeliveryScore(50),
			}, nil
		}
		return nil, fmt.Errorf("enrich_recipient: charger contact: %w", err)
	}

	scores, err := uc.ChannelScores.ListByPhone(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("enrich_recipient: charger scores: %w", err)
	}

	best := bestChannel(scores)

	return &RecipientProfile{
		Phone:         phone,
		Country:       contact.Country,
		KnownChannels: contact.KnownChannels,
		GlobalScore:   contact.DeliveryScore,
		ChannelScores: scores,
		BestChannel:   best,
	}, nil
}

// bestChannel retourne le canal ayant le score le plus eleve dans la liste.
// Retourne "" si la liste est vide.
func bestChannel(scores []*domain.ChannelScore) domain.Channel {
	var best domain.Channel
	max := -1
	for _, s := range scores {
		if s.Score.Value() > max {
			max = s.Score.Value()
			best = s.Channel
		}
	}
	return best
}
