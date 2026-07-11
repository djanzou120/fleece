package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"fleece/src/contact-intelligence/internal/application/ports/output"
	"fleece/src/contact-intelligence/internal/domain"
)

// Assertion de conformite : ChannelScoreRepository doit implementer output.ChannelScoreRepository.
var _ output.ChannelScoreRepository = (*ChannelScoreRepository)(nil)

// ChannelScoreRepository implemente output.ChannelScoreRepository via *sql.DB.
// Il n'accede qu'au schema "contact_intel" (table contact_channel_scores).
type ChannelScoreRepository struct {
	db *sql.DB
}

// NewChannelScoreRepository cree un ChannelScoreRepository.
func NewChannelScoreRepository(db *sql.DB) *ChannelScoreRepository {
	return &ChannelScoreRepository{db: db}
}

// Get recupere le score d'un (phone, channel).
// Retourne nil, nil si aucun score n'existe (contact/canal inconnu = score neutre).
func (r *ChannelScoreRepository) Get(ctx context.Context, phone string, ch domain.Channel) (*domain.ChannelScore, error) {
	const query = `
		SELECT phone, channel, score, sample_size, last_success_at, updated_at
		  FROM contact_intel.contact_channel_scores
		 WHERE phone = $1 AND channel = $2
	`
	row := r.db.QueryRowContext(ctx, query, phone, string(ch))
	var rec channelScoreRecord
	err := row.Scan(
		&rec.Phone,
		&rec.Channel,
		&rec.Score,
		&rec.SampleSize,
		&rec.LastSuccessAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // pas d'entree = score neutre a creer
		}
		return nil, fmt.Errorf("persistence: Get channel_score (%s,%s): %w", phone, ch, err)
	}
	return rec.toEntity(), nil
}

// ListByPhone retourne tous les scores de delivrabilite d'un contact (tous les canaux).
func (r *ChannelScoreRepository) ListByPhone(ctx context.Context, phone string) ([]*domain.ChannelScore, error) {
	const query = `
		SELECT phone, channel, score, sample_size, last_success_at, updated_at
		  FROM contact_intel.contact_channel_scores
		 WHERE phone = $1
		 ORDER BY score DESC
	`
	rows, err := r.db.QueryContext(ctx, query, phone)
	if err != nil {
		return nil, fmt.Errorf("persistence: ListByPhone %s: %w", phone, err)
	}
	defer rows.Close()

	var scores []*domain.ChannelScore
	for rows.Next() {
		var rec channelScoreRecord
		if err := rows.Scan(
			&rec.Phone,
			&rec.Channel,
			&rec.Score,
			&rec.SampleSize,
			&rec.LastSuccessAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("persistence: ListByPhone scan: %w", err)
		}
		scores = append(scores, rec.toEntity())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: ListByPhone rows: %w", err)
	}
	return scores, nil
}

// Upsert cree ou met a jour un score par (phone, channel).
// Utilise ON CONFLICT (phone, channel) DO UPDATE.
func (r *ChannelScoreRepository) Upsert(ctx context.Context, cs *domain.ChannelScore) error {
	return upsertChannelScore(ctx, r.db, cs)
}

// upsertChannelScore execute l'upsert sur contact_channel_scores via db (sql.DB ou sql.Tx).
func upsertChannelScore(ctx context.Context, db dbExecer, cs *domain.ChannelScore) error {
	var lastSuccessAt interface{}
	if cs.LastSuccessAt != nil {
		lastSuccessAt = *cs.LastSuccessAt
	}

	const query = `
		INSERT INTO contact_intel.contact_channel_scores
		       (phone, channel, score, sample_size, last_success_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (phone, channel) DO UPDATE
		  SET score          = EXCLUDED.score,
		      sample_size    = EXCLUDED.sample_size,
		      last_success_at = EXCLUDED.last_success_at,
		      updated_at     = now()
	`
	_, err := db.ExecContext(ctx, query,
		cs.Phone,
		string(cs.Channel),
		cs.Score.Value(),
		cs.SampleSize,
		lastSuccessAt,
	)
	if err != nil {
		return fmt.Errorf("persistence: Upsert channel_score (%s,%s): %w", cs.Phone, cs.Channel, err)
	}
	return nil
}

// dbExecer est une interface minimale abstraite sur *sql.DB et *sql.Tx.
type dbExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
