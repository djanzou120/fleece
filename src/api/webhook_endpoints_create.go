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
//
// D-M44 — VALIDATION HTTPS DE L'URL : la user-story DASH-04 impose de refuser une
// URL d'endpoint non-HTTPS (un secret HMAC envoyé en clair vers un endpoint http://
// est un vrai problème de sécurité). Ni l'ancien service (register_endpoint.go), ni
// le portage D-M40 ne validaient le schéma — dette pré-existante à T-005, corrigée
// ici (voir validateWebhookURL). Exemption explicite : `http://localhost` et
// `http://127.0.0.1` restent acceptés, MAIS UNIQUEMENT hors production (E5
// ci-dessous) — toute autre URL non-HTTPS est rejetée en 400.
//
// E5 (BLOQUANT — anti-SSRF, revue architecture Phase 3 / D-M43) : la validation
// D-M44 ne portait que sur le SCHEMA (https/http) et le HOST syntaxique, jamais
// sur la LOCALISATION RESEAU de la cible. Or core-processor (D-M43) émet
// désormais de VRAIS POST HTTP sortants DEPUIS L'INTERIEUR DU CLUSTER vers ces
// URLs (webhook_dispatch.go) : une URL https://169.254.169.254/... (métadonnées
// cloud), https://10.0.0.5/... ou https://<svc>.<ns>.svc.cluster.local/...
// passaient TOUTES la validation D-M44 (schéma https valide, host non vide).
// Et l'exemption localhost/127.0.0.1 s'appliquait INCONDITIONNELLEMENT, y
// compris en production. CORRECTIF :
//
//  1. Toute IP LITTÉRALE privée/loopback/link-local dans le host (IPv4 ET
//     IPv6, net.ParseIP + IsPrivate()/IsLoopback()/IsLinkLocalUnicast()) est
//     désormais rejetée à la création, MÊME en HTTPS (voir
//     validateWebhookHostNotPrivate). Un nom de domaine (ex.
//     "internal.svc.cluster.local") N'EST PAS résolu ici : la résolution DNS
//     à la création ne protège pas contre le DNS rebinding au moment du
//     dispatch réel, et une refonte de ce type est explicitement HORS
//     PÉRIMÈTRE (E1, voir rapport de tâche) — seule une IP littérale est
//     détectée et bloquée par ce correctif.
//  2. L'exemption localhost/127.0.0.1 (dev) est désormais conditionnée à un
//     environnement NON-PRODUCTION (Service.Env, variable d'environnement
//     FLEECE_ENV, voir isNonProductionEnv) — comportement par défaut
//     (FLEECE_ENV absent/vide) : PRODUCTION, donc exemption DÉSACTIVÉE. Il
//     faut positionner explicitement FLEECE_ENV=development (ou dev/test/
//     local) pour l'activer.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
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
//  1. Décoder + valider le corps JSON (workspace_id UUID valide, url non vide ET
//     https [ou http localhost/127.0.0.1 en dev, D-M44], au moins un événement
//     abonné).
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
	if err := validateWebhookURL(req.URL, s.Env); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Events) == 0 {
		writeError(w, http.StatusBadRequest, "au moins un evenement est requis")
		return
	}
	if err := validateWebhookEvents(req.Events); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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

// validateWebhookURL applique D-M44 (schéma) ET E5 (anti-SSRF) : rejette
// toute URL d'endpoint qui n'est pas syntaxiquement valide, qui n'utilise pas
// le schéma "https", ou dont le host est une IP LITTÉRALE privée/loopback/
// link-local (E5) — sauf exemption explicite hors-production pour le
// développement local (http://localhost ou http://127.0.0.1, avec ou sans
// port ; voir isNonProductionEnv/isLocalhostExemption). Fonction pure,
// testable isolément. env est la valeur de Service.Env (FLEECE_ENV) — voir
// doc de tête de fichier pour le comportement par défaut (production).
//
// Rationale D-M44 : un secret HMAC (X-Fleece-Signature) envoyé en clair vers
// un endpoint http:// est interceptable.
//
// Rationale E5 : core-processor (D-M43) émet de vrais POST HTTP sortants
// DEPUIS L'INTÉRIEUR DU CLUSTER vers cette URL — un schéma https:// valide ne
// garantit RIEN sur la localisation réseau de la cible (métadonnées cloud,
// service interne, IP privée). L'exemption localhost/127.0.0.1 est
// nécessaire au développement (pas de certificat TLS local) mais ne doit
// jamais s'appliquer en production (c'est précisément là que le risque
// existe).
func validateWebhookURL(raw string, env string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errInvalidWebhookURL
	}
	host := u.Hostname()

	if u.Scheme == "http" {
		// Exemption dev localhost/127.0.0.1 (D-M44), désormais conditionnée à
		// un environnement NON-PRODUCTION (E5) — et volontairement PAS
		// soumise au filtre anti-SSRF ci-dessous : c'est précisément sa
		// raison d'être (loopback volontaire, uniquement en dev).
		if isNonProductionEnv(env) && isLocalhostExemption(host) {
			return nil
		}
		return errInvalidWebhookURL
	}
	if u.Scheme != "https" {
		return errInvalidWebhookURL
	}

	return validateWebhookHostNotPrivate(host)
}

// validateWebhookHostNotPrivate applique E5 : rejette host s'il s'agit d'une
// IP LITTÉRALE (IPv4 ou IPv6) privée, loopback, ou link-local
// (unicast/multicast) — protection anti-SSRF contre les cibles internes au
// cluster/cloud (169.254.169.254 métadonnées cloud, 10.0.0.0/8, 172.16.0.0/12,
// 192.168.0.0/16, ::1, fe80::/10, etc.). Un nom de domaine (non résolu ici,
// voir doc de validateWebhookURL) passe toujours ce filtre : seule une IP
// littérale explicite dans l'URL est détectée.
func validateWebhookHostNotPrivate(host string) error {
	ip := net.ParseIP(host)
	if ip == nil {
		// Pas une IP littérale (nom de domaine) : hors périmètre de ce
		// filtre (voir doc de tête de fichier, E1 hors périmètre).
		return nil
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return errPrivateWebhookURL
	}
	return nil
}

// errInvalidWebhookURL est l'erreur retournée (message HTTP 400) quand l'URL
// est absente, syntaxiquement invalide, ou non-HTTPS hors exemption locale.
var errInvalidWebhookURL = &webhookURLError{"url : le schema https est requis (http autorise uniquement pour localhost/127.0.0.1 hors production)"}

// errPrivateWebhookURL est l'erreur retournée (message HTTP 400, E5) quand le
// host de l'URL est une IP littérale privée/loopback/link-local.
var errPrivateWebhookURL = &webhookURLError{"url : les adresses IP privees/loopback/link-local sont interdites (protection anti-SSRF)"}

// webhookURLError implémente error avec un message stable, réutilisable en
// variable (évite errors.New à chaque appel, style déjà présent ailleurs
// dans src/api pour les erreurs de validation constantes).
type webhookURLError struct{ msg string }

func (e *webhookURLError) Error() string { return e.msg }

// isLocalhostExemption retourne true si host (sans port, déjà extrait via
// url.URL.Hostname()) désigne explicitement une machine locale de
// développement : "localhost" ou "127.0.0.1". Toute autre valeur (y compris
// "0.0.0.0", d'autres loopbacks IPv6 comme "::1", ou un nom de domaine
// quelconque) est volontairement EXCLUE de l'exemption — périmètre le plus
// étroit possible pour une dérogation de sécurité.
func isLocalhostExemption(host string) bool {
	return host == "localhost" || host == "127.0.0.1"
}

// isNonProductionEnv retourne true si env (Service.Env, variable
// d'environnement FLEECE_ENV) désigne explicitement un environnement de
// développement/test (E5). COMPORTEMENT PAR DÉFAUT : une valeur absente/vide
// (ou toute valeur non reconnue, y compris "production") est traitée comme
// PRODUCTION — l'exemption localhost/127.0.0.1 est donc DÉSACTIVÉE par
// défaut ; il faut positionner explicitement FLEECE_ENV à l'une des valeurs
// ci-dessous pour l'activer. C'est le choix le plus sûr : un déploiement mal
// configuré (variable oubliée) reste protégé plutôt que vulnérable.
func isNonProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "development", "dev", "test", "local":
		return true
	default:
		return false
	}
}

// validWebhookEvents recense les évènements RÉELLEMENT livrés par
// core-processor (E3, revue architecture Phase 3 / D-M43 — voir
// src/core-processor/webhook_dispatch.go, fanOutWebhookEvent : ce worker ne
// fait le fan-out QUE pour "message.delivered" et "message.failed").
// AVANT ce correctif, un client pouvait s'abonner à n'importe lequel des 7
// évènements proposés par le dashboard (AVAILABLE_EVENTS,
// src/platform-app/.../webhooks/page.tsx : message.created, message.queued,
// message.sent, message.delivered, message.failed, wallet.debited,
// wallet.refunded) alors que 5 d'entre eux ne sont livrés par AUCUN chemin de
// production actuel — 201 renvoyé, endpoint affiché "actif", et AUCUN webhook
// jamais reçu, sans le moindre signal. Dupliqué ici volontairement (pas
// d'import inter-services, core-processor et src/api sont deux binaires
// déployables distincts) — si core-processor apprend un jour à livrer
// d'autres évènements, CETTE LISTE DOIT ÊTRE MISE À JOUR EN MÊME TEMPS.
var validWebhookEvents = map[string]bool{
	"message.delivered": true,
	"message.failed":    true,
}

// validateWebhookEvents (E3) rejette toute souscription à un évènement non
// livrable. Fonction pure, testable isolément.
func validateWebhookEvents(events []string) error {
	for _, e := range events {
		if !validWebhookEvents[e] {
			return &webhookURLError{
				"events : evenement inconnu ou non livrable \"" + e + "\" (valeurs acceptees : message.delivered, message.failed)",
			}
		}
	}
	return nil
}
