package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"fleece/src/campaign/internal/application/ports/output"
	"fleece/src/campaign/internal/domain"
)

// Assertion de conformite : RunRepository doit implementer output.RunRepository.
var _ output.RunRepository = (*RunRepository)(nil)

// RunRepository implemente output.RunRepository via *sql.DB.
// Il n'accede qu'au schema "campaign".
type RunRepository struct {
	db *sql.DB
}

// NewRunRepository cree un RunRepository.
func NewRunRepository(db *sql.DB) *RunRepository {
	return &RunRepository{db: db}
}

// Create cree un nouveau CampaignRun et retourne son ID bigserial.
func (r *RunRepository) Create(ctx context.Context, run *domain.CampaignRun) (int64, error) {
	if r.db == nil {
		return 1, nil // dev mode
	}

	const q = `
		INSERT INTO campaign.campaign_runs
		       (campaign_id, started_at, total_count, sent_count, delivered_count, failed_count)
		VALUES ($1, $2, $3, 0, 0, 0)
		RETURNING id
	`
	var id int64
	err := r.db.QueryRowContext(ctx, q, run.CampaignID, run.StartedAt, run.TotalCount).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("persistence: Create run campaign %s: %w", run.CampaignID, err)
	}
	return id, nil
}

// IncrementSent incremente sent_count d'une unite.
func (r *RunRepository) IncrementSent(ctx context.Context, runID int64) error {
	if r.db == nil {
		return nil
	}
	const q = `UPDATE campaign.campaign_runs SET sent_count = sent_count + 1 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, runID)
	if err != nil {
		return fmt.Errorf("persistence: IncrementSent run %d: %w", runID, err)
	}
	return nil
}

// IncrementDelivered incremente delivered_count d'une unite.
func (r *RunRepository) IncrementDelivered(ctx context.Context, runID int64) error {
	if r.db == nil {
		return nil
	}
	const q = `UPDATE campaign.campaign_runs SET delivered_count = delivered_count + 1 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, runID)
	if err != nil {
		return fmt.Errorf("persistence: IncrementDelivered run %d: %w", runID, err)
	}
	return nil
}

// IncrementFailed incremente failed_count d'une unite.
func (r *RunRepository) IncrementFailed(ctx context.Context, runID int64) error {
	if r.db == nil {
		return nil
	}
	const q = `UPDATE campaign.campaign_runs SET failed_count = failed_count + 1 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, runID)
	if err != nil {
		return fmt.Errorf("persistence: IncrementFailed run %d: %w", runID, err)
	}
	return nil
}

// Finish enregistre finished_at = now() pour le run.
func (r *RunRepository) Finish(ctx context.Context, runID int64) error {
	if r.db == nil {
		return nil
	}
	const q = `UPDATE campaign.campaign_runs SET finished_at = now() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, runID)
	if err != nil {
		return fmt.Errorf("persistence: Finish run %d: %w", runID, err)
	}
	return nil
}
