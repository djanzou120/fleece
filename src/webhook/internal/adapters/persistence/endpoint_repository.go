package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"fleece/src/webhook/internal/application/ports/output"
	"fleece/src/webhook/internal/domain"
)

// Assertion de conformite : EndpointRepository doit implementer output.EndpointRepository.
// Le compilateur echoue ici si une methode est manquante ou mal typee.
var _ output.EndpointRepository = (*EndpointRepository)(nil)

// EndpointRepository implemente output.EndpointRepository via *sql.DB.
// Il n'accede qu'au schema "webhook" (tables prefixees webhook.).
type EndpointRepository struct {
	db *sql.DB
}

// NewEndpointRepository cree un EndpointRepository avec la connexion fournie.
func NewEndpointRepository(db *sql.DB) *EndpointRepository {
	return &EndpointRepository{db: db}
}

// Save persiste (insert ou update) un endpoint dans webhook.webhook_endpoints.
// ON CONFLICT (id) DO UPDATE met a jour les champs modifiables.
func (r *EndpointRepository) Save(ctx context.Context, e *domain.WebhookEndpoint) error {
	rec := fromEndpoint(e)

	// Encoder le tableau d'evenements en literal Postgres : {"event1","event2"}
	eventsLiteral := arrayLiteral(rec.Events)

	const query = `
		INSERT INTO webhook.webhook_endpoints (id, workspace_id, url, secret, events)
		VALUES ($1, $2, $3, $4, $5::text[])
		ON CONFLICT (id) DO UPDATE
		  SET url    = EXCLUDED.url,
		      secret = EXCLUDED.secret,
		      events = EXCLUDED.events
	`
	_, err := r.db.ExecContext(ctx, query,
		rec.ID,
		rec.WorkspaceID,
		rec.URL,
		rec.Secret,
		eventsLiteral,
	)
	if err != nil {
		return fmt.Errorf("persistence: Save endpoint %s: %w", e.ID, err)
	}
	return nil
}

// FindByID recupere un endpoint par son identifiant.
// Retourne domain.ErrEndpointNotFound si l'endpoint est introuvable.
func (r *EndpointRepository) FindByID(ctx context.Context, id string) (*domain.WebhookEndpoint, error) {
	const query = `
		SELECT id, workspace_id, url, secret, events, created_at
		  FROM webhook.webhook_endpoints
		 WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	rec, err := scanEndpoint(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("persistence: FindByID endpoint %s: %w", id, domain.ErrEndpointNotFound)
		}
		return nil, fmt.Errorf("persistence: FindByID endpoint %s: %w", id, err)
	}
	return rec.toEntity(), nil
}

// FindByWorkspaceAndEvent retourne les endpoints actifs d'un workspace abonnes a l'evenement.
// Comme la colonne "active" est absente du schema, tous les endpoints sont consideres actifs.
func (r *EndpointRepository) FindByWorkspaceAndEvent(ctx context.Context, workspaceID, event string) ([]*domain.WebhookEndpoint, error) {
	// Filtre sur le tableau events via l'operateur @> (contains) de Postgres.
	const query = `
		SELECT id, workspace_id, url, secret, events, created_at
		  FROM webhook.webhook_endpoints
		 WHERE workspace_id = $1
		   AND events @> ARRAY[$2]::text[]
	`
	rows, err := r.db.QueryContext(ctx, query, workspaceID, event)
	if err != nil {
		return nil, fmt.Errorf("persistence: FindByWorkspaceAndEvent workspace=%s event=%s: %w", workspaceID, event, err)
	}
	defer rows.Close()

	return scanEndpoints(rows)
}

// ListByWorkspace retourne tous les endpoints d'un workspace.
func (r *EndpointRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.WebhookEndpoint, error) {
	const query = `
		SELECT id, workspace_id, url, secret, events, created_at
		  FROM webhook.webhook_endpoints
		 WHERE workspace_id = $1
		 ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("persistence: ListByWorkspace workspace=%s: %w", workspaceID, err)
	}
	defer rows.Close()

	return scanEndpoints(rows)
}

// Delete supprime un endpoint par son identifiant.
func (r *EndpointRepository) Delete(ctx context.Context, id string) error {
	const query = `DELETE FROM webhook.webhook_endpoints WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("persistence: Delete endpoint %s: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("persistence: Delete endpoint %s: %w", id, domain.ErrEndpointNotFound)
	}
	return nil
}

// --- helpers ---

// rowScanner est satisfait par *sql.Row et *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanEndpoint scanne une ligne de webhook_endpoints.
// Le champ events text[] est decode depuis la representation Postgres.
func scanEndpoint(row rowScanner) (endpointRecord, error) {
	var rec endpointRecord
	var eventsStr string // ex: "{event1,event2}"
	err := row.Scan(
		&rec.ID,
		&rec.WorkspaceID,
		&rec.URL,
		&rec.Secret,
		&eventsStr,
		&rec.CreatedAt,
	)
	if err != nil {
		return endpointRecord{}, err
	}
	rec.Events = parseArrayLiteral(eventsStr)
	return rec, nil
}

// scanEndpoints boucle sur des *sql.Rows et retourne les entites.
func scanEndpoints(rows *sql.Rows) ([]*domain.WebhookEndpoint, error) {
	var eps []*domain.WebhookEndpoint
	for rows.Next() {
		rec, err := scanEndpoint(rows)
		if err != nil {
			return nil, fmt.Errorf("persistence: scan endpoint: %w", err)
		}
		eps = append(eps, rec.toEntity())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: rows error: %w", err)
	}
	return eps, nil
}

// arrayLiteral encode une slice en literal tableau Postgres : {"a","b","c"}
// Pour une slice vide, retourne "{}".
func arrayLiteral(values []string) string {
	if len(values) == 0 {
		return "{}"
	}
	quoted := make([]string, len(values))
	for i, v := range values {
		// Echapper les guillemets internes.
		escaped := strings.ReplaceAll(v, `"`, `\"`)
		quoted[i] = `"` + escaped + `"`
	}
	return "{" + strings.Join(quoted, ",") + "}"
}

// parseArrayLiteral decode un literal tableau Postgres en slice Go.
// Exemple : {"event1","event2"} → []string{"event1", "event2"}
func parseArrayLiteral(s string) []string {
	// Supprimer les accolades encadrantes.
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	// Split naif sur virgule (sufficient pour des noms d'evenements sans virgule).
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		// Supprimer les guillemets optionnels.
		p = strings.Trim(p, `"`)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
