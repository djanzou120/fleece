// Package output contient les ports pilotes (driven) du service Contact Intelligence.
// Ce sont des interfaces que les adapters de couche 3 implementent.
// Les use cases dependent uniquement de ces interfaces, jamais des implementations concretes.
package output

import (
	"context"

	"fleece/src/contact-intelligence/internal/domain"
)

// ContactRepository persiste et recupere les contacts.
// Implemente en couche 3 (Postgres, schema "contact_intel").
type ContactRepository interface {
	// Get recupere un contact par son phone.
	// Retourne domain.ErrContactNotFound si le contact n'existe pas.
	Get(ctx context.Context, phone string) (*domain.Contact, error)

	// Upsert cree ou met a jour un contact (tous les champs).
	Upsert(ctx context.Context, c *domain.Contact) error
}

// ChannelScoreRepository persiste les scores par (phone, channel).
// Implemente en couche 3 (Postgres, schema "contact_intel").
type ChannelScoreRepository interface {
	// Get recupere le score d'un canal pour un contact.
	// Retourne nil, nil si aucune donnee n'existe encore (score neutre documenté).
	Get(ctx context.Context, phone string, ch domain.Channel) (*domain.ChannelScore, error)

	// ListByPhone retourne tous les scores d'un contact (tous les canaux).
	ListByPhone(ctx context.Context, phone string) ([]*domain.ChannelScore, error)

	// Upsert cree ou met a jour le score (phone, channel).
	Upsert(ctx context.Context, cs *domain.ChannelScore) error
}

// HistoryRepository persiste l'historique append-only des outcomes de livraison.
// Implemente en couche 3 (Postgres, schema "contact_intel").
type HistoryRepository interface {
	// Append insere un nouveau resultat de livraison dans l'historique.
	Append(ctx context.Context, o *domain.DeliveryOutcome) error
}

// OutcomeStore regroupe les trois repositories dans une transaction atomique.
// L'implementation concrete en couche 3 garantit la coherence transactionnelle
// (INSERT history + UPSERT channel_scores + UPDATE contacts dans une meme sql.Tx).
type OutcomeStore interface {
	// RecordAtomically persiste l'outcome, met a jour le channel_score et les
	// compteurs du contact dans une unique transaction Postgres.
	RecordAtomically(ctx context.Context, o *domain.DeliveryOutcome, cs *domain.ChannelScore, c *domain.Contact) error
}
