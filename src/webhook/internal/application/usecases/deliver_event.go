package usecases

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"fleece/src/webhook/internal/application/ports/output"
	"fleece/src/webhook/internal/domain"
)

// DeliverEvent orchestre la livraison d'un evenement vers tous les endpoints actifs
// du workspace abonnes a cet evenement.
//
// Pour chaque endpoint eligible :
//  1. Signe le payload via domain.Sign(secret, payload)
//  2. Appelle HTTPDispatcher.Dispatch
//  3. Cree et persiste une WebhookDelivery (Delivered si 2xx, Failed sinon)
type DeliverEvent struct {
	Endpoints  output.EndpointRepository
	Deliveries output.DeliveryRepository
	Dispatcher output.HTTPDispatcher
}

// Execute livre l'evenement au workspace donne.
// N'echoue pas si aucun endpoint n'est abonne (operation idempotente).
func (uc *DeliverEvent) Execute(ctx context.Context, workspaceID, event string, payload []byte) error {
	// 1. Charger les endpoints actifs abonnes a cet evenement.
	endpoints, err := uc.Endpoints.FindByWorkspaceAndEvent(ctx, workspaceID, event)
	if err != nil {
		return fmt.Errorf("deliver_event: charger endpoints: %w", err)
	}

	// Aucun endpoint abonne : rien a faire.
	if len(endpoints) == 0 {
		return nil
	}

	// 2. Livrer l'evenement a chaque endpoint.
	for _, ep := range endpoints {
		// Ignorer les endpoints inactifs (securite supplementaire).
		if !ep.Active {
			continue
		}
		if err := uc.dispatchToEndpoint(ctx, ep, event, payload); err != nil {
			// Les erreurs de livraison sont loguees mais n'arretent pas la boucle :
			// on tente de livrer a tous les endpoints.
			log.Printf("deliver_event: endpoint=%s event=%s erreur: %v", ep.ID, event, err)
		}
	}

	return nil
}

// dispatchToEndpoint signe le payload et tente la livraison vers un endpoint.
// Cree et persiste une WebhookDelivery quelle que soit l'issue.
func (uc *DeliverEvent) dispatchToEndpoint(ctx context.Context, ep *domain.WebhookEndpoint, event string, payload []byte) error {
	signature := domain.Sign(ep.Secret, payload)

	statusCode, dispatchErr := uc.Dispatcher.Dispatch(ctx, ep.URL, payload, signature)

	delivery := &domain.WebhookDelivery{
		EndpointID: ep.ID,
		Event:      event,
		Payload:    payload,
		Attempts:   1,
	}

	if dispatchErr == nil && isSuccess(statusCode) {
		delivery.Status = domain.StatusDelivered
	} else {
		delivery.Status = domain.StatusFailed
		// Programmer le premier retry avec le backoff.
		delivery.NextRetryAt = time.Now().Add(backoffFor(1))
	}

	if err := uc.Deliveries.Save(ctx, delivery); err != nil {
		return fmt.Errorf("persister delivery endpoint=%s: %w", ep.ID, err)
	}

	return nil
}

// isSuccess retourne true si le code HTTP indique un succes (2xx).
func isSuccess(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}
