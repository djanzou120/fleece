package usecases

import (
	"context"
	"fmt"
	"time"

	"fleece/src/provider/internal/application/ports/output"
	"fleece/src/provider/internal/domain"
)

// HandleDLR traite un delivery receipt (DLR) entrant.
// Il publie l'evenement de domaine correspondant et persiste le statut en base.
type HandleDLR struct {
	// Publisher publie les evenements provider.delivered / provider.failed.
	Publisher output.EventPublisher

	// Repo persiste le statut de livraison dans provider.provider_messages.
	// Identifie la ligne par son external_id (identifiant fournisseur).
	Repo output.ProviderRepository
}

// Execute valide la transition de statut, publie l'evenement correspondant
// et persiste le statut DLR dans provider.provider_messages.
//
// Mapping statut DLR -> string persiste :
//   - domain.StatusDelivered → "delivered"  (publie "provider.delivered")
//   - domain.StatusFailed    → "failed"     (publie "provider.failed")
//   - domain.StatusRejected  → "rejected"   (publie "provider.failed")
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

	// Persister le statut DLR en base (best-effort : une erreur de persistance
	// ne doit pas invalider le traitement du DLR deja publie).
	if uc.Repo != nil {
		if err := uc.Repo.UpdateStatus(ctx, externalId, string(status), time.Now()); err != nil {
			// Logue via evenement pour ne pas perdre l'information sans bloquer.
			_ = uc.Publisher.Publish(ctx, "provider.dlr_persist_error", map[string]string{
				"external_id": externalId,
				"status":      string(status),
				"error":       err.Error(),
			})
		}
	}

	return nil
}
