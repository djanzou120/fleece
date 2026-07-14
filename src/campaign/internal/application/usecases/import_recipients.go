package usecases

import (
	"context"
	"fmt"

	"fleece/src/campaign/internal/application/ports/input"
	"fleece/src/campaign/internal/application/ports/output"
)

// ImportRecipients importe les destinataires d'une campagne avec deduplication.
// La deduplication est enforced par la base (INSERT ... ON CONFLICT DO NOTHING).
// Le use case retourne le nombre de lignes reellement inserees.
type ImportRecipients struct {
	Recipients output.RecipientRepository
}

// Execute importe les destinataires et retourne le nombre insere.
func (uc *ImportRecipients) Execute(ctx context.Context, cmd input.ImportRecipientsCommand) (int64, error) {
	if cmd.CampaignID == "" {
		return 0, fmt.Errorf("import_recipients: campaign_id requis")
	}
	if len(cmd.Recipients) == 0 {
		return 0, nil
	}

	inserted, err := uc.Recipients.BulkInsertDedup(ctx, cmd.CampaignID, cmd.Recipients)
	if err != nil {
		return 0, fmt.Errorf("import_recipients: bulk insert: %w", err)
	}
	return inserted, nil
}
