package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

// routingEstimateRequest est le corps JSON de POST /routing/estimate.
type routingEstimateRequest struct {
	Recipient string `json:"recipient"` // requis
	Body      string `json:"body"`      // requis
	Strategy  string `json:"strategy"`  // requis ; lowest_cost|highest_delivery|round_robin
}

// routingEstimateResponse est la reponse JSON de POST /routing/estimate (200 OK).
type routingEstimateResponse struct {
	ProviderID         string `json:"provider_id"`
	Channel            string `json:"channel"`
	EstimatedCostCents int64  `json:"estimated_cost_cents"`
}

// HandleRoutingEstimate traite POST /routing/estimate.
//
// Pipeline :
//  1. Decode + validation (recipient, body non vides ; strategy reconnue).
//  2. Meme selection de provider que /routing/select (loadProviderScores sans filtre canal
//     car pas de canal en entree, puis SelectProvider).
//  3. SELECT cost FROM routing.provider_pricing WHERE provider=$1 AND channel=$2 LIMIT 1.
//     Si aucune ligne → cost = 0 (documente). Pas de filtre pays en M-013 (amelioration future).
//  4. Si le provider est disponible dans s.Providers : on appelle EstimateCost(recipient, body)
//     et on prend le max(pricing_cost, provider_cost) pour affiner l'estimation.
//     Sinon on retourne le cout de la table.
//  5. 200 + { provider_id, channel, estimated_cost_cents }. ErrNoProvider → 422.
func (s *Service) HandleRoutingEstimate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Decode + validation.
	var req routingEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requete invalide")
		return
	}
	if req.Recipient == "" {
		writeError(w, http.StatusBadRequest, "recipient est requis")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "body est requis")
		return
	}
	if !isValidStrategy(req.Strategy) {
		writeError(w, http.StatusBadRequest, "strategy doit etre l'une de : lowest_cost, highest_delivery, round_robin")
		return
	}

	// 2. Chargement des scores (sans filtre canal — aucun canal fourni en entree).
	// Reutilise loadProviderScores defini dans routing_select.go.
	scores, err := loadProviderScores(ctx, s, "")
	if err != nil {
		s.Logger.Error("routing/estimate: select provider_scores", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	if len(scores) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "aucun provider disponible")
		return
	}

	// Enrichissement du score contact pour highest_delivery (meme logique que /routing/select).
	// loadContactScore lit contact_intel.contacts in-process (fallback gracieux D28).
	if req.Strategy == "highest_delivery" && req.Recipient != "" {
		contactScore, _, err := s.loadContactScore(ctx, req.Recipient)
		if err != nil {
			s.Logger.Warn("routing/estimate: contact score indisponible (fallback)", "err", err)
		}
		scores = enrichScoresWithContact(scores, contactScore, err)
	}

	// Selection du provider.
	providerID, err := SelectProvider(scores, req.Strategy)
	if err != nil {
		if errors.Is(err, ErrNoProvider) {
			writeError(w, http.StatusUnprocessableEntity, "aucun provider disponible")
			return
		}
		s.Logger.Error("routing/estimate: SelectProvider", "err", err)
		writeError(w, http.StatusBadRequest, "strategie invalide")
		return
	}

	// Retrouver le canal du provider choisi.
	var selectedChannel string
	for _, sc := range scores {
		if sc.ProviderID == providerID {
			selectedChannel = sc.Channel
			break
		}
	}

	// 3. Lecture du cout depuis routing.provider_pricing.
	// Reutilise fetchProviderPricingCost defini dans routing_select.go.
	// Si aucune ligne → cost = 0 (documente : pas de tarification connue pour ce provider+canal).
	pricingCost, err := fetchProviderPricingCost(ctx, s, providerID, selectedChannel)
	if err != nil {
		s.Logger.Error("routing/estimate: provider_pricing", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 4. Affinement du cout via l'adapter provider (si disponible dans le registre).
	// Le brief mentionne que EstimateCost(recipient, body) peut affiner le cout
	// (ex. multiplication par le nombre de segments SMS). On prend le max pour
	// ne pas sous-estimer le cout (approche conservative).
	estimatedCost := pricingCost
	if prov, ok := s.Providers[providerID]; ok {
		providerCost := prov.EstimateCost(req.Recipient, req.Body)
		if providerCost > estimatedCost {
			estimatedCost = providerCost
		}
	}

	// 5. Reponse 200.
	writeJSON(w, http.StatusOK, routingEstimateResponse{
		ProviderID:         providerID,
		Channel:            selectedChannel,
		EstimatedCostCents: estimatedCost,
	})
}
