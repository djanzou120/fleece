package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"fleece/src/campaign/internal/application/ports/output"
	"fleece/src/campaign/internal/domain"
)

// Assertion de conformite : CampaignRepository doit implementer output.CampaignRepository.
var _ output.CampaignRepository = (*CampaignRepository)(nil)

// CampaignRepository implemente output.CampaignRepository via *sql.DB.
// Il n'accede qu'au schema "campaign" (search_path configure a la connexion).
type CampaignRepository struct {
	db *sql.DB
}

// NewCampaignRepository cree un CampaignRepository.
func NewCampaignRepository(db *sql.DB) *CampaignRepository {
	return &CampaignRepository{db: db}
}

// Save cree ou met a jour une campagne.
// Utilise ON CONFLICT (id) DO UPDATE pour l'upsert.
func (r *CampaignRepository) Save(ctx context.Context, c *domain.Campaign) error {
	if r.db == nil {
		return nil // dev mode sans base
	}

	var scheduledAt interface{}
	if c.ScheduledAt != nil {
		scheduledAt = *c.ScheduledAt
	}

	const q = `
		INSERT INTO campaign.campaigns
		       (id, workspace_id, name, status, scheduled_at, created_at,
		        message_body, channel_strategy, estimated_cost, currency, rate_limit)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE
		  SET name              = EXCLUDED.name,
		      status            = EXCLUDED.status,
		      scheduled_at      = EXCLUDED.scheduled_at,
		      message_body      = EXCLUDED.message_body,
		      channel_strategy  = EXCLUDED.channel_strategy,
		      estimated_cost    = EXCLUDED.estimated_cost,
		      currency          = EXCLUDED.currency,
		      rate_limit        = EXCLUDED.rate_limit
	`
	currency := c.EstimatedCost.Currency
	if currency == "" {
		currency = "XAF"
	}

	_, err := r.db.ExecContext(ctx, q,
		c.ID,
		c.WorkspaceID,
		c.Name,
		string(c.Status),
		scheduledAt,
		c.CreatedAt,
		c.MessageBody,
		c.ChannelStrategy,
		c.EstimatedCost.Amount,
		currency,
		c.RateLimit,
	)
	if err != nil {
		return fmt.Errorf("persistence: Save campaign %s: %w", c.ID, err)
	}
	return nil
}

// Get recupere une campagne par son ID.
// Retourne domain.ErrCampaignNotFound si absente.
func (r *CampaignRepository) Get(ctx context.Context, id string) (*domain.Campaign, error) {
	if r.db == nil {
		return nil, domain.ErrCampaignNotFound
	}

	const q = `
		SELECT id, workspace_id, name, status, scheduled_at, created_at,
		       message_body, channel_strategy, estimated_cost, currency, rate_limit
		  FROM campaign.campaigns
		 WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, q, id)
	return scanCampaign(row)
}

// ListScheduledDue retourne les campagnes en etat 'scheduled' dont scheduled_at <= now().
// Utilisee par le scheduler pour declencher l'execution.
func (r *CampaignRepository) ListScheduledDue(ctx context.Context) ([]*domain.Campaign, error) {
	if r.db == nil {
		return nil, nil
	}

	const q = `
		SELECT id, workspace_id, name, status, scheduled_at, created_at,
		       message_body, channel_strategy, estimated_cost, currency, rate_limit
		  FROM campaign.campaigns
		 WHERE status = 'scheduled'
		   AND scheduled_at <= now()
	`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: ListScheduledDue: %w", err)
	}
	defer rows.Close()

	var campaigns []*domain.Campaign
	for rows.Next() {
		c, err := scanCampaignRow(rows)
		if err != nil {
			return nil, fmt.Errorf("persistence: ListScheduledDue scan: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: ListScheduledDue rows: %w", err)
	}
	return campaigns, nil
}

// UpdateStatusAtomic met a jour le statut de facon atomique.
// Retourne le nombre de lignes affectees (0 = anti-double-run).
//
// VIGILANCE T-017 (anti-double-run) :
// UPDATE campaign.campaigns SET status=$1 WHERE id=$2 AND status=$3
// Si rowsAffected == 0, la campagne a deja change de statut (double-run detecte).
func (r *CampaignRepository) UpdateStatusAtomic(ctx context.Context, id string, from, to domain.CampaignStatus) (int64, error) {
	if r.db == nil {
		return 1, nil // dev mode
	}

	const q = `
		UPDATE campaign.campaigns
		   SET status = $1
		 WHERE id = $2
		   AND status = $3
	`
	result, err := r.db.ExecContext(ctx, q, string(to), id, string(from))
	if err != nil {
		return 0, fmt.Errorf("persistence: UpdateStatusAtomic campaign %s (%s→%s): %w", id, from, to, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("persistence: UpdateStatusAtomic RowsAffected: %w", err)
	}
	return rows, nil
}

// ─── Scanner helpers ──────────────────────────────────────────────────────────

// campaignScanner est une interface commune a *sql.Row et *sql.Rows.
type campaignScanner interface {
	Scan(dest ...any) error
}

// scanCampaign scan une *sql.Row en *domain.Campaign.
func scanCampaign(row *sql.Row) (*domain.Campaign, error) {
	var (
		id              string
		workspaceID     string
		name            string
		status          string
		scheduledAt     sql.NullTime
		createdAt       time.Time
		messageBody     string
		channelStrategy string
		estimatedCost   int64
		currency        string
		rateLimit       int
	)
	err := row.Scan(
		&id, &workspaceID, &name, &status, &scheduledAt, &createdAt,
		&messageBody, &channelStrategy, &estimatedCost, &currency, &rateLimit,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCampaignNotFound
		}
		return nil, fmt.Errorf("persistence: scan campaign: %w", err)
	}
	return buildCampaign(id, workspaceID, name, status, scheduledAt, createdAt,
		messageBody, channelStrategy, estimatedCost, currency, rateLimit), nil
}

// scanCampaignRow scan une *sql.Rows en *domain.Campaign.
func scanCampaignRow(rows *sql.Rows) (*domain.Campaign, error) {
	var (
		id              string
		workspaceID     string
		name            string
		status          string
		scheduledAt     sql.NullTime
		createdAt       time.Time
		messageBody     string
		channelStrategy string
		estimatedCost   int64
		currency        string
		rateLimit       int
	)
	if err := rows.Scan(
		&id, &workspaceID, &name, &status, &scheduledAt, &createdAt,
		&messageBody, &channelStrategy, &estimatedCost, &currency, &rateLimit,
	); err != nil {
		return nil, err
	}
	return buildCampaign(id, workspaceID, name, status, scheduledAt, createdAt,
		messageBody, channelStrategy, estimatedCost, currency, rateLimit), nil
}

// buildCampaign construit une entite domain.Campaign depuis les valeurs scannees.
func buildCampaign(id, workspaceID, name, status string, scheduledAt sql.NullTime,
	createdAt time.Time, messageBody, channelStrategy string,
	estimatedCost int64, currency string, rateLimit int) *domain.Campaign {

	c := &domain.Campaign{
		ID:              id,
		WorkspaceID:     workspaceID,
		Name:            name,
		Status:          domain.CampaignStatus(status),
		CreatedAt:       createdAt,
		MessageBody:     messageBody,
		ChannelStrategy: channelStrategy,
		RateLimit:       rateLimit,
	}
	if scheduledAt.Valid {
		t := scheduledAt.Time
		c.ScheduledAt = &t
	}
	cost, err := domain.NewMoney(estimatedCost, currency)
	if err == nil {
		c.EstimatedCost = cost
	}
	return c
}
