package domain

import "errors"

// Erreurs metier du domaine Campaign.
var (
	// ErrInvalidTransition est retournee quand une transition d'etat est impossible
	// depuis l'etat courant de la campagne.
	ErrInvalidTransition = errors.New("campaign: transition d'etat invalide")

	// ErrMissingMessageBody est retournee quand on tente de planifier une campagne
	// sans corps de message (message_body vide).
	ErrMissingMessageBody = errors.New("campaign: le corps du message est requis pour planifier")

	// ErrMissingSchedule est retournee quand on tente de planifier une campagne
	// sans date d'envoi (scheduled_at nil).
	ErrMissingSchedule = errors.New("campaign: la date d'envoi est requise pour planifier")

	// ErrCampaignNotFound est retournee quand une campagne est introuvable.
	ErrCampaignNotFound = errors.New("campaign: campagne introuvable")

	// ErrInvalidAmount est retournee quand un montant monetaire est invalide.
	ErrInvalidAmount = errors.New("campaign: montant monetaire invalide (negatif)")
)
