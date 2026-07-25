package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// routingSelectRequest est le corps JSON de POST /routing/select.
// channel et recipient sont optionnels.
type routingSelectRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Recipient   string `json:"recipient"` // optionnel ; requis pour highest_delivery + enrichissement
	Channel     string `json:"channel"`   // optionnel ; filtre les scores par canal si fourni
	Strategy    string `json:"strategy"`
}

// routingSelectResponse est la reponse JSON de POST /routing/select (200 OK).
type routingSelectResponse struct {
	ProviderID     string `json:"provider_id"`
	Channel        string `json:"channel"`
	EstimatedScore int    `json:"estimated_score"`
}

// HandleRoutingSelect traite POST /routing/select.
//
// Pipeline :
//  1. Decode + validation (workspace_id UUID, strategy reconnue). recipient et channel optionnels.
//  2. SELECT scores depuis routing.provider_scores (+ filtre channel si fourni).
//  3. Si strategy=highest_delivery ET recipient non vide : enrichissement via /contacts/{phone}/score
//     (fallback silencieux, port 8080 = ce meme binaire, D28).
//  4. SelectProvider(providers, strategy) → providerID.
//  5. 200 + { provider_id, channel, estimated_score }. ErrNoProvider → 422.
func (s *Service) HandleRoutingSelect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Decode + validation.
	var req routingSelectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requete invalide")
		return
	}
	if req.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id est requis")
		return
	}
	if _, err := parseUUID(req.WorkspaceID); err != nil {
		writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
		return
	}
	if !isValidStrategy(req.Strategy) {
		writeError(w, http.StatusBadRequest, "strategy doit etre l'une de : lowest_cost, highest_delivery, round_robin")
		return
	}

	// 2. Lecture des scores depuis routing.provider_scores.
	// Note : provider_scores n'a pas de colonne "cost" (migration 0004 + 0013).
	// Cost=0 dans ProviderScore est donc correct — documente.
	scores, err := loadProviderScores(ctx, s, req.Channel)
	if err != nil {
		s.Logger.Error("routing/select: select provider_scores", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	if len(scores) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "aucun provider disponible")
		return
	}

	// 3. Enrichissement du score contact pour la strategie highest_delivery.
	// Conforme a D28 : lecture DB in-process (loadContactScore) avec fallback gracieux.
	if req.Strategy == "highest_delivery" && req.Recipient != "" {
		contactScore, _, err := s.loadContactScore(ctx, req.Recipient)
		if err != nil {
			s.Logger.Warn("routing/select: contact score indisponible (fallback)", "err", err)
		}
		scores = enrichScoresWithContact(scores, contactScore, err)
	}

	// 4. Selection du provider.
	providerID, err := SelectProvider(scores, req.Strategy)
	if err != nil {
		if errors.Is(err, ErrNoProvider) {
			writeError(w, http.StatusUnprocessableEntity, "aucun provider disponible")
			return
		}
		// ErrInvalidStrategy ne peut pas survenir ici (valide en etape 1).
		s.Logger.Error("routing/select: SelectProvider", "err", err)
		writeError(w, http.StatusBadRequest, "strategie invalide")
		return
	}

	// Retrouver le canal et le score du provider choisi depuis la slice.
	var selectedChannel string
	var selectedScore int
	for _, sc := range scores {
		if sc.ProviderID == providerID {
			selectedChannel = sc.Channel
			selectedScore = sc.Score
			break
		}
	}

	// 5. Reponse 200.
	writeJSON(w, http.StatusOK, routingSelectResponse{
		ProviderID:     providerID,
		Channel:        selectedChannel,
		EstimatedScore: selectedScore,
	})
}

// loadProviderScores charge les scores depuis routing.provider_scores.
// Si channel est non vide, filtre par canal. Les scores sont tries par score DESC.
// Retourne une slice de ProviderScore avec Cost=0 (absent de la table).
func loadProviderScores(ctx context.Context, s *Service, channel string) ([]ProviderScore, error) {
	var rows []providerScoreRow
	var err error

	if channel != "" {
		err = s.DB.Select(ctx, &rows,
			`SELECT provider, channel, score
			   FROM routing.provider_scores
			  WHERE channel = $1
			  ORDER BY score DESC`,
			channel,
		)
	} else {
		err = s.DB.Select(ctx, &rows,
			`SELECT provider, channel, score
			   FROM routing.provider_scores
			  ORDER BY score DESC`,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("provider_scores: %w", err)
	}

	scores := make([]ProviderScore, len(rows))
	for i, row := range rows {
		scores[i] = ProviderScore{
			ProviderID: row.Provider,
			Channel:    row.Channel,
			Score:      row.Score,
			Cost:       0, // provider_scores n'a pas de colonne cost (migration 0004)
		}
	}
	return scores, nil
}

// fetchProviderPricingCost interroge routing.provider_pricing pour obtenir le
// cout unitaire (centimes) pour un provider + channel donne.
// Retourne 0 si aucune ligne n'existe (sql.ErrNoRows, fallback documente).
// Toute AUTRE erreur DB (panne, timeout, etc.) est remontee a l'appelant — une
// panne DB ne doit jamais se degrader silencieusement en cout 0.
// Prend le premier pays si plusieurs lignes existent (le choix du pays est hors
// scope de M-013 — amelioration future : filtrer par pays du destinataire).
func fetchProviderPricingCost(ctx context.Context, s *Service, providerID, channel string) (int64, error) {
	var cost int64
	err := s.DB.Get(ctx, &cost,
		`SELECT cost
		   FROM routing.provider_pricing
		  WHERE provider = $1
		    AND channel  = $2
		  LIMIT 1`,
		providerID, channel,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Aucune tarification connue → cost estime = 0 (documente).
			return 0, nil
		}
		return 0, fmt.Errorf("provider_pricing: %w", err)
	}
	return cost, nil
}
