package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"fleece/src/routing/internal/domain"
)

// PricingRepository implemente output.ProviderPricingRepository via *sql.DB.
// N'accede qu'au schema "routing" (table routing.provider_pricing).
type PricingRepository struct {
	db              *sql.DB
	defaultCurrency string
}

// NewPricingRepository cree un PricingRepository avec la connexion et la devise par defaut.
// defaultCurrency est utilise pour construire le Money de chaque entite domaine,
// car la table ne stocke pas la devise.
func NewPricingRepository(db *sql.DB, defaultCurrency string) *PricingRepository {
	return &PricingRepository{db: db, defaultCurrency: defaultCurrency}
}

// ListByChannelCountry retourne tous les tarifs disponibles pour un canal et un pays.
// Retourne une slice vide (sans erreur) si aucun tarif n'est configure.
func (r *PricingRepository) ListByChannelCountry(ctx context.Context, channel, country string) ([]domain.ProviderPricing, error) {
	const query = `
		SELECT id, provider, channel, country, cost
		  FROM routing.provider_pricing
		 WHERE channel = $1
		   AND country = $2
	`
	rows, err := r.db.QueryContext(ctx, query, channel, country)
	if err != nil {
		return nil, fmt.Errorf("persistence: ListByChannelCountry canal=%s pays=%s: %w", channel, country, err)
	}
	defer rows.Close()

	var result []domain.ProviderPricing
	for rows.Next() {
		var rec pricingRecord
		if err := rows.Scan(&rec.ID, &rec.Provider, &rec.Channel, &rec.Country, &rec.Cost); err != nil {
			return nil, fmt.Errorf("persistence: ListByChannelCountry scan: %w", err)
		}
		result = append(result, rec.toEntity(r.defaultCurrency))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: ListByChannelCountry rows: %w", err)
	}
	return result, nil
}
