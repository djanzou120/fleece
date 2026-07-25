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

// HandleGetMessage traite GET /messages/{id}.
//
// Retourne 404 si le message n'existe pas, 200 + JSON sinon.
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

	var msg messageRow
	if err := s.DB.Get(ctx, &msg,
		`SELECT id, workspace_id, recipient, content, status, channel, created_at
		   FROM messaging.messages
		  WHERE id = $1`,
		id,
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
