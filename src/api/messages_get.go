package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"
)

// messageRow est le resultat d'un SELECT sur messaging.messages.
// Les champs correspondent exactement aux colonnes de la migration 0003.
type messageRow struct {
	ID          string    `db:"id"`
	WorkspaceID string    `db:"workspace_id"`
	Recipient   string    `db:"recipient"`
	Content     string    `db:"content"`
	Status      string    `db:"status"`
	Channel     string    `db:"channel"`
	CreatedAt   time.Time `db:"created_at"`
}

// HandleGetMessage traite GET /messages/{id}?workspace_id=<uuid>.
//
// Retourne 404 si le message n'existe pas, 200 + JSON sinon.
//
// CORRECTIF D-M41 — SCOPING WORKSPACE OBLIGATOIRE. Avant ce correctif, la
// requete etait `WHERE id = $1` seul : CONNAITRE UN UUID DE MESSAGE SUFFISAIT
// A LIRE SON CONTENU, SON DESTINATAIRE ET SON STATUT, MEME S'IL APPARTENAIT A
// UN AUTRE WORKSPACE. Defaut herite de l'ancien service messaging, mais
// AGGRAVE par l'unification : un seul service sert desormais TOUS les
// workspaces, il n'y a donc plus aucune frontiere de deploiement pour rattraper
// l'absence de filtre applicatif.
//
// `workspace_id` est un parametre REQUIS (400 s'il manque), aligne sur la
// decision de scoping prise en D-M40 pour /webhook-endpoints — et non
// optionnel comme il l'etait sur GET /messages, precisement parce qu'un filtre
// « optionnel » sur une frontiere de securite finit toujours par etre omis.
//
// 404 UNIFORME, JAMAIS 403 : un message inexistant et un message appartenant a
// un autre workspace renvoient exactement la meme reponse. Distinguer les deux
// transformerait cet endpoint en oracle d'existence — un appelant pourrait
// enumerer les identifiants valides des autres workspaces sans jamais lire leur
// contenu. Meme raisonnement que le DELETE de D-M40.
func (s *Service) HandleGetMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extraire l'id depuis le path parameter (Go 1.22+ net/http mux).
	rawID := r.PathValue("id")
	if rawID == "" {
		writeError(w, http.StatusBadRequest, "id requis dans le chemin")
		return
	}
	id, err := parseUUID(rawID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id : format UUID invalide")
		return
	}

	rawWorkspaceID := r.URL.Query().Get("workspace_id")
	if rawWorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id est requis")
		return
	}
	workspaceID, err := parseUUID(rawWorkspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
		return
	}

	var msg messageRow
	if err := s.DB.Get(ctx, &msg,
		`SELECT id, workspace_id, recipient, content, status, channel, created_at
		   FROM messaging.messages
		  WHERE id = $1 AND workspace_id = $2`,
		id, workspaceID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "message introuvable")
			return
		}
		s.Logger.Error("messages: get by id", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	writeJSON(w, http.StatusOK, msg)
}
