package usecases

import (
	"context"
	"fmt"

	"fleece/src/provider/internal/application/ports/output"
	"fleece/src/provider/internal/domain"
)

// HandleDLR traite un delivery receipt (DLR) entrant.
//
// TODO(persistence): le schema 0005 ne contient pas de colonne status dans
// provider.provider_messages. Le statut n'est donc pas persiste en base.
// Une migration future devra ajouter cette colonne. En attendant, le statut
// est uniquement communique via l'evenement publie.
type HandleDLR struct {
	// Publisher publie les evenements provider.delivered / provider.failed.
	Publisher output.EventPublisher
}

// Execute valide la transition de statut et publie l'evenement correspondant.
//
// Transitions autorisees depuis le DLR entrant :
//   - Delivered → publie "provider.delivered"
//   - Failed / Rejected → publie "provider.failed"
func (uc *HandleDLR) Execute(ctx context.Context, externalId string, status domain.DeliveryStatus) error {
	if externalId == "" {
		return fmt.Errorf("handle_dlr: externalId est requis")
	}

	// Valider que le statut recu est un etat final attendu d'un DLR.
	// Un DLR ne peut exprimer que Delivered, Failed ou Rejected.
	switch status {
	case domain.StatusDelivered:
		_ = uc.Publisher.Publish(ctx, "provider.delivered", map[string]string{
			"external_id": externalId,
			"status":      string(status),
		})
	case domain.StatusFailed, domain.StatusRejected:
		_ = uc.Publisher.Publish(ctx, "provider.failed", map[string]string{
			"external_id": externalId,
			"status":      string(status),
		})
	default:
		// Statut inattendu dans un DLR.
		return fmt.Errorf("handle_dlr: statut DLR invalide: %s", status)
	}

	return nil
}
