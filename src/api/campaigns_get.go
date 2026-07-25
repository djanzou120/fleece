package api

// campaigns_get.go — GET /campaigns/{id}
//
// Retourne une campagne par son identifiant UUID.
//
// Colonnes lues (migrations 0007 + 0016) :
//   id, workspace_id, name, status, scheduled_at, created_at,
//   message_body, channel_strategy, estimated_cost, currency, rate_limit.

import (
	"database/sql"
	"errors"
	"net/http"
)

// HandleGetCampaign traite GET /campaigns/{id}.
//
// Pipeline :
//  1. Extraire {id} depuis PathValue → valider UUID → 400 sinon.
//  2. SELECT depuis campaign.campaigns WHERE id=$1 → 404 si sql.ErrNoRows.
//  3. 200 + JSON.
func (s *Service) HandleGetCampaign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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

	var row campaignRow
	if err := s.DB.Get(ctx, &row,
		`SELECT id, workspace_id, name, status, scheduled_at, created_at,
		        message_body, channel_strategy, estimated_cost, currency, rate_limit
		   FROM campaign.campaigns
		  WHERE id = $1`,
		campaignID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "campagne introuvable")
			return
		}
		s.Logger.Error("campaigns/get: select", "campaign_id", campaignID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	writeJSON(w, http.StatusOK, row)
}
