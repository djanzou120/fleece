package api

// campaigns_cancel.go — PATCH /campaigns/{id}/cancel
//
// Annule une campagne via la machine a etats Campaign.
//
// Transitions autorisees vers cancelled (d'apres campaignTransitions) :
//   - draft      → cancelled (OK)
//   - scheduled  → cancelled (OK)
//   - paused     → cancelled (OK)
//   - running    → cancelled (INTERDIT — retourne 409)
//   - completed  → cancelled (INTERDIT — retourne 409)
//   - failed     → cancelled (INTERDIT — retourne 409)
//
// Pipeline :
//  1. Valider {id} UUID → 400.
//  2. SELECT depuis campaign.campaigns WHERE id=$1 → 404 si absent.
//  3. Reconstruire Campaign avec Status courant.
//  4. campaign.Transition(CampaignCancelled) → 409 si ErrInvalidTransition.
//  5. UPDATE status='cancelled' WHERE id=$1.
//  6. 200 + campagne.

import (
	"database/sql"
	"errors"
	"net/http"
)

// HandleCancelCampaign traite PATCH /campaigns/{id}/cancel.
func (s *Service) HandleCancelCampaign(w http.ResponseWriter, r *http.Request) {
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

	// 2. Charger la campagne.
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
		s.Logger.Error("campaigns/cancel: select", "campaign_id", campaignID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 3. Reconstruire Campaign avec le Status courant.
	c := &Campaign{
		ID:     row.ID,
		Status: CampaignStatus(row.Status),
	}

	// 4. Appliquer la transition → CampaignCancelled.
	if err := c.Transition(CampaignCancelled); err != nil {
		if errors.Is(err, ErrInvalidTransition) {
			writeError(w, http.StatusConflict, "transition interdite depuis le statut courant : "+string(CampaignStatus(row.Status)))
			return
		}
		s.Logger.Error("campaigns/cancel: Transition()", "campaign_id", campaignID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 5. Persister le nouveau statut.
	if err := s.DB.Exec(ctx,
		`UPDATE campaign.campaigns SET status = 'cancelled' WHERE id = $1`,
		campaignID,
	); err != nil {
		s.Logger.Error("campaigns/cancel: update", "campaign_id", campaignID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 6. Retourner la campagne mise a jour.
	row.Status = string(CampaignCancelled)
	writeJSON(w, http.StatusOK, row)
}
