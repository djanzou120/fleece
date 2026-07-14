package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"fleece/src/campaign/internal/application/ports/output"
	"fleece/src/campaign/internal/domain"
)

// Assertion de conformite : RecipientRepository doit implementer output.RecipientRepository.
var _ output.RecipientRepository = (*RecipientRepository)(nil)

// RecipientRepository implemente output.RecipientRepository via *sql.DB.
// Il n'accede qu'au schema "campaign".
type RecipientRepository struct {
	db *sql.DB
}

// NewRecipientRepository cree un RecipientRepository.
func NewRecipientRepository(db *sql.DB) *RecipientRepository {
	return &RecipientRepository{db: db}
}

// BulkInsertDedup insere les destinataires avec deduplication.
//
// VIGILANCE T-017 (dedup base) :
// INSERT INTO campaign.campaign_recipients (campaign_id, recipient, status)
// VALUES (...) ON CONFLICT (campaign_id, recipient) DO NOTHING
// L'index UNIQUE (campaign_id, recipient) garantit la deduplication en base.
func (r *RecipientRepository) BulkInsertDedup(ctx context.Context, campaignID string, recipients []string) (int64, error) {
	if r.db == nil {
		return int64(len(recipients)), nil // dev mode
	}
	if len(recipients) == 0 {
		return 0, nil
	}

	// Construction du multi-INSERT avec placeholders parametres.
	// On evite les injections SQL en utilisant uniquement des $N.
	var sb strings.Builder
	sb.WriteString("INSERT INTO campaign.campaign_recipients (campaign_id, recipient, status) VALUES ")

	args := make([]any, 0, 1+len(recipients)*2)
	args = append(args, campaignID) // $1

	for i, rec := range recipients {
		if i > 0 {
			sb.WriteString(", ")
		}
		// campaign_id = $1, recipient = $N+1, status = 'pending' (constante)
		fmt.Fprintf(&sb, "($1, $%d, 'pending')", i+2)
		args = append(args, rec)
	}
	sb.WriteString(" ON CONFLICT (campaign_id, recipient) DO NOTHING")

	result, err := r.db.ExecContext(ctx, sb.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("persistence: BulkInsertDedup campaign %s: %w", campaignID, err)
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("persistence: BulkInsertDedup RowsAffected: %w", err)
	}
	return inserted, nil
}

// ListPendingByCampaign retourne les destinataires en etat 'pending'.
//
// VIGILANCE T-017 (reprise idempotente) :
// WHERE campaign_id=$1 AND status='pending'
// Ne retourne jamais les destinataires deja sent/delivered/failed.
func (r *RecipientRepository) ListPendingByCampaign(ctx context.Context, campaignID string) ([]*domain.CampaignRecipient, error) {
	if r.db == nil {
		return nil, nil
	}

	const q = `
		SELECT id, campaign_id, recipient, status, message_id
		  FROM campaign.campaign_recipients
		 WHERE campaign_id = $1
		   AND status = 'pending'
	`
	rows, err := r.db.QueryContext(ctx, q, campaignID)
	if err != nil {
		return nil, fmt.Errorf("persistence: ListPendingByCampaign %s: %w", campaignID, err)
	}
	defer rows.Close()

	var result []*domain.CampaignRecipient
	for rows.Next() {
		var (
			id          int64
			cid         string
			recipient   string
			status      string
			messageID   sql.NullString
		)
		if err := rows.Scan(&id, &cid, &recipient, &status, &messageID); err != nil {
			return nil, fmt.Errorf("persistence: ListPendingByCampaign scan: %w", err)
		}
		r2 := &domain.CampaignRecipient{
			ID:         id,
			CampaignID: cid,
			Recipient:  recipient,
			Status:     domain.RecipientStatus(status),
		}
		if messageID.Valid {
			s := messageID.String
			r2.MessageID = &s
		}
		result = append(result, r2)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: ListPendingByCampaign rows: %w", err)
	}
	return result, nil
}

// UpdateStatusByMessageID met a jour le statut d'un destinataire identifie par message_id.
//
// Correlation DLR :
//   - message_id est la cle de correlation entre un DLR et un destinataire.
//   - Si message_id vide ou inconnu : retourne (0, nil) sans erreur.
//   - rowsAffected = 0 → DLR orphelin (loggue par le use case).
func (r *RecipientRepository) UpdateStatusByMessageID(ctx context.Context, messageID string, status domain.RecipientStatus) (int64, error) {
	if r.db == nil {
		return 0, nil
	}
	if messageID == "" {
		return 0, nil
	}

	const q = `
		UPDATE campaign.campaign_recipients
		   SET status = $1
		 WHERE message_id = $2
	`
	result, err := r.db.ExecContext(ctx, q, string(status), messageID)
	if err != nil {
		return 0, fmt.Errorf("persistence: UpdateStatusByMessageID %s: %w", messageID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("persistence: UpdateStatusByMessageID RowsAffected: %w", err)
	}
	return rows, nil
}

// MarkSent marque un destinataire comme envoye et enregistre son message_id.
func (r *RecipientRepository) MarkSent(ctx context.Context, id int64, messageID string) error {
	if r.db == nil {
		return nil
	}

	var msgID interface{}
	if messageID != "" {
		msgID = messageID
	}

	const q = `
		UPDATE campaign.campaign_recipients
		   SET status = 'sent', message_id = $1
		 WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, q, msgID, id)
	if err != nil {
		return fmt.Errorf("persistence: MarkSent recipient %d: %w", id, err)
	}
	return nil
}

// MarkFailed marque un destinataire comme echoue.
func (r *RecipientRepository) MarkFailed(ctx context.Context, id int64) error {
	if r.db == nil {
		return nil
	}

	const q = `
		UPDATE campaign.campaign_recipients
		   SET status = 'failed'
		 WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("persistence: MarkFailed recipient %d: %w", id, err)
	}
	return nil
}
