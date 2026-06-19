package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"fleece/src/routing/internal/domain"
)

// ScoreRepository implemente output.ProviderScoreRepository via *sql.DB.
// N'accede qu'au schema "routing" (table routing.provider_scores).
type ScoreRepository struct {
	db *sql.DB
}

// NewScoreRepository cree un ScoreRepository avec la connexion fournie.
func NewScoreRepository(db *sql.DB) *ScoreRepository {
	return &ScoreRepository{db: db}
}

// ListByChannel retourne tous les scores pour un canal donne.
// Retourne une slice vide (sans erreur) si aucun score n'est enregistre.
// Les colonnes avg_latency_ms et country (migration 0013) sont lues.
func (r *ScoreRepository) ListByChannel(ctx context.Context, channel string) ([]domain.ProviderScore, error) {
	if r.db == nil {
		return nil, nil
	}
	const query = `
		SELECT provider, channel, score, avg_latency_ms, country
		  FROM routing.provider_scores
		 WHERE channel = $1
	`
	rows, err := r.db.QueryContext(ctx, query, channel)
	if err != nil {
		return nil, fmt.Errorf("persistence: ListByChannel canal=%s: %w", channel, err)
	}
	defer rows.Close()

	var result []domain.ProviderScore
	for rows.Next() {
		var rec scoreRecord
		if err := rows.Scan(&rec.Provider, &rec.Channel, &rec.Score, &rec.AvgLatencyMs, &rec.Country); err != nil {
			return nil, fmt.Errorf("persistence: ListByChannel scan: %w", err)
		}
		result = append(result, rec.toEntity())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: ListByChannel rows: %w", err)
	}
	return result, nil
}

// Upsert insere ou met a jour le score d'un provider sur un canal.
// Utilise ON CONFLICT (provider, channel) pour eviter les doublons.
// Les colonnes avg_latency_ms et country (migration 0013) sont incluses.
func (r *ScoreRepository) Upsert(ctx context.Context, score domain.ProviderScore) error {
	if r.db == nil {
		return nil
	}
	const query = `
		INSERT INTO routing.provider_scores (provider, channel, score, avg_latency_ms, country)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, channel) DO UPDATE
		  SET score         = EXCLUDED.score,
		      avg_latency_ms = EXCLUDED.avg_latency_ms,
		      country       = EXCLUDED.country
	`
	_, err := r.db.ExecContext(ctx, query,
		score.ProviderID,
		string(score.Channel),
		score.Score,
		score.AvgLatencyMs,
		score.Country,
	)
	if err != nil {
		return fmt.Errorf("persistence: Upsert score provider=%s canal=%s: %w", score.ProviderID, string(score.Channel), err)
	}
	return nil
}
