package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"fleece/src/routing/internal/domain"
)

// RuleRepository implemente output.RoutingRuleRepository via *sql.DB.
// N'accede qu'au schema "routing" (table routing.routing_rules).
type RuleRepository struct {
	db *sql.DB
}

// NewRuleRepository cree un RuleRepository avec la connexion fournie.
func NewRuleRepository(db *sql.DB) *RuleRepository {
	return &RuleRepository{db: db}
}

// GetByWorkspace retourne la regle de routage d'un workspace.
// Retourne domain.ErrRuleNotFound si aucune regle n'est configuree.
func (r *RuleRepository) GetByWorkspace(ctx context.Context, workspaceID string) (domain.RoutingRule, error) {
	const query = `
		SELECT workspace_id, strategy
		  FROM routing.routing_rules
		 WHERE workspace_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, workspaceID)
	var rec ruleRecord
	err := row.Scan(&rec.WorkspaceID, &rec.Strategy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.RoutingRule{}, fmt.Errorf("persistence: GetByWorkspace %s: %w", workspaceID, domain.ErrRuleNotFound)
		}
		return domain.RoutingRule{}, fmt.Errorf("persistence: GetByWorkspace %s: %w", workspaceID, err)
	}
	return rec.toEntity(), nil
}
