package api

// webhook_endpoints_list.go — GET /webhook-endpoints?workspace_id=<uuid>
//
// Voir webhook_endpoints_create.go pour le contexte général (D-M40 / T-005).
//
// Scoping obligatoire (D-M41, ne pas reproduire la même dette que /messages/{id}) :
// workspace_id est un paramètre requis — sans lui, un appelant verrait les endpoints
// (et surtout la simple existence/URL, pas le secret) de n'importe quel autre
// workspace. Seuls les endpoints actifs (active = true) sont retournés : la
// suppression est logique (voir webhook_endpoints_delete.go), donc un endpoint
// supprimé ne doit plus apparaître ici.
//
// Décision de sécurité (D-M40) : cette réponse NE CONTIENT JAMAIS le secret — le
// type webhookEndpointListRow n'a volontairement pas de champ Secret. Le secret en
// clair n'est disponible qu'à la création (POST, réponse 201).
//
// Décision de nommage de paramètre (D-M40) : `workspace_id` (snake_case), cohérent
// avec le reste de src/api (cf. messages_list.go, campaigns_list.go). Le client TS
// (src/graphql-api/adapters/clients/webhook.client.ts) envoie actuellement
// `?workspaceId=` (camelCase) — écart à ajuster côté TS par le PM ; voir le rapport
// de la tâche D-M40 pour le détail de cet arbitrage.

import (
	"net/http"
	"time"

	"github.com/lib/pq"
)

// webhookEndpointListRow est le résultat d'un SELECT sur webhook.webhook_endpoints,
// SANS la colonne secret (jamais renvoyée en dehors de la création).
type webhookEndpointListRow struct {
	ID          string         `db:"id"           json:"id"`
	WorkspaceID string         `db:"workspace_id" json:"workspace_id"`
	URL         string         `db:"url"          json:"url"`
	Events      pq.StringArray `db:"events"       json:"events"`
	Active      bool           `db:"active"       json:"active"`
	CreatedAt   time.Time      `db:"created_at"   json:"created_at"`
}

// HandleListWebhookEndpoints traite GET /webhook-endpoints?workspace_id=<uuid>.
//
// Pipeline :
//  1. Valider workspace_id (requis, UUID valide) → 400 sinon.
//  2. SELECT scopé au workspace, filtré sur active = true, sans la colonne secret.
//  3. 200 + tableau JSON (jamais null).
func (s *Service) HandleListWebhookEndpoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id est requis")
		return
	}
	workspaceID, err := parseUUID(workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
		return
	}

	var rows []webhookEndpointListRow
	if err := s.DB.Select(ctx, &rows,
		`SELECT id, workspace_id, url, events, active, created_at
		   FROM webhook.webhook_endpoints
		  WHERE workspace_id = $1
		    AND active = true
		  ORDER BY created_at ASC`,
		workspaceID,
	); err != nil {
		s.Logger.Error("webhook-endpoints/list: select", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	if rows == nil {
		rows = []webhookEndpointListRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}
