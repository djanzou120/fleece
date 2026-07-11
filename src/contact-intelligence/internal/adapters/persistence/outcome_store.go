package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"fleece/src/contact-intelligence/internal/application/ports/output"
	"fleece/src/contact-intelligence/internal/domain"
)

// Assertion de conformite : OutcomeStore doit implementer output.OutcomeStore.
var _ output.OutcomeStore = (*OutcomeStore)(nil)

// OutcomeStore implemente output.OutcomeStore en garantissant la coherence
// transactionnelle : INSERT history + UPSERT channel_scores + UPSERT contacts
// s'executent dans une unique sql.Tx.
//
// VIGILANCE T-011 / D24 : RecordAtomically fait l'upsert contact_channel_scores
// ET la mise a jour des compteurs/agregats de contacts dans la MEME transaction,
// conformement a l'exigence de coherence transactionnelle.
type OutcomeStore struct {
	db *sql.DB
}

// NewOutcomeStore cree un OutcomeStore.
func NewOutcomeStore(db *sql.DB) *OutcomeStore {
	return &OutcomeStore{db: db}
}

// RecordAtomically persiste l'outcome, met a jour le channel_score et les
// compteurs du contact dans une unique transaction Postgres.
//
// Ordre des operations dans la transaction :
//  1. INSERT INTO contact_channel_history (append DLR)
//  2. UPSERT contact_channel_scores (score par canal)
//  3. UPSERT contacts (compteurs agregats + score global)
//
// Si l'une des operations echoue, toute la transaction est rollbackee.
func (s *OutcomeStore) RecordAtomically(ctx context.Context, o *domain.DeliveryOutcome, cs *domain.ChannelScore, c *domain.Contact) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("outcome_store: begin tx: %w", err)
	}
	// Rollback automatique si on n'a pas commite.
	defer func() { _ = tx.Rollback() }()

	// 1. Append a l'historique.
	if err := insertHistory(ctx, tx, o); err != nil {
		return fmt.Errorf("outcome_store: insert history: %w", err)
	}

	// 2. Upsert channel_score (score par canal).
	if err := upsertChannelScore(ctx, tx, cs); err != nil {
		return fmt.Errorf("outcome_store: upsert channel_score: %w", err)
	}

	// 3. Upsert contact (compteurs agregats + score global).
	if err := upsertContact(ctx, tx, c); err != nil {
		return fmt.Errorf("outcome_store: upsert contact: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("outcome_store: commit: %w", err)
	}
	return nil
}

// insertHistory insere un outcome dans contact_channel_history.
func insertHistory(ctx context.Context, tx *sql.Tx, o *domain.DeliveryOutcome) error {
	const query = `
		INSERT INTO contact_intel.contact_channel_history (phone, channel, success, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := tx.ExecContext(ctx, query,
		o.Phone,
		string(o.Channel),
		o.Success,
		o.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("persistence: insertHistory (%s,%s): %w", o.Phone, o.Channel, err)
	}
	return nil
}

// upsertContact met a jour les compteurs agregats et le score global du contact.
func upsertContact(ctx context.Context, tx *sql.Tx, c *domain.Contact) error {
	channels := make([]string, len(c.KnownChannels))
	for i, ch := range c.KnownChannels {
		channels[i] = string(ch)
	}
	knownChannelsLiteral := toTextArrayLiteral(channels)

	var lastSuccessAt interface{}
	if c.LastSuccessAt != nil {
		lastSuccessAt = *c.LastSuccessAt
	}

	const query = `
		INSERT INTO contact_intel.contacts
		       (phone, country, known_channels, delivery_score,
		        last_channel, last_success_at, success_count, failure_count, updated_at)
		VALUES ($1, $2, $3::text[], $4, $5, $6, $7, $8, now())
		ON CONFLICT (phone) DO UPDATE
		  SET country         = EXCLUDED.country,
		      known_channels  = EXCLUDED.known_channels,
		      delivery_score  = EXCLUDED.delivery_score,
		      last_channel    = EXCLUDED.last_channel,
		      last_success_at = EXCLUDED.last_success_at,
		      success_count   = EXCLUDED.success_count,
		      failure_count   = EXCLUDED.failure_count,
		      updated_at      = now()
	`
	_, err := tx.ExecContext(ctx, query,
		c.Phone,
		c.Country,
		knownChannelsLiteral,
		c.DeliveryScore.Value(),
		string(c.LastChannel),
		lastSuccessAt,
		c.SuccessCount,
		c.FailureCount,
	)
	if err != nil {
		return fmt.Errorf("persistence: upsertContact %s: %w", c.Phone, err)
	}
	return nil
}
