// Package usecases implemente les cas d'usage du service webhook (couche 2).
//
// Chaque use case depend uniquement des ports (interfaces) de application/ports.
// Aucune dependance vers adapters ou infrastructure.
package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"fleece/src/webhook/internal/application/ports/output"
	"fleece/src/webhook/internal/domain"
)

// RegisterEndpoint orchestre l'enregistrement d'un nouvel endpoint webhook.
// Il ne depend que des ports (interfaces) : aucune dependance vers un framework.
type RegisterEndpoint struct {
	Endpoints output.EndpointRepository
}

// Execute valide l'endpoint, genere un secret si absent, et le persiste.
//
// Validations :
//   - URL non vide
//   - Au moins un evenement abonne
//
// Si Secret est vide, un secret HMAC de 32 octets est genere via crypto/rand.
func (uc *RegisterEndpoint) Execute(ctx context.Context, endpoint *domain.WebhookEndpoint) (*domain.WebhookEndpoint, error) {
	// 1. Valider l'URL.
	if endpoint.URL == "" {
		return nil, fmt.Errorf("register_endpoint: url est requise")
	}

	// 2. Valider qu'il y a au moins un evenement.
	if len(endpoint.Events) == 0 {
		return nil, fmt.Errorf("register_endpoint: au moins un evenement est requis")
	}

	// 3. Generer un secret aleatoire si absent.
	if endpoint.Secret == "" {
		secret, err := generateSecret()
		if err != nil {
			return nil, fmt.Errorf("register_endpoint: generer secret: %w", err)
		}
		endpoint.Secret = secret
	}

	// 4. Activer l'endpoint a la creation.
	endpoint.Active = true

	// 5. Persister l'endpoint.
	if err := uc.Endpoints.Save(ctx, endpoint); err != nil {
		return nil, fmt.Errorf("register_endpoint: persister: %w", err)
	}

	return endpoint, nil
}

// generateSecret genere un secret hexadecimal de 32 octets (256 bits) via crypto/rand.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}
