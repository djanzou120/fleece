package usecases

import (
	"context"
	"fmt"
	"time"

	"fleece/src/contact-intelligence/internal/application/ports/output"
	"fleece/src/contact-intelligence/internal/domain"
)

// RecordDeliveryOutcome orchestre la mise a jour de la joignabilite
// sur reception d'un DLR (message.delivered / message.failed).
//
// Flux :
//  1. Valider l'outcome.
//  2. Charger (ou creer) le ChannelScore pour (phone, channel).
//  3. Mettre a jour le ChannelScore via la regle de scoring du domaine.
//  4. Charger (ou creer) le Contact.
//  5. Mettre a jour les compteurs/agregats du Contact.
//  6. Persister atomiquement : INSERT history + UPSERT channel_scores +
//     UPDATE contacts dans une unique transaction Postgres (via OutcomeStore).
//
// Isolation transactionnelle : garantie par OutcomeStore.RecordAtomically.
type RecordDeliveryOutcome struct {
	Contacts      output.ContactRepository
	ChannelScores output.ChannelScoreRepository
	Store         output.OutcomeStore
}

// Execute applique un outcome de livraison et met a jour la joignabilite.
func (uc RecordDeliveryOutcome) Execute(ctx context.Context, phone string, ch domain.Channel, success bool) error {
	if phone == "" {
		return domain.ErrEmptyPhone
	}
	if !ch.IsValid() {
		return domain.ErrInvalidChannel
	}

	now := time.Now().UTC()

	// 1. Construire l'outcome valide (validation domaine).
	o, err := domain.NewDeliveryOutcome(phone, ch, success, now)
	if err != nil {
		return fmt.Errorf("record_delivery_outcome: outcome invalide: %w", err)
	}

	// 2. Charger ou creer le ChannelScore pour (phone, channel).
	cs, err := uc.ChannelScores.Get(ctx, phone, ch)
	if err != nil {
		return fmt.Errorf("record_delivery_outcome: charger channel_score: %w", err)
	}
	if cs == nil {
		// Premier outcome pour ce (phone, channel) : creer un ChannelScore vierge.
		cs, err = domain.NewChannelScore(phone, ch)
		if err != nil {
			return fmt.Errorf("record_delivery_outcome: creer channel_score: %w", err)
		}
	}

	// 3. Mettre a jour le score via la regle domaine.
	cs.RecordOutcome(success, now)

	// 4. Charger ou creer le Contact.
	contact, err := uc.Contacts.Get(ctx, phone)
	if err != nil {
		if !isNotFound(err) {
			return fmt.Errorf("record_delivery_outcome: charger contact: %w", err)
		}
		// Nouveau contact : deduire le pays du prefixe (laisse vide pour l'instant,
		// enrichissement geographique hors scope T-012).
		contact, err = domain.NewContact(phone, "")
		if err != nil {
			return fmt.Errorf("record_delivery_outcome: creer contact: %w", err)
		}
	}

	// 5. Mettre a jour les compteurs/agregats du Contact.
	if err := contact.RecordOutcome(ch, success, now); err != nil {
		return fmt.Errorf("record_delivery_outcome: mettre a jour contact: %w", err)
	}

	// 6. Persister atomiquement : history + channel_scores + contacts.
	if err := uc.Store.RecordAtomically(ctx, o, cs, contact); err != nil {
		return fmt.Errorf("record_delivery_outcome: persister: %w", err)
	}

	return nil
}
