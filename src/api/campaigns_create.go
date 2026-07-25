package api

// campaigns_create.go — POST /campaigns
//
// Cree une nouvelle campagne dans l'etat initial draft.
//
// Colonnes reelles (migrations 0007 + 0016) :
//   campaign.campaigns :
//     id uuid PK, workspace_id uuid, name text, status text DEFAULT 'draft',
//     scheduled_at timestamptz (nullable), created_at timestamptz,
//     message_body text DEFAULT '', channel_strategy text DEFAULT 'lowest_cost',
//     estimated_cost bigint DEFAULT 0, currency text DEFAULT 'XAF',
//     rate_limit int DEFAULT 0.

import (
	"encoding/json"
	"net/http"
	"time"
)

// createCampaignRequest est le corps JSON de POST /campaigns.
type createCampaignRequest struct {
	WorkspaceID     string `json:"workspace_id"`
	Name            string `json:"name"`
	MessageBody     string `json:"message_body"`
	ChannelStrategy string `json:"channel_strategy"`
	RateLimit       int    `json:"rate_limit"` // optionnel ; 0 = illimite
}

// campaignRow est la representation d'une ligne campaign.campaigns en DB
// et sert aussi de DTO de reponse JSON.
// Les tags db: correspondent aux noms exacts des colonnes (migrations 0007 + 0016).
type campaignRow struct {
	ID              string     `db:"id"               json:"id"`
	WorkspaceID     string     `db:"workspace_id"     json:"workspace_id"`
	Name            string     `db:"name"             json:"name"`
	Status          string     `db:"status"           json:"status"`
	ScheduledAt     *time.Time `db:"scheduled_at"     json:"scheduled_at"`
	CreatedAt       time.Time  `db:"created_at"       json:"created_at"`
	MessageBody     string     `db:"message_body"     json:"message_body"`
	ChannelStrategy string     `db:"channel_strategy" json:"channel_strategy"`
	EstimatedCost   int64      `db:"estimated_cost"   json:"estimated_cost"`
	Currency        string     `db:"currency"         json:"currency"`
	RateLimit       int        `db:"rate_limit"       json:"rate_limit"`
}

// HandleCreateCampaign traite POST /campaigns.
//
// Pipeline :
//  1. Decoder le corps JSON + validation (workspace_id UUID, name non vide,
//     channel_strategy valide ou defaut 'lowest_cost') → 400 sinon.
//  2. Generer un UUID v4 via newUUID().
//  3. INSERT INTO campaign.campaigns.
//  4. 201 + la campagne creee (construite depuis les valeurs inserees).
func (s *Service) HandleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req createCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requete invalide")
		return
	}

	// Validation workspace_id.
	if req.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id est requis")
		return
	}
	if _, err := parseUUID(req.WorkspaceID); err != nil {
		writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
		return
	}

	// Validation name.
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name est requis")
		return
	}

	// Validation channel_strategy : defaut 'lowest_cost' si absent, sinon valider.
	strategy := req.ChannelStrategy
	if strategy == "" {
		strategy = "lowest_cost"
	} else if !isValidStrategy(strategy) {
		writeError(w, http.StatusBadRequest, "channel_strategy doit etre l'une de : lowest_cost, highest_delivery, round_robin")
		return
	}

	id := newUUID()
	now := time.Now().UTC()

	ctx := r.Context()
	if err := s.DB.Exec(ctx,
		`INSERT INTO campaign.campaigns
		  (id, workspace_id, name, status, message_body, channel_strategy, estimated_cost, currency, rate_limit, created_at)
		 VALUES ($1, $2, $3, 'draft', $4, $5, 0, 'XAF', $6, $7)`,
		id, req.WorkspaceID, req.Name, req.MessageBody, strategy, req.RateLimit, now,
	); err != nil {
		s.Logger.Error("campaigns/create: insert", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	writeJSON(w, http.StatusCreated, campaignRow{
		ID:              id,
		WorkspaceID:     req.WorkspaceID,
		Name:            req.Name,
		Status:          string(CampaignDraft),
		ScheduledAt:     nil,
		CreatedAt:       now,
		MessageBody:     req.MessageBody,
		ChannelStrategy: strategy,
		EstimatedCost:   0,
		Currency:        "XAF",
		RateLimit:       req.RateLimit,
	})
}
