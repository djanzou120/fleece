package api

// campaigns_schedule.go — PATCH /campaigns/{id}/schedule
//
// Planifie une campagne : draft → scheduled.
//
// Pipeline :
//  1. Valider {id} UUID, parser scheduled_at (RFC3339) → 400.
//  2. SELECT campaign depuis campaign.campaigns WHERE id=$1 → 404 si absent.
//  3. Reconstruire la struct Campaign avec le Status courant issu de la DB.
//  4. Calcul estimated_cost :
//     a. COUNT(*) des recipients.
//     b. Lire un cout unitaire depuis routing.provider_pricing (canal derive
//        de channel_strategy ; requete SEPAREE — jamais de JOIN cross-schema).
//     c. estimated_cost = count * unit_cost.
//  5. Appeler campaign.Schedule(scheduledAt).
//     ErrMissingMessageBody → 422 ; ErrInvalidTransition → 409.
//  6. UPDATE campaign.campaigns SET status='scheduled', scheduled_at=$1,
//     estimated_cost=$2 WHERE id=$3.
//  7. 200 + campagne mise a jour.
//
// Isolation D11 :
//   La derivation du canal depuis channel_strategy utilise la table
//   routing.provider_pricing dans une requete SEPAREE (pas de JOIN avec campaign.*).

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// scheduleCampaignRequest est le corps JSON de PATCH /campaigns/{id}/schedule.
type scheduleCampaignRequest struct {
	ScheduledAt string `json:"scheduled_at"` // RFC3339
}

// HandleScheduleCampaign traite PATCH /campaigns/{id}/schedule.
func (s *Service) HandleScheduleCampaign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1a. Valider l'id UUID.
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

	// 1b. Parser scheduled_at.
	var req scheduleCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requete invalide")
		return
	}
	if req.ScheduledAt == "" {
		writeError(w, http.StatusBadRequest, "scheduled_at est requis (format RFC3339)")
		return
	}
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "scheduled_at : format RFC3339 invalide (ex: 2026-07-01T10:00:00Z)")
		return
	}
	scheduledAt = scheduledAt.UTC()

	// 2. Charger la campagne depuis la DB.
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
		s.Logger.Error("campaigns/schedule: select", "campaign_id", campaignID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 3. Reconstruire la struct Campaign avec le status courant depuis la DB.
	c := &Campaign{
		ID:              row.ID,
		WorkspaceID:     row.WorkspaceID,
		Name:            row.Name,
		Status:          CampaignStatus(row.Status),
		ScheduledAt:     row.ScheduledAt,
		CreatedAt:       row.CreatedAt,
		MessageBody:     row.MessageBody,
		ChannelStrategy: row.ChannelStrategy,
		EstimatedCost:   row.EstimatedCost,
		RateLimit:       row.RateLimit,
	}

	// 4a. Compter les destinataires.
	// Requete separee (isolation D11 — pas de JOIN cross-schema).
	type countRow struct {
		Count int64 `db:"count"`
	}
	var cr countRow
	if err := s.DB.Get(ctx, &cr,
		`SELECT COUNT(*) AS count FROM campaign.campaign_recipients WHERE campaign_id = $1`,
		campaignID,
	); err != nil {
		s.Logger.Error("campaigns/schedule: count recipients", "campaign_id", campaignID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	recipientCount := cr.Count

	// 4b. Lire le cout unitaire depuis routing.provider_pricing.
	// Derivation du canal depuis channel_strategy :
	//   - "lowest_cost"      → "sms"       (canal le moins couteux, cible africaine)
	//   - "highest_delivery" → "whatsapp"  (canal a meilleure delivrabilite)
	//   - "round_robin"      → "sms"       (canal par defaut)
	//   Autre valeur         → "sms"       (fallback)
	// Cette derivation est une heuristique documentee (pas de lookup dynamique).
	// Un calcul exact par canal necessite de connaitre le ou les providers
	// selectionnes, ce qui est fait lors de l'envoi reel (M-012). L'estimated_cost
	// est une estimation a la creation / planification.
	channel := deriveChannelFromStrategy(row.ChannelStrategy)

	// Requete SEPAREE sur routing.provider_pricing (pas de JOIN avec campaign.*).
	type pricingRow struct {
		Cost int64 `db:"cost"`
	}
	var pr pricingRow
	unitCost := int64(0)
	if err := s.DB.Get(ctx, &pr,
		`SELECT cost FROM routing.provider_pricing WHERE channel = $1 LIMIT 1`,
		channel,
	); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.Logger.Error("campaigns/schedule: select pricing", "channel", channel, "err", err)
			// Non-bloquant : on continue avec unitCost=0 (estimation degradee).
		}
		// Si aucun tarif trouve pour le canal, estimation = 0.
	} else {
		unitCost = pr.Cost
	}

	// 4c. Calcul estimated_cost.
	estimatedCost := recipientCount * unitCost

	// 5. Appeler campaign.Schedule — valide la garde (MessageBody != "") + transition.
	if err := c.Schedule(scheduledAt); err != nil {
		switch {
		case errors.Is(err, ErrMissingMessageBody):
			writeError(w, http.StatusUnprocessableEntity, "message_body est requis pour planifier la campagne")
		case errors.Is(err, ErrInvalidTransition):
			writeError(w, http.StatusConflict, "transition interdite depuis le statut courant : "+string(c.Status))
		default:
			s.Logger.Error("campaigns/schedule: Schedule()", "campaign_id", campaignID, "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
		}
		return
	}

	// 6. Persister le nouveau statut, la date planifiee et le cout estime.
	if err := s.DB.Exec(ctx,
		`UPDATE campaign.campaigns
		    SET status         = 'scheduled',
		        scheduled_at   = $1,
		        estimated_cost = $2
		  WHERE id = $3`,
		scheduledAt, estimatedCost, campaignID,
	); err != nil {
		s.Logger.Error("campaigns/schedule: update", "campaign_id", campaignID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 7. Retourner la campagne mise a jour.
	row.Status = string(CampaignScheduled)
	row.ScheduledAt = &scheduledAt
	row.EstimatedCost = estimatedCost
	writeJSON(w, http.StatusOK, row)
}

// deriveChannelFromStrategy retourne le canal associe a une strategie de routage.
// Cette heuristique est utilisee pour l'estimation du cout avant envoi.
// Le canal reel est determine au moment de l'envoi par le service de routage.
//
// Mapping :
//   - "highest_delivery" → "whatsapp" (meilleure delivrabilite)
//   - "lowest_cost" / "round_robin" / autre → "sms" (canal le moins couteux / defaut)
func deriveChannelFromStrategy(strategy string) string {
	if strategy == "highest_delivery" {
		return "whatsapp"
	}
	return "sms"
}
