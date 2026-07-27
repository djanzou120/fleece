package api

// webhook_endpoints_create.go — POST /webhook-endpoints
//
// Porte le CRUD des webhooks SORTANTS (D-M40 / T-005), absent de src/api depuis la
// Phase 2 de la migration. Référence : src/webhook/internal/application/usecases/
// register_endpoint.go + internal/adapters/http/{handler,dto}.go + internal/domain/
// endpoint.go (service à supprimer par M-023 — logique réécrite ici à plat, aucun
// import de fleece/src/webhook).
//
// Schéma (migrations 0006_webhook.sql + 0010_webhook_schema.sql, inchangé) :
//
//	webhook.webhook_endpoints : id uuid PK, workspace_id uuid NOT NULL, url text NOT NULL,
//	  secret text NOT NULL, events text[] NOT NULL, created_at timestamptz, active boolean
//	  NOT NULL DEFAULT true.
//
// Décision de sécurité (D-M40, alignée sur le pattern api-keys de src/auth-api) : le
// secret HMAC en clair n'est retourné QU'À LA CRÉATION (réponse 201 ci-dessous).
// GET /webhook-endpoints (webhook_endpoints_list.go) NE renvoie JAMAIS le secret —
// c'est la seule occasion pour l'appelant de le récupérer, exactement comme une clé
// d'API. Voir webhook_endpoints_list.go pour le DTO de lecture sans secret.
//
// Décision de nommage de route (D-M40) : `/webhook-endpoints` (et non `/endpoints`)
// pour deux raisons — (1) `/webhooks/*` est déjà pris par les callbacks entrants
// (webhooks_om.go, webhooks_mtn.go, webhooks_telegram.go) ; (2) c'est exactement ce
// que src/graphql-api/adapters/clients/webhook.client.ts appelle déjà (0 changement
// côté TS pour le chemin lui-même).

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/lib/pq"
)

// createWebhookEndpointRequest est le corps JSON de POST /webhook-endpoints.
type createWebhookEndpointRequest struct {
	WorkspaceID string   `json:"workspace_id"`
	URL         string   `json:"url"`
	Secret      string   `json:"secret,omitempty"` // optionnel : généré via crypto/rand si absent
	Events      []string `json:"events"`
}

// createWebhookEndpointResponse est la réponse JSON de POST /webhook-endpoints (201).
// Secret présent en clair — UNIQUEMENT dans cette réponse (voir décision ci-dessus).
type createWebhookEndpointResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	URL         string    `json:"url"`
	Secret      string    `json:"secret"`
	Events      []string  `json:"events"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

// HandleCreateWebhookEndpoint traite POST /webhook-endpoints.
//
// Pipeline :
//  1. Décoder + valider le corps JSON (workspace_id UUID valide, url non vide,
//     au moins un événement abonné).
//  2. Générer un secret cryptographiquement sûr si absent (crypto/rand, 32 octets
//     hex — méthode reprise à l'identique de l'ancien register_endpoint.go).
//  3. INSERT dans webhook.webhook_endpoints (active=true par défaut à la création).
//  4. 201 + endpoint créé, secret EN CLAIR (seule occasion).
func (s *Service) HandleCreateWebhookEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req createWebhookEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requete invalide")
		return
	}

	if req.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id est requis")
		return
	}
	workspaceID, err := parseUUID(req.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url est requise")
		return
	}
	if len(req.Events) == 0 {
		writeError(w, http.StatusBadRequest, "au moins un evenement est requis")
		return
	}

	secret := req.Secret
	if secret == "" {
		secret, err = generateWebhookSecret()
		if err != nil {
			s.Logger.Error("webhook-endpoints/create: generer secret", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
	}

	id := newUUID()
	createdAt := time.Now().UTC()

	if err := s.DB.Exec(ctx,
		`INSERT INTO webhook.webhook_endpoints (id, workspace_id, url, secret, events, active, created_at)
		 VALUES ($1, $2, $3, $4, $5::text[], true, $6)`,
		id, workspaceID, req.URL, secret, pq.Array(req.Events), createdAt,
	); err != nil {
		s.Logger.Error("webhook-endpoints/create: insert", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	writeJSON(w, http.StatusCreated, createWebhookEndpointResponse{
		ID:          id,
		WorkspaceID: workspaceID,
		URL:         req.URL,
		Secret:      secret,
		Events:      req.Events,
		Active:      true,
		CreatedAt:   createdAt,
	})
}

// generateWebhookSecret genere un secret hexadecimal de 32 octets (256 bits) via
// crypto/rand — reprise a l'identique de src/webhook/internal/application/usecases/
// register_endpoint.go (generateSecret), seule methode connue et deja validee.
func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
