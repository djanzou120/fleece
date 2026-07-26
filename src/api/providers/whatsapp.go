package providers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"fleece/src/api"
	golog "fleece/src/go/log"
)

// Assertion de conformite au port : ne compile pas si WhatsAppMeta n'implemente pas api.Provider.
var _ api.Provider = (*WhatsAppMeta)(nil)

// WhatsAppMeta implemente api.Provider pour l'API WhatsApp Business (Meta).
//
// TODO(production): remplacer les stubs par de vrais appels HTTP a l'API Meta Cloud :
//   - POST https://graph.facebook.com/v18.0/{phone_number_id}/messages
//   - Authentification : Bearer token (WHATSAPP_META_TOKEN)
//   - Parsing de la reponse pour extraire le message ID Meta
type WhatsAppMeta struct {
	// client est le client HTTP utilise pour les appels vers l'API Meta.
	client *http.Client

	// baseURL est l'URL de base de l'API Meta Graph.
	// TODO(production): lire depuis la config (env WHATSAPP_META_BASE_URL).
	baseURL string

	// token est le Bearer token d'authentification Meta.
	// TODO(production): lire depuis la config (env WHATSAPP_META_TOKEN) et ne jamais logger.
	token string

	// phoneNumberID est l'identifiant du numero WhatsApp Business.
	// TODO(production): lire depuis la config (env WHATSAPP_META_PHONE_NUMBER_ID).
	phoneNumberID string

	// Logger est le logger structure partage (D-M17, Phase 3), optionnel :
	// nil ⇒ aucun log emis (garde nil, meme pattern que s.AMQP == nil
	// ailleurs dans le depot). Cable par providers.BuildRegistry depuis le
	// composition root (src/api/cmd/api/main.go). Jamais log.Printf stdlib.
	Logger *golog.Logger
}

// NewWhatsAppMeta cree un adapter WhatsApp Meta avec les parametres fournis.
// En production, baseURL = "https://graph.facebook.com/v18.0".
func NewWhatsAppMeta(baseURL, token, phoneNumberID string) *WhatsAppMeta {
	return &WhatsAppMeta{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL:       baseURL,
		token:         token,
		phoneNumberID: phoneNumberID,
	}
}

// Send envoie un message WhatsApp via l'API Meta Cloud.
//
// TODO(production): construire et executer la vraie requete HTTP :
//
//	POST {baseURL}/{phoneNumberID}/messages
//	Authorization: Bearer {token}
//	Content-Type: application/json
//	Body: { "messaging_product": "whatsapp", "to": to, "text": { "body": body } }
//
// L'ExternalID retourne est le "messages[0].id" de la reponse Meta.
func (p *WhatsAppMeta) Send(_ context.Context, to, body string) (api.ProviderResult, error) {
	// Validation de base.
	if to == "" {
		return api.ProviderResult{}, fmt.Errorf("whatsapp_meta: recipient vide: %w", errSendFailed)
	}
	if body == "" {
		return api.ProviderResult{}, fmt.Errorf("whatsapp_meta: contenu vide: %w", errSendFailed)
	}

	// TODO(production): executer la vraie requete HTTP vers l'API Meta.
	// Stub : genere un externalId non vide pour simuler un envoi reussi.
	externalID := fmt.Sprintf("wamid.HBgLNjM%d", time.Now().UnixNano())
	logInfo(p.Logger, "whatsapp_meta: stub send", "to", to, "external_id", externalID)

	return api.ProviderResult{
		ExternalID: externalID,
		Status:     "sent",
		Cost:       p.EstimateCost(to, body),
	}, nil
}

// EstimateCost retourne le cout estime d'un message WhatsApp en centimes.
//
// TODO(production): interroger l'API Meta Pricing ou utiliser une grille tarifaire locale.
// Tarifs indicatifs Meta (en centimes) :
//   - CM (Cameroun) : ~5 XAF par message
//   - SN (Senegal)  : ~5 XAF par message
//   - FR (France)   : ~3 EUR cents par message
//
// Le parametre to est le numero E.164 du destinataire.
// body est le contenu du message.
func (p *WhatsAppMeta) EstimateCost(_, _ string) int64 {
	// TODO(production): remplacer par une grille tarifaire reelle.
	// Stub : cout fixe de 5 centimes (XAF) par message.
	return 5
}

// GetDeliveryStatus interroge l'API Meta pour obtenir le statut de livraison.
//
// TODO(production): appeler l'endpoint de statut Meta ou exploiter les webhooks DLR
// (Meta notifie les DLR en push via webhook plutot qu'en pull).
// Dans ce stub, on retourne toujours "sent" pour simuler un message transmis.
func (p *WhatsAppMeta) GetDeliveryStatus(_ context.Context, externalID string) (string, error) {
	if externalID == "" {
		return "", fmt.Errorf("whatsapp_meta: externalId vide: %w", errProviderUnavailable)
	}
	// TODO(production): requete GET vers l'API Meta pour obtenir le vrai statut.
	logInfo(p.Logger, "whatsapp_meta: stub get_delivery_status", "external_id", externalID, "status", "sent")
	return "sent", nil
}
