package api

// webhooks_telegram.go — POST /webhooks/telegram (DLR Telegram)
//
// Ce handler recoit les "updates" Telegram envoyees par l'API Bot Telegram
// lorsque l'URL de webhook est configuree via setWebhook.
//
// CORRELATION DLR (M-018, dette soldee par la migration additive
// 0018_messaging_dlr_correlation) :
//   messaging.messages porte desormais les colonnes provider_id/external_id/cost
//   (voir messages_send.go/insertMessage). La correlation d'un DLR Telegram
//   utilise donc en priorite external_id :
//
//   CONTRAT external_id (D-M13, partage avec providers/telegram.go) : external_id
//   est TOUJOURS la representation decimale d'un entier — le message_id Telegram
//   — cote provider (Send()) comme cote webhook (update.message.message_id) ; ne
//   jamais y introduire de prefixe ou de format oppose sous peine de casser
//   silencieusement toute correlation DLR (voir dette D-M13, .ia/MEMORY.md).
//     UPDATE messaging.messages SET status=$1
//      WHERE external_id=$2 AND provider_id='telegram-bot'
//   ("telegram-bot" = cle canonique du registry, voir providers/registry.go).
//   external_id est renseigne par providers.TelegramBot.Send() a partir du
//   message_id retourne par l'API Telegram (voir providers/telegram.go) ; le
//   webhook recoit ce meme message_id dans update.message.message_id, converti
//   en string pour la comparaison.
//
//   Chemin de compatibilite conserve : si le payload porte encore un champ non
//   standard "fleece_message_id" (UUID) ET qu'aucun message_id Telegram
//   exploitable n'est present, on retombe sur l'ancien mecanisme :
//     UPDATE messaging.messages SET status=$1 WHERE id=$2
//   Ce chemin est herite de l'epoque ou external_id n'existait pas encore ;
//   il ne devrait plus etre exerce par de vrais callbacks Telegram (qui
//   fournissent toujours un message_id), mais reste supporte pour ne pas casser
//   d'eventuels producteurs de test/replay qui l'utiliseraient encore.
//
//   La strategie de correlation (quelle colonne utiliser, a partir de quelles
//   valeurs) est decidee par la fonction pure resolveTelegramCorrelation,
//   testable sans acces DB.
//
//   L'index sur external_id (migration 0018) est non-UNIQUE et partiel : si
//   plusieurs lignes matchent (collision improbable mais possible), l'UPDATE
//   les affecte toutes sans erreur ; on logge un WARN pour observabilite mais
//   on ne fait jamais echouer le webhook pour cette raison.
//
// PUBLICATION AMQP :
//   Si une correlation a pu etre etablie, on publie "message.delivered" ou
//   "message.failed" selon le statut Telegram. Garde nil sur s.AMQP.
//
// Telegram exige toujours 200 :
//   Si Telegram recoit autre chose que 200, il reessaie l'update indefiniment
//   (jusqu'a 24h). On renvoie donc 200 dans TOUS les cas, meme si le payload
//   n'est pas reconnu ou si la DB echoue.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

// telegramProviderID est la cle canonique du provider Telegram dans le
// registry (voir providers/registry.go, BuildRegistry). Utilisee pour
// restreindre la correlation external_id au bon provider (l'index composite
// (provider_id, external_id) de la migration 0018 est concu pour ce filtre).
const telegramProviderID = "telegram-bot"

// telegramUpdate represente une update Telegram (sous-ensemble minimal).
// Le format complet de l'API Telegram Bot est documente sur
// https://core.telegram.org/bots/api#update.
// TODO(production): enrichir cette struct avec Message, CallbackQuery, etc.
type telegramUpdate struct {
	// UpdateID est l'identifiant unique de l'update (entier incrementiel Telegram).
	UpdateID int64 `json:"update_id"`
	// Message est present si l'update est un message entrant.
	// Nil si c'est un autre type d'update (callback_query, inline_query, etc.).
	Message *telegramMessage `json:"message,omitempty"`
}

// telegramMessage represente un message Telegram (sous-ensemble minimal).
type telegramMessage struct {
	// MessageID est l'identifiant Telegram du message (entier).
	// DIFFERENT de messaging.messages.id (UUID Fleece).
	MessageID int64 `json:"message_id"`
	// Text est le contenu textuel du message (optionnel).
	Text string `json:"text,omitempty"`
	// FleeceMsgID est un champ NON STANDARD, herite de l'ancien workaround
	// (avant migration 0018) : si le payload Telegram porte ce champ (UUID),
	// il permet la correlation avec messaging.messages.id. Conserve en chemin
	// de compatibilite (voir resolveTelegramCorrelation) mais n'existe pas
	// dans le protocole Telegram standard — external_id (derive de MessageID)
	// est desormais le chemin principal.
	FleeceMsgID string `json:"fleece_message_id,omitempty"`
	// DeliveryStatus est un champ NON STANDARD indiquant le statut de livraison.
	// Valeurs attendues : "delivered", "failed", "read".
	// TODO(production): Telegram ne fournit pas de DLR standard pour les bots ;
	// ce champ serait a injecter par un mecanisme custom (ex. Webhook avec meta).
	DeliveryStatus string `json:"delivery_status,omitempty"`
}

// HandleWebhookTelegram traite POST /webhooks/telegram.
//
// Pipeline :
//  1. Lire le body complet.
//  2. Parser la TelegramUpdate.
//  3. Tenter la correlation avec messaging.messages (external_id en priorite,
//     fleece_message_id en compatibilite — voir resolveTelegramCorrelation).
//  4. Si correle et DLR reconnaissable : UPDATE messaging.messages.status.
//  5. Publier evenement AMQP "message.delivered" ou "message.failed" si correle.
//  6. Repondre 200 TOUJOURS (meme si non reconnu ou echec DB).
func (s *Service) HandleWebhookTelegram(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Lire le body.
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		s.Logger.Warn("webhooks/telegram: lecture body echouee", "err", err)
		// 200 meme en cas d'erreur de lecture — Telegram ne doit pas re-essayer.
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Parser l'update Telegram.
	var update telegramUpdate
	if err := json.Unmarshal(rawBody, &update); err != nil {
		s.Logger.Warn("webhooks/telegram: payload JSON invalide",
			"err", err, "raw", string(rawBody))
		// 200 meme sur JSON invalide.
		w.WriteHeader(http.StatusOK)
		return
	}

	s.Logger.Info("webhooks/telegram: update recue", "update_id", update.UpdateID)

	// 3. Tenter la correlation et le traitement DLR.
	if update.Message != nil {
		s.processTelegramDLR(ctx, update)
	} else {
		// Update d'un type non reconnu (callback_query, inline, etc.).
		// Log et ignoré — Telegram attend 200.
		s.Logger.Info("webhooks/telegram: update non-message ignoree",
			"update_id", update.UpdateID)
	}

	// 6. Repondre 200 TOUJOURS.
	w.WriteHeader(http.StatusOK)
}

// telegramCorrelation decrit la strategie de correlation choisie pour un DLR
// Telegram donne. Column est le nom de la colonne messaging.messages a filtrer
// ("external_id" ou "id"), vide si aucune correlation n'est possible.
type telegramCorrelation struct {
	Column string
	Value  string
}

// resolveTelegramCorrelation decide, a partir du seul contenu du message
// Telegram (aucun acces DB), quelle colonne utiliser pour retrouver le message
// Fleece correspondant :
//
//   - message_id present (cas normal : l'API Telegram fournit toujours cet
//     identifiant entier) → correlation par external_id (chemin principal,
//     migration 0018). C'est la meme valeur que providers.TelegramBot.Send()
//     a stockee dans messaging.messages.external_id lors de l'envoi.
//   - sinon, si fleece_message_id (UUID) est fourni et syntaxiquement valide
//     → correlation par id (chemin de compatibilite herite de l'ancien
//     workaround, conserve pour ne pas casser d'eventuels rejeux existants).
//   - sinon → aucune correlation possible.
//
// Fonction pure, testable sans base de donnees.
func resolveTelegramCorrelation(msg *telegramMessage) telegramCorrelation {
	if msg.MessageID != 0 {
		return telegramCorrelation{Column: "external_id", Value: strconv.FormatInt(msg.MessageID, 10)}
	}
	if msg.FleeceMsgID != "" {
		if _, err := parseUUID(msg.FleeceMsgID); err == nil {
			return telegramCorrelation{Column: "id", Value: msg.FleeceMsgID}
		}
	}
	return telegramCorrelation{}
}

// processTelegramDLR traite le volet DLR d'une update Telegram portant un message.
// Cette fonction ne retourne PAS d'erreur : en cas d'echec, elle log et continue.
// Telegram ne doit jamais recevoir autre chose que 200.
func (s *Service) processTelegramDLR(ctx context.Context, update telegramUpdate) {
	msg := update.Message

	// Verifier si c'est un DLR reconnaissable.
	// Critere : presence d'un delivery_status non vide.
	if msg.DeliveryStatus == "" {
		// Pas un DLR — message entrant ordinaire ou update de statut non reconnue.
		// TODO(production): si on veut traiter les messages entrants (chatbot),
		// c'est ici qu'on deleguerait vers un handler de conversation.
		s.Logger.Info("webhooks/telegram: message sans delivery_status — ignore",
			"telegram_message_id", msg.MessageID)
		return
	}

	// Determiner le routing key AMQP selon le statut Telegram.
	routingKey := mapTelegramStatusToRoutingKey(msg.DeliveryStatus)
	// Statut Fleece correspondant.
	fleeceStatus := mapTelegramStatusToFleeceStatus(msg.DeliveryStatus)

	// 3. Decider la strategie de correlation (fonction pure, sans I/O).
	corr := resolveTelegramCorrelation(msg)

	if corr.Column == "" {
		// Ni external_id (message_id absent — improbable pour un vrai callback
		// Telegram) ni fleece_message_id valide (chemin de compatibilite) : pas
		// de correlation possible.
		s.Logger.Warn("webhooks/telegram: DLR sans correlation possible (ni external_id ni fleece_message_id valide)",
			"telegram_message_id", msg.MessageID,
			"delivery_status", msg.DeliveryStatus,
		)
		// Publier l'evenement AMQP meme sans correlation (audit, futur consumer).
		s.publishTelegramDLR(ctx, "", fleeceStatus, routingKey)
		return
	}

	// 4. UPDATE messaging.messages SET status=$1 WHERE <colonne choisie>=$2
	// (+ AND provider_id=$3 pour external_id, qui exploite l'index composite
	// (provider_id, external_id) de la migration 0018 et evite les collisions
	// entre providers).
	// Invariant du service (D-M01/rule 6) : s.DB est toujours injecte en
	// production (aucune garde nil ici, harmonise avec les autres handlers qui
	// derefencent s.DB directement) ; les tests httptest evitent ce chemin en
	// ne fournissant pas de message_id/fleece_message_id exploitable (voir
	// webhooks_endpoints_test.go).
	var (
		rows int64
		err  error
	)
	switch corr.Column {
	case "external_id":
		rows, err = s.DB.ExecRows(ctx,
			`UPDATE messaging.messages SET status = $1 WHERE external_id = $2 AND provider_id = $3`,
			fleeceStatus, corr.Value, telegramProviderID,
		)
	case "id":
		rows, err = s.DB.ExecRows(ctx,
			`UPDATE messaging.messages SET status = $1 WHERE id = $2`,
			fleeceStatus, corr.Value,
		)
	}

	switch {
	case err != nil:
		s.Logger.Warn("webhooks/telegram: UPDATE messaging.messages echoue",
			"correlation_column", corr.Column, "correlation_value", corr.Value, "err", err)
	case rows == 0:
		// Index partiel non-unique : 0 ligne signifie simplement qu'aucun message
		// ne correspond (callback en avance sur l'insertion, ou identifiant
		// inconnu). On log et on continue — jamais d'erreur sur ce cas.
		s.Logger.Warn("webhooks/telegram: aucun message correle",
			"correlation_column", corr.Column, "correlation_value", corr.Value)
	default:
		if rows > 1 {
			// Index non-unique : plusieurs messages ont matche la meme valeur.
			// On les a tous mis a jour (comportement SQL naturel) — log pour
			// observabilite uniquement, pas d'echec.
			s.Logger.Warn("webhooks/telegram: plusieurs messages correles pour la meme valeur (index non-unique)",
				"correlation_column", corr.Column, "correlation_value", corr.Value, "rows", rows)
		}
		s.Logger.Info("webhooks/telegram: statut mis a jour",
			"correlation_column", corr.Column, "correlation_value", corr.Value, "status", fleeceStatus)
	}

	// 5. Publier evenement AMQP.
	s.publishTelegramDLR(ctx, corr.Value, fleeceStatus, routingKey)
}

// mapTelegramStatusToRoutingKey convertit un statut Telegram DLR en routing key AMQP.
// Valeurs d'entree attendues : "delivered", "read", "failed".
// Routing keys : "message.delivered" ou "message.failed".
func mapTelegramStatusToRoutingKey(deliveryStatus string) string {
	switch deliveryStatus {
	case "delivered", "read":
		return "message.delivered"
	default:
		return "message.failed"
	}
}

// mapTelegramStatusToFleeceStatus convertit un statut Telegram DLR vers les
// valeurs reconnues par messaging.messages.status.
func mapTelegramStatusToFleeceStatus(deliveryStatus string) string {
	switch deliveryStatus {
	case "delivered", "read":
		return string(MessageDelivered)
	default:
		return string(MessageFailed)
	}
}

// publishTelegramDLR publie un evenement AMQP de livraison/echec Telegram.
// messageID peut etre vide si la correlation n'a pas pu etre etablie.
// Garde nil sur s.AMQP (meme pattern que publishMessageSent).
func (s *Service) publishTelegramDLR(ctx context.Context, messageID, status, routingKey string) {
	if s.AMQP == nil {
		return
	}

	event := struct {
		MessageID string `json:"message_id"`
		Status    string `json:"status"`
		Source    string `json:"source"`
	}{
		MessageID: messageID,
		Status:    status,
		Source:    "telegram",
	}

	body, err := json.Marshal(event)
	if err != nil {
		s.Logger.Warn("webhooks/telegram: marshal amqp event", "err", err)
		return
	}

	if err := s.AMQP.Publish(ctx, "fleece", routingKey, body); err != nil {
		s.Logger.Warn("webhooks/telegram: amqp publish", "routing_key", routingKey, "err", err)
	}
}
