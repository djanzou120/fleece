package usecases

import (
	"context"
	"fmt"
	"time"

	"fleece/src/webhook/internal/application/ports/output"
	"fleece/src/webhook/internal/domain"
)

// maxAttempts est le nombre maximum de tentatives avant de passer en Exhausted.
const maxAttempts = 5

// RetryDelivery orchestre la retentative d'une livraison webhook en echec.
//
// Flux :
//  1. Charge la delivery
//  2. Charge l'endpoint cible
//  3. Verifie que le max de tentatives n'est pas atteint
//  4. Signe et tente le dispatch
//  5. Incremente attempts, recalcule nextRetryAt
//  6. Passe en Exhausted apres maxAttempts echecs
type RetryDelivery struct {
	Deliveries output.DeliveryRepository
	Endpoints  output.EndpointRepository
	Dispatcher output.HTTPDispatcher
}

// Execute retente la livraison identifiee par deliveryID.
// Retourne domain.ErrMaxRetriesExceeded si le max est atteint.
func (uc *RetryDelivery) Execute(ctx context.Context, deliveryID int64) error {
	// 1. Charger la delivery.
	delivery, err := uc.Deliveries.FindByID(ctx, deliveryID)
	if err != nil {
		return fmt.Errorf("retry_delivery: charger delivery %d: %w", deliveryID, err)
	}

	// 2. Verifier qu'elle est en echec.
	if delivery.Status == domain.StatusDelivered {
		return nil // Deja livree, rien a faire.
	}
	if delivery.Status == domain.StatusExhausted {
		return fmt.Errorf("retry_delivery: delivery %d: %w", deliveryID, domain.ErrMaxRetriesExceeded)
	}

	// 3. Charger l'endpoint.
	endpoint, err := uc.Endpoints.FindByID(ctx, delivery.EndpointID)
	if err != nil {
		return fmt.Errorf("retry_delivery: charger endpoint %s: %w", delivery.EndpointID, err)
	}

	// 4. Verifier le nombre de tentatives restantes.
	if delivery.Attempts >= maxAttempts {
		delivery.Status = domain.StatusExhausted
		_ = uc.Deliveries.Save(ctx, delivery)
		return fmt.Errorf("retry_delivery: delivery %d: %w", deliveryID, domain.ErrMaxRetriesExceeded)
	}

	// TODO(schema): payload non persiste (colonne absente de 0006) — RetryDelivery ne peut
	// pas redispatcher un body fiable tant que webhook_deliveries ne stocke pas le payload +
	// next_retry_at. Necessite une migration (db-engineer) avant le cablage d'un scheduler
	// de retry.
	if len(delivery.Payload) == 0 {
		return fmt.Errorf("retry_delivery: delivery %d: %w", deliveryID, domain.ErrEmptyPayload)
	}

	// 5. Signer et tenter le dispatch.
	signature := domain.Sign(endpoint.Secret, delivery.Payload)
	statusCode, dispatchErr := uc.Dispatcher.Dispatch(ctx, endpoint.URL, delivery.Payload, signature)

	// 6. Incrementer attempts.
	delivery.Attempts++

	if dispatchErr == nil && isSuccess(statusCode) {
		delivery.Status = domain.StatusDelivered
		delivery.NextRetryAt = time.Time{} // Pas de prochain retry.
	} else {
		// Verifier si on a atteint le max apres cet echec.
		if delivery.Attempts >= maxAttempts {
			delivery.Status = domain.StatusExhausted
		} else {
			delivery.Status = domain.StatusFailed
			delivery.NextRetryAt = time.Now().Add(backoffFor(delivery.Attempts))
		}
	}

	// 7. Persister la delivery mise a jour.
	if err := uc.Deliveries.Save(ctx, delivery); err != nil {
		return fmt.Errorf("retry_delivery: persister delivery %d: %w", deliveryID, err)
	}

	// Retourner ErrMaxRetriesExceeded si epuise.
	if delivery.Status == domain.StatusExhausted {
		return fmt.Errorf("retry_delivery: delivery %d: %w", deliveryID, domain.ErrMaxRetriesExceeded)
	}

	return nil
}

// backoffFor retourne la duree d'attente avant la prochaine tentative selon le
// schema de backoff exponentiel :
//
//	tentative 1 → 1 min
//	tentative 2 → 5 min
//	tentative 3 → 30 min
//	tentative 4 → 2 h
//	tentative 5 → 8 h
//
// Pour toute tentative > 5, retourne 8 h (valeur plafond).
// Cette fonction est exportee pour etre testable independamment.
func backoffFor(attempts int) time.Duration {
	switch attempts {
	case 1:
		return 1 * time.Minute
	case 2:
		return 5 * time.Minute
	case 3:
		return 30 * time.Minute
	case 4:
		return 2 * time.Hour
	default:
		// Tentative 5 et au-dela : plafond a 8h.
		return 8 * time.Hour
	}
}
