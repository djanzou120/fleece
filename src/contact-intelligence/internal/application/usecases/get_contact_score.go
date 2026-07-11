package usecases

import (
	"context"
	"fmt"

	"fleece/src/contact-intelligence/internal/application/ports/output"
	"fleece/src/contact-intelligence/internal/domain"
)

// ContactScoreResult est le resultat du use case GetContactScore.
// Il contient les scores par canal ainsi que le contact si disponible.
type ContactScoreResult struct {
	// Phone est le numero de telephone.
	Phone string
	// ChannelScores contient les scores par canal (vide si aucune donnee).
	ChannelScores []*domain.ChannelScore
	// DeliveryScore est le score global du contact (50 = prior neutre si inconnu).
	DeliveryScore domain.DeliveryScore
	// Found indique si le contact est connu de la base.
	Found bool
}

// GetContactScore retourne les scores de delivrabilite d'un destinataire par canal.
//
// Comportement si contact inconnu : retourne un ContactScoreResult avec Found=false,
// ChannelScores=nil et DeliveryScore=50 (prior neutre). Pas d'erreur dure.
// Ce comportement est documente et consomme par le Routing Service (T-016).
type GetContactScore struct {
	Contacts      output.ContactRepository
	ChannelScores output.ChannelScoreRepository
}

// Execute retourne le profil de scoring du contact identifie par phone.
func (uc GetContactScore) Execute(ctx context.Context, phone string) (*ContactScoreResult, error) {
	if phone == "" {
		return nil, domain.ErrEmptyPhone
	}

	contact, err := uc.Contacts.Get(ctx, phone)
	if err != nil {
		// Contact inconnu : retourner un score neutre, pas d'erreur dure.
		if isNotFound(err) {
			return &ContactScoreResult{
				Phone:         phone,
				ChannelScores: nil,
				DeliveryScore: domain.NewDeliveryScore(50),
				Found:         false,
			}, nil
		}
		return nil, fmt.Errorf("get_contact_score: charger contact: %w", err)
	}

	scores, err := uc.ChannelScores.ListByPhone(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("get_contact_score: charger scores: %w", err)
	}

	return &ContactScoreResult{
		Phone:         phone,
		ChannelScores: scores,
		DeliveryScore: contact.DeliveryScore,
		Found:         true,
	}, nil
}

// isNotFound retourne true si l'erreur est de type "not found".
func isNotFound(err error) bool {
	return err != nil && (err == domain.ErrContactNotFound ||
		containsErr(err, domain.ErrContactNotFound))
}

// containsErr verifie si target est dans la chaine d'erreurs.
func containsErr(err, target error) bool {
	if err == nil {
		return false
	}
	if err == target {
		return true
	}
	// unwrap
	type unwrapper interface{ Unwrap() error }
	if uw, ok := err.(unwrapper); ok {
		return containsErr(uw.Unwrap(), target)
	}
	return false
}
