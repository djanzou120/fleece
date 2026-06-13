package usecases

import (
	"context"
	"fmt"

	"fleece/src/routing/internal/application/ports/output"
	"fleece/src/routing/internal/domain"
)

// UpdateProviderScore met a jour le score de delivrabilite d'un provider.
// Appele apres reception d'un DLR (Delivery Receipt).
//
// Choix de conception : l'appelant fournit directement le score entier (0..100)
// plutot qu'un bool delivered. Ce choix est plus simple et plus flexible :
// le Messaging Service peut calculer un score glissant avant d'appeler ce use case.
type UpdateProviderScore struct {
	Scores output.ProviderScoreRepository
}

// Execute met a jour le score du provider providerID sur le canal channel.
// score doit etre compris entre 0 et 100 (non verifie ici, responsabilite de l'appelant HTTP).
func (uc UpdateProviderScore) Execute(ctx context.Context, providerID, channel string, score int) error {
	s := domain.ProviderScore{
		ProviderID: providerID,
		Channel:    domain.Channel(channel),
		Score:      score,
	}
	if err := uc.Scores.Upsert(ctx, s); err != nil {
		return fmt.Errorf("update_score: upsert provider=%s canal=%s: %w", providerID, channel, err)
	}
	return nil
}
