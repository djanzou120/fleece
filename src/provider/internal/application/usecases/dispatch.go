// Package usecases — orchestration des cas d usage (couche 2).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package usecases

import (
	"context"
	"fmt"

	"fleece/src/provider/internal/application/ports/output"
	"fleece/src/provider/internal/domain"
)

// Dispatch orchestre l'envoi d'un message sortant via le fournisseur identifie
// par msg.ProviderId.
//
// La registry est une map[providerId]output.Provider injectee au composition root.
// Ce choix (registry par providerId plutot que par canal) permet de selectionner
// un fournisseur precis quand plusieurs fournisseurs supportent le meme canal.
// Le service appelant (Routing) a deja selectionne le providerId optimal.
type Dispatch struct {
	// Registry mappe chaque providerId a son implementation concrete du port Provider.
	Registry map[string]output.Provider

	// Repo persiste les associations internalId ↔ externalId dans provider_messages.
	Repo output.ProviderRepository

	// Publisher publie les evenements de domaine (provider.sent / provider.failed).
	Publisher output.EventPublisher
}

// Execute selectionne le fournisseur, envoie le message, persiste le mapping et publie.
//
// Flux :
//  1. Lookup fournisseur dans la registry → ErrProviderNotFound si absent.
//  2. Appel provider.Send() → ErrSendFailed (wrappant la cause) si echec.
//  3. Persist provider_messages (internalId, providerId, externalId).
//  4. Publier "provider.sent" (succes) ou "provider.failed" (echec Send).
func (uc *Dispatch) Execute(ctx context.Context, msg domain.OutboundMessage) (string, error) {
	// 1. Selectionner le fournisseur.
	p, ok := uc.Registry[msg.ProviderId]
	if !ok {
		return "", fmt.Errorf("dispatch: %w: %s", domain.ErrProviderNotFound, msg.ProviderId)
	}

	// 2. Envoyer le message via le fournisseur.
	externalId, err := p.Send(ctx, msg)
	if err != nil {
		// Publier l'evenement d'echec (best-effort).
		_ = uc.Publisher.Publish(ctx, "provider.failed", map[string]string{
			"internal_id": msg.Id,
			"provider_id": msg.ProviderId,
			"error":       err.Error(),
		})
		return "", fmt.Errorf("dispatch: %w: %w", domain.ErrSendFailed, err)
	}

	// 3. Persister le mapping internalId ↔ externalId.
	if saveErr := uc.Repo.Save(ctx, msg.Id, msg.ProviderId, externalId); saveErr != nil {
		// Erreur de persistance : on logue (via evenement) mais on ne fait pas echouer
		// l'operation — le message a deja ete envoye.
		_ = uc.Publisher.Publish(ctx, "provider.sent", map[string]string{
			"internal_id": msg.Id,
			"provider_id": msg.ProviderId,
			"external_id": externalId,
			"persist_err": saveErr.Error(),
		})
		return externalId, nil
	}

	// 4. Publier l'evenement de succes (best-effort).
	_ = uc.Publisher.Publish(ctx, "provider.sent", map[string]string{
		"internal_id": msg.Id,
		"provider_id": msg.ProviderId,
		"external_id": externalId,
	})

	return externalId, nil
}
