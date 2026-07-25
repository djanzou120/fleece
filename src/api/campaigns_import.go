package api

// campaigns_import.go — POST /campaigns/{id}/recipients
//
// Importe une liste de destinataires dans une campagne.
// Les doublons (campaign_id, recipient) sont ignores (ON CONFLICT DO NOTHING)
// grace a l'index unique uq_campaign_recipients_campaign_recipient (migration 0016).
//
// Technique de comptage des inseres :
//   INSERT ... ON CONFLICT DO NOTHING RETURNING id
//   Le nombre de lignes retournees = nombre d'inseres (les doublons ne retournent rien).
//   tx.Select scanne dans []int64 pour obtenir la liste des id bigserial inseres.
//
// Pipeline :
//  1. Valider {id} UUID, liste non vide → 400.
//  2. Verifier que la campagne existe → 404 sinon.
//  3. Transaction : pour chaque recipient inserer via RETURNING et compter.
//  4. 200 { "imported": N, "skipped": M }.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

// importRecipientsRequest est le corps JSON de POST /campaigns/{id}/recipients.
type importRecipientsRequest struct {
	Recipients []string `json:"recipients"`
}

// importRecipientsResponse est la reponse JSON (200 OK).
type importRecipientsResponse struct {
	Imported int `json:"imported"` // nombre de lignes effectivement inserees
	Skipped  int `json:"skipped"`  // doublons ignores (ON CONFLICT DO NOTHING)
}

// isNotFound teste si une erreur de db.Get correspond a sql.ErrNoRows.
// Centralise le test pour eviter de dupliquer l'import database/sql.
func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// HandleImportRecipients traite POST /campaigns/{id}/recipients.
func (s *Service) HandleImportRecipients(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Valider l'id UUID.
	rawID := r.PathValue("id")
	if rawID == "" {
		writeError(w, http.StatusBadRequest, "id requis dans le chemin")
		return
	}
	campaignID, err := parseUUID(rawID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id : format UUID invalide")
		return
	}

	// Decoder le corps JSON.
	var req importRecipientsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requete invalide")
		return
	}
	if len(req.Recipients) == 0 {
		writeError(w, http.StatusBadRequest, "recipients ne doit pas etre vide")
		return
	}

	// 2. Verifier que la campagne existe.
	type existsRow struct {
		ID string `db:"id"`
	}
	var ex existsRow
	if err := s.DB.Get(ctx, &ex,
		`SELECT id FROM campaign.campaigns WHERE id = $1`,
		campaignID,
	); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "campagne introuvable")
			return
		}
		s.Logger.Error("campaigns/import: check campaign exists", "campaign_id", campaignID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 3. Transaction : inserer les destinataires.
	// Technique : INSERT ... ON CONFLICT DO NOTHING RETURNING id
	// Les lignes retournees dans ids correspondent aux inserts reels.
	// Les doublons ne retournent rien → counted as skipped.
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		s.Logger.Error("campaigns/import: begin tx", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	defer func() { _ = tx.Rollback() }()

	imported := 0
	for _, recipient := range req.Recipients {
		if recipient == "" {
			continue // ignorer les entrees vides dans la liste
		}
		// RETURNING id : si ON CONFLICT DO NOTHING, aucune ligne retournee.
		// tx.Select scanne dans []int64 (colonne bigserial).
		var ids []int64
		if err := tx.Select(ctx, &ids,
			`INSERT INTO campaign.campaign_recipients (campaign_id, recipient, status)
			 VALUES ($1, $2, 'pending')
			 ON CONFLICT (campaign_id, recipient) DO NOTHING
			 RETURNING id`,
			campaignID, recipient,
		); err != nil {
			s.Logger.Error("campaigns/import: insert recipient", "campaign_id", campaignID, "recipient", recipient, "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		imported += len(ids)
	}

	if err := tx.Commit(); err != nil {
		s.Logger.Error("campaigns/import: commit", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// Calculer les doublons : total demandes (non vides) - inseres.
	nonEmpty := 0
	for _, r := range req.Recipients {
		if r != "" {
			nonEmpty++
		}
	}
	skipped := nonEmpty - imported

	// 4. Reponse.
	writeJSON(w, http.StatusOK, importRecipientsResponse{
		Imported: imported,
		Skipped:  skipped,
	})
}
