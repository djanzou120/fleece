package providers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"fleece/src/api"
)

// Assertion de conformite au port : ne compile pas si TelegramBot n'implemente pas api.Provider.
var _ api.Provider = (*TelegramBot)(nil)

// Capabilities du canal Telegram (constantes Go — decision D27 : pas de colonne base).
//
// Ces constantes documentent les capacites techniques de l'adapter Telegram Bot API.
// Elles sont definies ici car elles ne font l'objet d'aucune requete SQL et n'appartiennent
// pas au domaine metier. Si un besoin dynamique emergeait en V2, une colonne JSONB dans
// provider.providers serait une migration additive triviale.
const (
	// TelegramMaxMessageLength est le nombre maximal de caracteres autorise par message
	// Telegram (limite de l'API Bot Telegram, champ "text").
	TelegramMaxMessageLength = 4096

	// TelegramSupportsRichMedia indique que Telegram supporte les medias riches
	// (images, documents, audio, video via les methodes sendPhoto, sendDocument, etc.).
	// L'adapter actuel ne gere que le texte (sendMessage) ; les medias seront ajoutes
	// dans une iteration suivante.
	TelegramSupportsRichMedia = true

	// TelegramRequiresTemplate indique que Telegram n'impose aucun template pre-approuve.
	// Contrairement a WhatsApp Business, les messages Telegram sont libres.
	TelegramRequiresTemplate = false

	// TelegramChannel est le nom canonique du canal utilise dans le registry et le routing.
	TelegramChannel = "telegram"
)

// TelegramBot implemente api.Provider pour l'API Telegram Bot.
//
// TODO(production): remplacer les stubs par de vrais appels HTTP a l'API Telegram Bot :
//   - POST https://api.telegram.org/bot{token}/sendMessage
//   - Corps JSON : { "chat_id": to, "text": body }
//   - Authentification : le token est inclus dans l'URL (jamais dans les logs).
//
// TODO(production) D27: le bot token est actuellement charge depuis la config (env
// TELEGRAM_BOT_TOKEN). La decision D27 prevoit a terme son stockage chiffre AES-GCM
// dans provider.provider_credentials (colonne secret_enc bytea). Le mecanisme de
// dechiffrement (cle AES-GCM injectee via Secret K8s) reste a implementer
// (voir T-024 DevOps). Quand ce mecanisme sera disponible, remplacer la lecture
// depuis la config par une lecture dans la base via ProviderRepository.
// Ne jamais logger le token.
type TelegramBot struct {
	// client est le client HTTP utilise pour les appels vers l'API Telegram.
	client *http.Client

	// baseURL est l'URL de base de l'API Telegram Bot.
	// En production : "https://api.telegram.org".
	// Parametre via env TELEGRAM_BASE_URL (permet de pointer sur un stub en test).
	baseURL string

	// botToken est le token du bot Telegram (format "123456:ABC-DEF...").
	// Charge depuis env TELEGRAM_BOT_TOKEN. Ne jamais logger.
	// TODO(production) D27: lire depuis provider_credentials (AES-GCM) quand disponible.
	botToken string
}

// NewTelegramBot cree un adapter Telegram Bot avec les parametres fournis.
//
// baseURL : URL de base de l'API Telegram (ex. "https://api.telegram.org").
// botToken : token du bot Telegram. Ne jamais logger ce parametre.
//
// Le client HTTP est configure avec un timeout de 10 secondes.
func NewTelegramBot(baseURL, botToken string) *TelegramBot {
	return &TelegramBot{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL:  baseURL,
		botToken: botToken,
	}
}

// Send envoie un message Telegram via l'API Bot.
//
// to doit contenir le chat_id Telegram du destinataire (entier ou @username).
// body doit etre non vide et inferieur a TelegramMaxMessageLength caracteres.
//
// TODO(production): construire et executer la vraie requete HTTP :
//
//	POST {baseURL}/bot{botToken}/sendMessage
//	Content-Type: application/json
//	Body: { "chat_id": to, "text": body }
//
// L'ExternalID retourne sera le champ "result.message_id" (entier) de la reponse
// Telegram, convertie en string (ex. "42"). Ce message_id permet de correlater
// les DLR et de referencer le message pour les operations ulterieures.
//
// CONTRAT external_id (D-M13, partage avec webhooks_telegram.go) : l'ExternalID
// retourne par Send() DOIT toujours etre la representation decimale d'un entier
// (le message_id Telegram), jamais une valeur prefixee ou opaque — c'est cette
// meme forme que webhooks_telegram.go.resolveTelegramCorrelation compare a
// update.message.message_id (converti en string) pour correler un DLR.
// IMPORTANT : ne jamais inclure botToken dans les logs.
func (p *TelegramBot) Send(_ context.Context, to, body string) (api.ProviderResult, error) {
	// Validation de base.
	if to == "" {
		return api.ProviderResult{}, fmt.Errorf("telegram_bot: recipient vide: %w", errSendFailed)
	}
	if body == "" {
		return api.ProviderResult{}, fmt.Errorf("telegram_bot: contenu vide: %w", errSendFailed)
	}

	// TODO(production): executer la vraie requete HTTP vers l'API Telegram et
	// utiliser le veritable result.message_id retourne par sendMessage.
	// Stub (D-M08, hors perimetre ici) : simule un message_id Telegram par un
	// entier positif base sur l'horloge, SANS prefixe non numerique — respecter
	// strictement le contrat decrit ci-dessus (D-M13 : un stub qui ne produit pas
	// un entier decimal casse silencieusement toute correlation DLR cote webhook).
	externalID := strconv.FormatInt(time.Now().UnixNano(), 10)
	log.Printf("[telegram_bot:stub] Send to=%s externalId=%s", to, externalID)
	// NOTE : le botToken n'est jamais inclus dans les logs.

	return api.ProviderResult{
		ExternalID: externalID,
		Status:     "sent",
		Cost:       p.EstimateCost(to, body),
	}, nil
}

// EstimateCost retourne le cout estime d'un message Telegram en centimes.
//
// L'API Telegram Bot est entierement GRATUITE — Telegram ne facture pas les messages
// envoyes via Bot API, contrairement a WhatsApp Business ou aux SMS.
// Le cout est donc toujours 0, quelle que soit la destination.
func (p *TelegramBot) EstimateCost(_, _ string) int64 {
	// Telegram Bot API est gratuit : montant = 0.
	return 0
}

// GetDeliveryStatus retourne le statut de livraison d'un message Telegram.
//
// NOTE : l'API Telegram Bot ne fournit pas de mecanisme de DLR (Delivery Report)
// en mode pull. Telegram confirme l'envoi via la reponse a sendMessage (champ ok=true),
// mais ne notifie pas ulterieurement si le message a ete lu ou livre. Les "read receipts"
// existent dans l'interface utilisateur Telegram mais ne sont pas accessibles via Bot API.
//
// TODO(production): en production, le statut "delivered" pourra etre approche via
// les webhooks Telegram (getUpdates ou setWebhook) si le bot recoit une confirmation,
// mais ce mecanisme est hors scope du stub actuel.
func (p *TelegramBot) GetDeliveryStatus(_ context.Context, externalID string) (string, error) {
	if externalID == "" {
		return "", fmt.Errorf("telegram_bot: externalId vide: %w", errProviderUnavailable)
	}
	// TODO(production): Telegram ne supporte pas le DLR en pull. On retourne "sent"
	// (message transmis a Telegram) car c'est le dernier etat connu avec certitude.
	log.Printf("[telegram_bot:stub] GetDeliveryStatus externalId=%s → sent", externalID)
	return "sent", nil
}

// MapTelegramResponseStatus mappe une reponse de l'API Telegram Bot vers un statut normalise.
//
// Cette fonction pure documente le mapping prevu pour la production et est testee
// independamment du stub runtime. Elle sera appelee par Send() en production pour
// interpreter la reponse HTTP de sendMessage.
//
// Conventions de mapping :
//   - ok=true (reponse nominale Telegram) → "sent" (message transmis au serveur)
//   - error_code 400 (Bad Request : chat_id invalide, texte vide, etc.) → "rejected"
//   - error_code 403 (Forbidden : bot bloque par l'utilisateur, chat inexistant) → "rejected"
//   - error_code 5xx (erreur serveur Telegram) → "failed"
//   - timeout ou erreur reseau (code -1 par convention) → "failed"
//
// Parametres :
//   - ok : champ "ok" de la reponse JSON Telegram (true = succes).
//   - errorCode : champ "error_code" en cas d'erreur (0 si ok=true, -1 pour timeout/reseau).
func MapTelegramResponseStatus(ok bool, errorCode int) string {
	if ok {
		return "sent"
	}
	switch {
	case errorCode == 400:
		// Bad Request : parametres invalides, message refuse par Telegram.
		return "rejected"
	case errorCode == 403:
		// Forbidden : bot bloque, chat inaccessible, utilisateur a supprime la conversation.
		return "rejected"
	case errorCode >= 500:
		// Erreur serveur Telegram (5xx) : echec technique temporaire.
		return "failed"
	default:
		// Timeout reseau (code -1) ou code inconnu : echec non categorise.
		return "failed"
	}
}
