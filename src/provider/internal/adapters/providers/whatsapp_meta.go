// Package providers — adapters fournisseurs implementant le port Provider (couche 3).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package providers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"fleece/src/provider/internal/domain"
)

// WhatsAppMeta implemente le port output.Provider pour l'API WhatsApp Business (Meta).
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
//	Body: { "messaging_product": "whatsapp", "to": msg.Recipient, "text": { "body": msg.Content } }
//
// L'externalId retourne est le "messages[0].id" de la reponse Meta.
func (p *WhatsAppMeta) Send(_ context.Context, msg domain.OutboundMessage) (string, error) {
	// Validation de base.
	if msg.Recipient == "" {
		return "", fmt.Errorf("whatsapp_meta: recipient vide: %w", domain.ErrSendFailed)
	}
	if msg.Content == "" {
		return "", fmt.Errorf("whatsapp_meta: contenu vide: %w", domain.ErrSendFailed)
	}

	// TODO(production): executer la vraie requete HTTP vers l'API Meta.
	// Stub : genere un externalId non vide pour simuler un envoi reussi.
	externalId := fmt.Sprintf("wamid.HBgLNjM%d", time.Now().UnixNano())
	log.Printf("[whatsapp_meta:stub] Send internalId=%s recipient=%s externalId=%s",
		msg.Id, msg.Recipient, externalId)

	return externalId, nil
}

// EstimateCost retourne le cout estime d'un message WhatsApp pour le pays donne.
//
// TODO(production): interroger l'API Meta Pricing ou utiliser une grille tarifaire locale.
// Tarifs indicatifs Meta (en centimes XAF) :
//   - CM (Cameroun) : ~5 XAF par message
//   - SN (Senegal)  : ~5 XAF par message
//   - FR (France)   : ~3 EUR cents par message
func (p *WhatsAppMeta) EstimateCost(_ context.Context, channel, country string) (domain.Money, error) {
	// TODO(production): remplacer par une grille tarifaire reelle (API ou table).
	_ = channel
	switch country {
	case "FR", "BE", "CH":
		return domain.Money{Amount: 3, Currency: "EUR"}, nil
	default:
		// Afrique : tarif par defaut en XAF.
		return domain.Money{Amount: 5, Currency: "XAF"}, nil
	}
}

// GetDeliveryStatus interroge l'API Meta pour obtenir le statut de livraison.
//
// TODO(production): appeler l'endpoint de statut Meta ou exploiter les webhooks DLR
// (Meta notifie les DLR en push via webhook plutot qu'en pull).
// Dans ce stub, on retourne toujours StatusSent pour simuler un message transmis.
func (p *WhatsAppMeta) GetDeliveryStatus(_ context.Context, externalId string) (domain.DeliveryStatus, error) {
	if externalId == "" {
		return "", fmt.Errorf("whatsapp_meta: externalId vide: %w", domain.ErrProviderUnavailable)
	}
	// TODO(production): requete GET vers l'API Meta pour obtenir le vrai statut.
	log.Printf("[whatsapp_meta:stub] GetDeliveryStatus externalId=%s → sent", externalId)
	return domain.StatusSent, nil
}
