package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"fleece/src/contact-intelligence/internal/application/ports/output"
	"fleece/src/contact-intelligence/internal/domain"
)

// Assertion de conformite : ContactRepository doit implementer output.ContactRepository.
var _ output.ContactRepository = (*ContactRepository)(nil)

// ContactRepository implemente output.ContactRepository via *sql.DB.
// Il n'accede qu'au schema "contact_intel".
type ContactRepository struct {
	db *sql.DB
}

// NewContactRepository cree un ContactRepository avec la connexion fournie.
func NewContactRepository(db *sql.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

// Get recupere un contact par son phone.
// Retourne domain.ErrContactNotFound si le contact n'existe pas.
func (r *ContactRepository) Get(ctx context.Context, phone string) (*domain.Contact, error) {
	const query = `
		SELECT phone, country, known_channels, delivery_score,
		       last_channel, last_success_at, success_count, failure_count, updated_at
		  FROM contact_intel.contacts
		 WHERE phone = $1
	`
	row := r.db.QueryRowContext(ctx, query, phone)
	var rec contactRecord
	var knownChannelsStr string // postgres retourne '{sms,whatsapp}' en texte sans driver array
	err := row.Scan(
		&rec.Phone,
		&rec.Country,
		&knownChannelsStr,
		&rec.DeliveryScore,
		&rec.LastChannel,
		&rec.LastSuccessAt,
		&rec.SuccessCount,
		&rec.FailureCount,
		&rec.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("persistence: Get contact %s: %w", phone, domain.ErrContactNotFound)
		}
		return nil, fmt.Errorf("persistence: Get contact %s: %w", phone, err)
	}
	rec.KnownChannels = parseTextArray(knownChannelsStr)
	return rec.toEntity(), nil
}

// Upsert cree ou met a jour un contact.
func (r *ContactRepository) Upsert(ctx context.Context, c *domain.Contact) error {
	channels := make([]string, len(c.KnownChannels))
	for i, ch := range c.KnownChannels {
		channels[i] = string(ch)
	}
	knownChannelsLiteral := toTextArrayLiteral(channels)

	var lastSuccessAt interface{}
	if c.LastSuccessAt != nil {
		lastSuccessAt = *c.LastSuccessAt
	}

	const query = `
		INSERT INTO contact_intel.contacts
		       (phone, country, known_channels, delivery_score,
		        last_channel, last_success_at, success_count, failure_count, updated_at)
		VALUES ($1, $2, $3::text[], $4, $5, $6, $7, $8, now())
		ON CONFLICT (phone) DO UPDATE
		  SET country        = EXCLUDED.country,
		      known_channels = EXCLUDED.known_channels,
		      delivery_score = EXCLUDED.delivery_score,
		      last_channel   = EXCLUDED.last_channel,
		      last_success_at = EXCLUDED.last_success_at,
		      success_count  = EXCLUDED.success_count,
		      failure_count  = EXCLUDED.failure_count,
		      updated_at     = now()
	`
	_, err := r.db.ExecContext(ctx, query,
		c.Phone,
		c.Country,
		knownChannelsLiteral,
		c.DeliveryScore.Value(),
		string(c.LastChannel),
		lastSuccessAt,
		c.SuccessCount,
		c.FailureCount,
	)
	if err != nil {
		return fmt.Errorf("persistence: Upsert contact %s: %w", c.Phone, err)
	}
	return nil
}

// parseTextArray parse la representation PostgreSQL d'un tableau TEXT : '{a,b,c}'.
// Fallback manuel pour eviter l'import de lib/pq pour les arrays.
func parseTextArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return []string{}
	}
	// Supprimer les accolades.
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// toTextArrayLiteral convertit une slice Go en literal PostgreSQL '{a,b,c}'.
func toTextArrayLiteral(ss []string) string {
	if len(ss) == 0 {
		return "{}"
	}
	return "{" + strings.Join(ss, ",") + "}"
}
