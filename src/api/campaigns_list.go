package api

// campaigns_list.go — GET /campaigns?workspace_id=&status=&limit=&offset=
//
// Liste les campagnes avec filtres optionnels.
// Trie par created_at DESC. Pagination : limit [1,200] defaut 50, offset >= 0 defaut 0.

import (
	"fmt"
	"net/http"
	"strconv"
)

// HandleListCampaigns traite GET /campaigns?workspace_id=&status=&limit=&offset=
//
// Pipeline :
//  1. Lire et valider les query params.
//  2. Construire la requete SELECT avec filtres dynamiques.
//  3. 200 + tableau JSON (tableau vide si aucun resultat).
func (s *Service) HandleListCampaigns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	// Parametre workspace_id (optionnel ; si fourni doit etre un UUID valide).
	workspaceID := q.Get("workspace_id")
	if workspaceID != "" {
		if _, err := parseUUID(workspaceID); err != nil {
			writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
			return
		}
	}

	// Parametre status (optionnel, pas de validation stricte).
	status := q.Get("status")

	// Parametre limit.
	limit := defaultLimit
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "limit doit etre un entier positif")
			return
		}
		if n > maxLimit {
			n = maxLimit
		}
		limit = n
	}

	// Parametre offset.
	offset := 0
	if raw := q.Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset doit etre un entier >= 0")
			return
		}
		offset = n
	}

	// Construction de la requete avec filtres dynamiques (placeholders positionnels $N).
	query, args := buildCampaignListQuery(workspaceID, status, limit, offset)

	var campaigns []campaignRow
	if err := s.DB.Select(ctx, &campaigns, query, args...); err != nil {
		s.Logger.Error("campaigns/list: select", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// Retourner un tableau vide plutot que null si aucun resultat.
	if campaigns == nil {
		campaigns = []campaignRow{}
	}
	writeJSON(w, http.StatusOK, campaigns)
}

// buildCampaignListQuery construit la requete SELECT avec les filtres optionnels.
func buildCampaignListQuery(workspaceID, status string, limit, offset int) (string, []any) {
	base := `SELECT id, workspace_id, name, status, scheduled_at, created_at,
	                message_body, channel_strategy, estimated_cost, currency, rate_limit
	           FROM campaign.campaigns
	          WHERE 1=1`

	args := make([]any, 0, 4)
	idx := 1

	if workspaceID != "" {
		base += fmt.Sprintf(" AND workspace_id = $%d", idx)
		args = append(args, workspaceID)
		idx++
	}
	if status != "" {
		base += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, status)
		idx++
	}

	base += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	return base, args
}
