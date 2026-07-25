package api

import (
	"fmt"
	"net/http"
	"strconv"
)

const (
	// defaultLimit est la valeur par defaut du parametre limit.
	defaultLimit = 50
	// maxLimit est la valeur maximale autorisee du parametre limit.
	maxLimit = 200
)

// HandleListMessages traite GET /messages?workspace_id=&status=&limit=&offset=
//
// Tous les filtres sont optionnels. Les resultats sont tries par created_at DESC.
// limit est borne a [1, 200] (defaut 50). offset defaut = 0.
func (s *Service) HandleListMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	// Parametre workspace_id (optionnel mais si fourni doit etre un UUID valide).
	workspaceID := q.Get("workspace_id")
	if workspaceID != "" {
		if _, err := parseUUID(workspaceID); err != nil {
			writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
			return
		}
	}

	// Parametre status (optionnel, pas de validation stricte — filtre dynamique).
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

	// Construction de la requete avec filtres dynamiques.
	// On utilise des placeholders positionnels ($1, $2, …) pour lib/pq.
	query, args := buildListQuery(workspaceID, status, limit, offset)

	var msgs []messageRow
	if err := s.DB.Select(ctx, &msgs, query, args...); err != nil {
		s.Logger.Error("messages: list", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// Retourner un tableau vide plutot que null si aucun resultat.
	if msgs == nil {
		msgs = []messageRow{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

// buildListQuery construit la requete SELECT avec les filtres optionnels.
// Retourne la requete SQL et les arguments positionnels associes.
func buildListQuery(workspaceID, status string, limit, offset int) (string, []any) {
	base := `SELECT id, workspace_id, recipient, content, status, channel, created_at
	           FROM messaging.messages
	          WHERE 1=1`

	args := make([]any, 0, 4)
	idx := 1 // compteur de placeholder $N

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
