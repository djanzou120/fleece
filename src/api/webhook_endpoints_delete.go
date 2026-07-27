package api

// webhook_endpoints_delete.go — DELETE /webhook-endpoints/{id}?workspace_id=<uuid>
//
// Voir webhook_endpoints_create.go pour le contexte général (D-M40 / T-005).
//
// Décision suppression logique vs physique (D-M40) : logique (active = false), PAS
// un DELETE physique. Deux raisons :
//  1. webhook.webhook_deliveries.endpoint_id porte une FOREIGN KEY vers
//     webhook_endpoints(id) (migrations 0006 + 0010) — un DELETE physique échouerait
//     dès qu'une livraison existe pour cet endpoint (violation de contrainte).
//  2. La colonne "active" existe précisément pour ça (ajoutée par 0010) : elle permet
//     de désactiver un endpoint tout en préservant l'historique de livraison
//     (webhook_deliveries), utile pour l'audit/debug. GET /webhook-endpoints
//     (webhook_endpoints_list.go) filtre sur active = true, donc un endpoint
//     désactivé disparaît bien de la liste — le comportement observable côté API
//     est identique à une suppression.
//
// Scoping obligatoire (D-M41) : la désactivation est conditionnée à
// workspace_id = $2, sinon n'importe quel appelant pourrait désactiver l'endpoint
// d'un autre workspace en devinant son UUID. 404 si aucune ligne n'a été affectée
// (id inconnu, déjà inactif, ou n'appartenant pas à ce workspace — même réponse
// dans les trois cas, pour ne pas fuiter l'existence d'un endpoint d'un autre
// workspace).

import (
	"net/http"
)

// HandleDeleteWebhookEndpoint traite DELETE /webhook-endpoints/{id}?workspace_id=<uuid>.
//
// Pipeline :
//  1. Valider {id} et workspace_id (requis, UUID valides) → 400 sinon.
//  2. UPDATE ... SET active = false WHERE id = $1 AND workspace_id = $2 AND active = true.
//  3. 0 ligne affectée → 404. Sinon → 204 No Content.
func (s *Service) HandleDeleteWebhookEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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

	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id est requis")
		return
	}
	workspaceID, err = parseUUID(workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
		return
	}

	n, err := s.DB.ExecRows(ctx,
		`UPDATE webhook.webhook_endpoints
		    SET active = false
		  WHERE id = $1
		    AND workspace_id = $2
		    AND active = true`,
		id, workspaceID,
	)
	if err != nil {
		s.Logger.Error("webhook-endpoints/delete: update", "id", id, "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "endpoint webhook introuvable")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
