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
// `workspace_id` est REQUIS ; les autres filtres sont optionnels. Les resultats
// sont tries par created_at DESC. limit est borne a [1, 200] (defaut 50).
// offset defaut = 0.
//
// CORRECTIF D-M41 (2e occurrence, PLUS GRAVE QUE CELLE DECRITE PAR LA DETTE) —
// `workspace_id` etait OPTIONNEL ici : l'appeler SANS ce parametre retournait
// les messages de TOUS LES WORKSPACES CONFONDUS (jusqu'a maxLimit=200 lignes,
// contenu et destinataires compris), en une seule requete et sans rien avoir a
// deviner. La dette D-M41 ne mentionnait que GET /messages/{id}, qui exige au
// moins de connaitre un UUID ; cette variante-ci est une fuite EN MASSE, et le
// filtre dynamique de buildListQuery la rendait invisible a la lecture (aucun
// `WHERE workspace_id` n'apparait dans le code quand le parametre est vide).
//
// Le parametre est donc desormais REQUIS (400 s'il manque), comme sur
// /webhook-endpoints (D-M40) et sur GET /messages/{id}. Sans impact sur les
// appelants connus : le seul client, src/graphql-api/adapters/clients/
// messaging.client.ts, transmet deja systematiquement workspace_id.
func (s *Service) HandleListMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	// Parametre workspace_id : REQUIS (frontiere de securite, voir doc ci-dessus).
	workspaceID := q.Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id est requis")
		return
	}
	if _, err := parseUUID(workspaceID); err != nil {
		writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
		return
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
//
// D-M41 : le filtre workspace_id est INCONDITIONNEL — il fait partie de la
// clause WHERE de base, il n'est plus ajoute « si le parametre est non vide ».
// C'est deliberé : un filtre de securite conditionnel disparait SILENCIEUSEMENT
// du SQL quand l'appelant omet la valeur, et rien dans le code ne signale
// l'absence de frontiere. En le rendant structurel, appeler cette fonction avec
// un workspaceID vide produit une requete qui ne matche RIEN (comportement sur
// et visible), au lieu de tout retourner.
func buildListQuery(workspaceID, status string, limit, offset int) (string, []any) {
	base := `SELECT id, workspace_id, recipient, content, status, channel, created_at
	           FROM messaging.messages
	          WHERE workspace_id = $1`

	args := make([]any, 0, 4)
	args = append(args, workspaceID)
	idx := 2 // compteur de placeholder $N ($1 = workspace_id)

	if status != "" {
		base += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, status)
		idx++
	}

	base += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	return base, args
}
