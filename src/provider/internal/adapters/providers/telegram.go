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

	"fleece/src/provider/internal/application/ports/output"
	"fleece/src/provider/internal/domain"
)

// Assertion de conformite au port : ne compile pas si TelegramBot n'implemente pas output.Provider.
var _ output.Provider = (*TelegramBot)(nil)

// Capabilities du canal Telegram (constantes Go — decision D27 : pas de colonne base).
//
// Ces constantes documentent les capacites techniques de l'adapter Telegram Bot API.
// Elles sont definies en couche 3 (adapter) car elles ne font l'objet d'aucune requete
// SQL et n'appartiennent pas au domaine metier. Si un besoin dynamique emergeait en V2,
// une colonne JSONB dans provider.providers serait une migration additive triviale.
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

// TelegramBot implemente le port output.Provider pour l'API Telegram Bot.
//
// TODO(production): remplacer les stubs par de vrais appels HTTP a l'API Telegram Bot :
//   - POST https://api.telegram.org/bot{token}/sendMessage
//   - Corps JSON : { "chat_id": recipient, "text": content }
//   - Authentification : le token est inclus dans l'URL (jamais dans les logs).
//
// TODO(production) D27: le bot token est actuellement charge depuis la config (env
// TELEGRAM_BOT_TOKEN). La decision D27 prevoit a terme son stockage chiffre AES-GCM
// dans provider.provider_credentials (colonne secret_enc bytea). Le mecanisme de
// dechiffrement (cle AES-GCM injectee via Secret K8s) reste a implémenter
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
// msg.Recipient doit contenir le chat_id Telegram du destinataire (entier ou @username).
// msg.Content doit etre non vide et inferieur a TelegramMaxMessageLength caracteres.
//
// TODO(production): construire et executer la vraie requete HTTP :
//
//	POST {baseURL}/bot{botToken}/sendMessage
//	Content-Type: application/json
//	Body: { "chat_id": msg.Recipient, "text": msg.Content }
//
// L'externalId retourne sera le champ "result.message_id" (entier) de la reponse
// Telegram, convertie en string (ex. "42"). Ce message_id permet de correlater
// les DLR et de referencer le message pour les operations ulterieures.
// IMPORTANT : ne jamais inclure botToken dans les logs.
func (p *TelegramBot) Send(_ context.Context, msg domain.OutboundMessage) (string, error) {
	// Validation de base.
	if msg.Recipient == "" {
		return "", fmt.Errorf("telegram_bot: recipient vide: %w", domain.ErrSendFailed)
	}
	if msg.Content == "" {
		return "", fmt.Errorf("telegram_bot: contenu vide: %w", domain.ErrSendFailed)
	}

	// TODO(production): executer la vraie requete HTTP vers l'API Telegram.
	// Stub : genere un externalId non vide simulant un message_id Telegram.
	// Le message_id Telegram est un entier croissant par chat ; on simule avec le nanos.
	externalId := fmt.Sprintf("tg%d", time.Now().UnixNano())
	log.Printf("[telegram_bot:stub] Send internalId=%s recipient=%s externalId=%s",
		msg.Id, msg.Recipient, externalId)
	// NOTE : le botToken n'est jamais inclus dans les logs.

	return externalId, nil
}

// EstimateCost retourne le cout estime d'un message Telegram pour le pays donne.
//
// L'API Telegram Bot est entierement GRATUITE — Telegram ne facture pas les messages
// envoyes via Bot API, contrairement a WhatsApp Business ou aux SMS.
// Le cout est donc toujours 0, quelle que soit la destination.
//
// Convention de devise : EUR pour les pays europeens (FR/BE/CH), XAF par defaut
// (coherent avec les autres adapters du service).
func (p *TelegramBot) EstimateCost(_ context.Context, channel, country string) (domain.Money, error) {
	// Telegram Bot API est gratuit : montant = 0.
	// On conserve la convention de devise des autres adapters pour la coherence
	// (le routeur et la grille tarifaire peuvent filtrer par devise).
	_ = channel
	switch country {
	case "FR", "BE", "CH":
		return domain.Money{Amount: 0, Currency: "EUR"}, nil
	default:
		// Afrique et reste du monde : XAF par defaut, montant 0.
		return domain.Money{Amount: 0, Currency: "XAF"}, nil
	}
}

// GetDeliveryStatus retourne le statut de livraison d'un message Telegram.
//
// NOTE : l'API Telegram Bot ne fournit pas de mecansime de DLR (Delivery Report)
// en mode pull. Telegram confirme l'envoi via la reponse a sendMessage (champ ok=true),
// mais ne notifie pas ulterieurement si le message a ete lu ou livre. Les "read receipts"
// existent dans l'interface utilisateur Telegram mais ne sont pas accessibles via Bot API.
//
// TODO(production): en production, le statut "delivered" pourra etre approche via
// les webhooks Telegram (getUpdates ou setWebhook) si le bot recoit une confirmation,
// mais ce mecanisme est hors scope du stub actuel.
func (p *TelegramBot) GetDeliveryStatus(_ context.Context, externalId string) (domain.DeliveryStatus, error) {
	if externalId == "" {
		return "", fmt.Errorf("telegram_bot: externalId vide: %w", domain.ErrProviderUnavailable)
	}
	// TODO(production): Telegram ne supporte pas le DLR en pull. On retourne StatusSent
	// (message transmis a Telegram) car c'est le dernier etat connu avec certitude.
	log.Printf("[telegram_bot:stub] GetDeliveryStatus externalId=%s → sent", externalId)
	return domain.StatusSent, nil
}

// MapTelegramResponseStatus mappe une reponse de l'API Telegram Bot vers un DeliveryStatus.
//
// Cette fonction pure documente le mapping prevu pour la production et est testee
// independamment du stub runtime. Elle sera appelee par Send() en production pour
// interpreter la reponse HTTP de sendMessage.
//
// Conventions de mapping :
//   - ok=true (reponse nominale Telegram) → StatusSent (message transmis au serveur)
//   - error_code 400 (Bad Request : chat_id invalide, texte vide, etc.) → StatusRejected
//   - error_code 403 (Forbidden : bot bloque par l'utilisateur, chat inexistant) → StatusRejected
//   - error_code 5xx (erreur serveur Telegram) → StatusFailed
//   - timeout ou erreur reseau (code -1 par convention) → StatusFailed
//
// Parametres :
//   - ok : champ "ok" de la reponse JSON Telegram (true = succes).
//   - errorCode : champ "error_code" en cas d'erreur (0 si ok=true, -1 pour timeout/reseau).
func MapTelegramResponseStatus(ok bool, errorCode int) domain.DeliveryStatus {
	if ok {
		return domain.StatusSent
	}
	switch {
	case errorCode == 400:
		// Bad Request : parametres invalides, message refuse par Telegram.
		return domain.StatusRejected
	case errorCode == 403:
		// Forbidden : bot bloque, chat inaccessible, utilisateur a supprime la conversation.
		return domain.StatusRejected
	case errorCode >= 500:
		// Erreur serveur Telegram (5xx) : echec technique temporaire.
		return domain.StatusFailed
	default:
		// Timeout reseau (code -1) ou code inconnu : echec non categorise.
		return domain.StatusFailed
	}
}
