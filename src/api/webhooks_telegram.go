package api

// webhooks_telegram.go — POST /webhooks/telegram (DLR Telegram)
//
// Ce handler recoit les "updates" Telegram envoyees par l'API Bot Telegram
// lorsque l'URL de webhook est configuree via setWebhook.
//
// ═══════════════════════════════════════════════════════════════════════════
// B1 (BLOQUANT, Phase 3, revue d'architecture) — UN SEUL ECRIVAIN DU STATUT
// ═══════════════════════════════════════════════════════════════════════════
//
// AVANT ce correctif, ce handler ECRIVAIT directement messaging.messages.status
// (UPDATE ... RETURNING id) PUIS publiait l'evenement AMQP consomme par
// src/core-processor. Or core-processor applique lui-meme une garde de statut
// ("WHERE status = 'sent'"/"WHERE status NOT IN ('delivered','failed','rejected')")
// avant de rembourser le wallet en cas d'echec — garde qui suppose que le
// statut est ENCORE dans son etat pre-DLR au moment ou core-processor la lit.
// Puisque CE handler posait deja le statut TERMINAL avant de publier,
// core-processor voyait TOUJOURS 0 ligne affectee (branche "deja traite"),
// COMMIT sans remboursement, pour 100% des evenements nominaux. Les deux
// seules responsabilites de core-processor (transition + remboursement)
// etaient structurellement inatteignables. Invisible avec Telegram (gratuit,
// cost=0 — rien a rembourser), CE BUG PERD SILENCIEUSEMENT TOUT REMBOURSEMENT
// D'ECHEC des qu'un provider payant partage le meme patron de webhook.
//
// CORRECTIF : ce handler ne fait plus JAMAIS d'UPDATE sur messaging.messages —
// UNIQUEMENT des SELECT (lecture seule) pour RESOUDRE l'UUID Fleece a publier
// dans l'evenement AMQP. core-processor (src/core-processor/on_message_delivered.go/
// on_message_failed.go) redevient l'UNIQUE ecrivain de messaging.messages.status,
// et ses gardes de statut redeviennent effectives (elles n'ont pas ete
// modifiees par ce correctif — seul CE fichier change).
//
// CONSEQUENCE ASSUMEE ET VOULUE : sans worker core-processor deployé et
// consommant la queue, le statut n'est PLUS mis a jour par le webhook seul
// (contrairement au comportement — bugue — d'avant ce correctif). C'est
// voulu : les workers font partie integrante du deploiement cible (cf.
// Phase 4/M-024 devops, topologie RabbitMQ). Un webhook qui ecrirait "pour de
// vrai" sans le worker donnerait une illusion de fonctionnement correct qui
// masquerait justement l'absence de remboursement — precisement le bug ici
// corrige.
//
// ═══════════════════════════════════════════════════════════════════════════
// B2 (BLOQUANT, Phase 3) — recipient N'EST PAS TOUJOURS le chat_id Telegram
// ═══════════════════════════════════════════════════════════════════════════
//
// L'ancien arbitrage D-M13 ("recipient == chat_id, donc le predicat
// AND recipient = $N est une correlation dure valide") etait FAUX dans le cas
// general. Deux chemins reels le violent :
//
//   - SelectProvider (routing.go) est AGNOSTIQUE DU FORMAT du destinataire —
//     rien ne filtre les providers par compatibilite de format. Un
//     POST /messages {"recipient":"+237690000000"} peut tres bien selectionner
//     "telegram-bot" (ex. strategie highest_delivery) alors que le recipient
//     est un numero E.164, PAS un chat_id Telegram. C'est le cas NORMAL des
//     campagnes (src/intelligence-processor/on_campaign_run.go poste des
//     numeros E.164 importes, pas des chat_id).
//   - providers/telegram.go documente (avant ce correctif, a tort) que `to`
//     "EST" le chat_id — alors que Telegram autorise aussi un `@username`
//     comme destinataire, jamais egal a un chat_id entier.
//
// Un predicat DUR "AND recipient = $N" RETRECIT la correlation par rapport a
// la Phase 2 (ou (external_id, provider_id) suffisait) : c'est une
// REGRESSION FONCTIONNELLE, pas un durcissement neutre — un DLR Telegram
// nominal, sur un message envoye a un numero E.164 via ce provider, ne
// correlerait plus JAMAIS avec recipient=chat_id en dur.
//
// CORRECTIF : recipient/chat_id redevient un FILTRE OPPORTUNISTE, jamais une
// condition dure :
//  1. Tenter d'abord la correlation STRICTE (external_id, provider_id,
//     recipient=chat_id) — reste le chemin ideal quand elle matche (anti-
//     collision inter-chats, l'objectif initial de D-M13).
//  2. Si 0 ligne, RETENTER (external_id, provider_id) SEUL (repli — c'etait le
//     comportement de la Phase 2, avant D-M13).
//  3. Si le repli aboutit LA OU la strategie stricte a echoue : c'est le
//     signal d'une desynchronisation reelle entre le recipient stocke et le
//     chat_id du callback — log ERROR distinctif ("dlr_recipient_mismatch"),
//     PAS noye dans le bruit WARN habituel, avec les deux valeurs (chat_id
//     attendu vs recipient effectivement trouve en base).
//
// DOCUMENTATION CORRIGEE (D-M13) : l'affirmation "recipient EST le chat_id"
// (providers/telegram.go, ce fichier avant correctif) est FAUSSE en general —
// ce n'est vrai QUE si le client HTTP a fourni un chat_id numerique en
// `recipient` ET que le routage a effectivement choisi telegram-bot pour ce
// message. Rien dans le code actuel ne garantit cette coincidence. La vraie
// correction (capturer le chat_id renvoye par sendMessage dans ProviderResult
// et le persister dans une colonne DEDIEE, distincte de recipient) est la
// dette D-M27 (liee a D-M08, remplacement du stub Telegram) — HORS PERIMETRE
// ici (aucune migration ajoutee par ce correctif).
//
// ═══════════════════════════════════════════════════════════════════════════
// CORRELATION DLR — MECANIQUE DETAILLEE
// ═══════════════════════════════════════════════════════════════════════════
//
//   messaging.messages porte les colonnes provider_id/external_id/cost
//   (migration 0018, voir messages_send.go/insertMessage). La correlation d'un
//   DLR Telegram utilise donc en priorite external_id :
//
//   CONTRAT external_id (partage avec providers/telegram.go) : external_id est
//   TOUJOURS la representation decimale d'un entier — le message_id Telegram —
//   cote provider (Send()) comme cote webhook (update.message.message_id) ; ne
//   jamais y introduire de prefixe ou de format oppose sous peine de casser
//   silencieusement toute correlation DLR.
//
//   Requete stricte (chat.id present dans l'update Telegram — toujours le cas
//   pour un vrai Message, cf. https://core.telegram.org/bots/api#message) :
//     SELECT id, recipient FROM messaging.messages
//      WHERE external_id = $1 AND provider_id = $2 AND recipient = $3
//   ("telegram-bot" = cle canonique du registry, voir providers/registry.go.)
//
//   Repli (0 ligne sur la requete stricte, OU chat.id absent/nul sur ce
//   callback — improbable pour un vrai callback Telegram mais on ne doit ni
//   planter ni elargir silencieusement la correlation sans log) :
//     SELECT id, recipient FROM messaging.messages
//      WHERE external_id = $1 AND provider_id = $2
//   Si CE repli aboutit APRES l'echec d'une correlation stricte (chat.id
//   present mais ne matchant aucune ligne) : log ERROR "dlr_recipient_mismatch"
//   (voir B2 ci-dessus). Si le repli est tente parce que chat.id etait
//   simplement absent/nul (jamais tente strictement) : log WARN habituel
//   (risque de collision inter-chats, pas une desynchronisation constatee).
//
//   Chemin de compatibilite conserve : si le payload porte encore un champ non
//   standard "fleece_message_id" (UUID) ET qu'aucun message_id Telegram
//   exploitable n'est present, on retombe sur l'ancien mecanisme :
//     SELECT id FROM messaging.messages WHERE id = $1
//   Ce chemin est herite de l'epoque ou external_id n'existait pas encore ; il
//   ne devrait plus etre exerce par de vrais callbacks Telegram (qui
//   fournissent toujours un message_id), mais reste supporte pour ne pas
//   casser d'eventuels producteurs de test/replay qui l'utiliseraient encore.
//
//   La strategie de correlation (quelle colonne / quelles valeurs utiliser,
//   AVANT tout acces DB) est decidee par la fonction pure
//   resolveTelegramCorrelation, testable sans acces DB. La RESOLUTION EFFECTIVE
//   (SELECT, repli, log de desynchronisation) est faite par
//   resolveTelegramMessageID (et ses variantes par cas), qui EST couplee a s.DB.
//
//   Les index sur external_id / (provider_id, external_id) (migration 0018)
//   sont non-UNIQUE et partiels : si plusieurs lignes matchent (collision
//   improbable mais possible, surtout sur le chemin de repli sans chat_id),
//   on utilise la premiere et on logge un WARN pour observabilite ; jamais
//   d'echec du webhook pour cette raison.
//
// CONTRAT DE L'EVENEMENT AMQP "message.delivered" / "message.failed" (D-M21,
// PRODUCTEUR — voir aussi src/core-processor/consumer.go pour le contrat cote
// CONSOMMATEUR, qui doit rester rigoureusement identique) :
//
//   {
//     "message_id":  "<uuid Fleece — messaging.messages.id>",
//     "external_id": "<identifiant provider, ex. message_id Telegram>",
//     "provider_id": "telegram-bot",
//     "status":      "delivered|failed|...",
//     "source":      "telegram"
//   }
//
//   Regles strictes (D-M21) :
//     - message_id est TOUJOURS l'UUID Fleece (messaging.messages.id), jamais
//       l'identifiant provider (external_id) ni une valeur ambigue.
//     - Si aucune ligne messaging.messages n'a pu etre correlee (SELECT sans
//       resultat, ou erreur technique) : on NE PUBLIE PAS d'evenement. Un
//       evenement non correlable ne peut etre traite par aucun consumer (il ne
//       reference rien) et n'a aucune valeur d'audit ; il ne fait que polluer
//       la queue. On se contente donc de logger un WARN/ERROR et de sortir.
//
// PUBLICATION AMQP :
//   Si (et seulement si) une correlation a pu etre etablie par SELECT (au
//   moins une ligne trouvee), on publie "message.delivered" ou "message.failed"
//   selon le statut Telegram — le statut CIBLE (fleeceStatus) est INCLUS dans
//   l'evenement pour que core-processor sache quoi ecrire, mais ce handler ne
//   l'ecrit JAMAIS lui-meme (B1). Garde nil sur s.AMQP.
//
//   OBSERVABILITE (B1, point 4) : si la publication AMQP echoue APRES une
//   correlation reussie, le DLR est DEFINITIVEMENT PERDU (aucun retry, aucune
//   trace autre que ce log) — logge en ERROR (pas WARN) avec un champ
//   distinctif ("dlr_publish_lost").
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

// telegramChat represente le champ standard Telegram "chat" d'un Message
// (https://core.telegram.org/bots/api#message — toujours present sur un vrai
// Message). Sous-ensemble minimal : seul l'identifiant est necessaire a la
// correlation DLR.
type telegramChat struct {
	// ID est le chat_id Telegram de ce callback. NE PAS CONFONDRE avec
	// messaging.messages.recipient (voir B2, doc de tete de fichier) : rien
	// ne garantit qu'ils coincident en general — seulement si le routage a
	// choisi telegram-bot ET que le client a fourni un chat_id numerique en
	// recipient. Utilise en repli/filtre opportuniste, jamais en condition dure.
	ID int64 `json:"id"`
}

// telegramMessage represente un message Telegram (sous-ensemble minimal).
type telegramMessage struct {
	// MessageID est l'identifiant Telegram du message (entier), unique PAR
	// CHAT seulement — voir Chat ci-dessous pour la seconde moitie de la clef
	// de correlation stricte. DIFFERENT de messaging.messages.id (UUID Fleece).
	MessageID int64 `json:"message_id"`
	// Chat est le chat d'origine du message. Toujours present sur un vrai
	// callback Telegram (champ standard de l'API Bot) ; peut etre nil dans ce
	// code si le producteur de l'update ne le fournit pas (payload de test/
	// replay non standard) — voir resolveTelegramCorrelation.
	Chat *telegramChat `json:"chat,omitempty"`
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
//  3. Tenter la correlation avec messaging.messages (external_id+recipient en
//     priorite, fleece_message_id en compatibilite — voir
//     resolveTelegramCorrelation).
//  4. Si correle et DLR reconnaissable : SELECT (LECTURE SEULE, B1) pour
//     confirmer la correlation et recuperer l'UUID Fleece a publier — ce
//     handler N'ECRIT JAMAIS messaging.messages.status (proprietaire exclusif :
//     src/core-processor).
//  5. Publier evenement AMQP "message.delivered" ou "message.failed"
//     UNIQUEMENT si la correlation a ete confirmee par le SELECT (D-M21).
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
// Telegram donne. Column est le nom de la colonne pivot de messaging.messages
// a filtrer ("external_id" ou "id"), vide si aucune correlation n'est possible.
type telegramCorrelation struct {
	Column string
	Value  string
	// ChatID est le chat_id Telegram du callback (candidat de filtre
	// opportuniste — B2, jamais une condition dure), extrait de msg.Chat.ID si
	// present et non nul. Uniquement pertinent quand Column == "external_id".
	// Vide si absent/nul : resolveTelegramMessageID retombe alors directement
	// sur (external_id, provider_id) seul, avec un log WARN explicite (risque
	// de collision inter-chats).
	ChatID string
}

// resolveTelegramCorrelation decide, a partir du seul contenu du message
// Telegram (aucun acces DB), quelle colonne/valeurs utiliser pour retrouver
// le message Fleece correspondant :
//
//   - message_id present (cas normal : l'API Telegram fournit toujours cet
//     identifiant entier) → correlation par external_id (chemin principal,
//     migration 0018), avec chat_id comme filtre opportuniste si disponible
//     (B2 : PAS une condition dure — voir resolveTelegramMessageID pour le
//     repli). C'est la meme valeur que providers.TelegramBot.Send() a stockee
//     dans messaging.messages.external_id lors de l'envoi.
//   - sinon, si fleece_message_id (UUID) est fourni et syntaxiquement valide
//     → correlation par id (chemin de compatibilite herite de l'ancien
//     workaround, conserve pour ne pas casser d'eventuels rejeux existants).
//   - sinon → aucune correlation possible.
//
// Fonction pure, testable sans base de donnees. INCHANGEE par le correctif
// B1/B2 (Phase 3) : c'est resolveTelegramMessageID, qui EST couplee a s.DB,
// qui applique desormais la strategie strict-puis-repli (B2) et ne fait plus
// jamais d'UPDATE (B1).
func resolveTelegramCorrelation(msg *telegramMessage) telegramCorrelation {
	var chatID string
	if msg.Chat != nil && msg.Chat.ID != 0 {
		chatID = strconv.FormatInt(msg.Chat.ID, 10)
	}

	if msg.MessageID != 0 {
		return telegramCorrelation{
			Column: "external_id",
			Value:  strconv.FormatInt(msg.MessageID, 10),
			ChatID: chatID,
		}
	}
	if msg.FleeceMsgID != "" {
		if _, err := parseUUID(msg.FleeceMsgID); err == nil {
			return telegramCorrelation{Column: "id", Value: msg.FleeceMsgID}
		}
	}
	return telegramCorrelation{}
}

// telegramCorrelatedRow scanne le resultat d'un SELECT de resolution de
// correlation. Recipient n'est peuple (et pertinent) que sur les chemins
// external_id (comparaison avec le chat_id attendu, B2) — laisse a la valeur
// zero (non scannee) sur le chemin de compatibilite "id" (SELECT id seul).
type telegramCorrelatedRow struct {
	ID        string `db:"id"`
	Recipient string `db:"recipient"`
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
	// Statut Fleece cible — INCLUS dans l'evenement AMQP pour core-processor,
	// mais JAMAIS ecrit ici (B1 : ce handler ne fait que lire).
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
		// D-M21 : aucune correlation -> aucune publication.
		return
	}

	// 4. SELECT (lecture seule, B1) pour confirmer la correlation et recuperer
	// l'UUID Fleece a publier. AUCUN UPDATE messaging.messages ici — voir doc
	// de tete de fichier (B1).
	messageID := s.resolveTelegramMessageID(ctx, corr)
	if messageID == "" {
		// Deja logge (WARN/ERROR selon le cas) par resolveTelegramMessageID.
		return
	}

	s.Logger.Info("webhooks/telegram: message correle (statut sera applique par core-processor)",
		"correlation_column", corr.Column, "correlation_value", corr.Value,
		"message_id", messageID, "status", fleeceStatus)

	// external_id n'est pertinent a inclure dans l'evenement que sur le chemin
	// principal (compat "id" : on ne connait pas l'external_id du message).
	var externalID string
	if corr.Column == "external_id" {
		externalID = corr.Value
	}

	// 5. Publier evenement AMQP — message_id est TOUJOURS l'UUID Fleece
	// confirme par le SELECT (D-M21). status est le statut CIBLE (fleeceStatus) :
	// core-processor l'ecrit lui-meme, sous sa propre garde de transition.
	s.publishTelegramDLR(ctx, messageID, externalID, telegramProviderID, fleeceStatus, routingKey)
}

// resolveTelegramMessageID resout l'UUID Fleece (messaging.messages.id) a
// partir de la strategie de correlation decidee par resolveTelegramCorrelation.
// LECTURE SEULE UNIQUEMENT (B1) : aucun UPDATE n'est jamais emis ici. Retourne
// "" si aucune ligne n'a pu etre correlee (deja logge par la fonction).
func (s *Service) resolveTelegramMessageID(ctx context.Context, corr telegramCorrelation) string {
	switch {
	case corr.Column == "external_id" && corr.ChatID != "":
		return s.resolveTelegramMessageIDWithChatID(ctx, corr)
	case corr.Column == "external_id":
		return s.resolveTelegramMessageIDNoChatID(ctx, corr)
	default: // corr.Column == "id" (chemin de compatibilite fleece_message_id)
		return s.resolveTelegramMessageIDCompat(ctx, corr)
	}
}

// resolveTelegramMessageIDWithChatID implemente la strategie B2 : correlation
// STRICTE (external_id, provider_id, recipient=chat_id) en premier ; si 0
// ligne, REPLI sur (external_id, provider_id) SEUL. Si le repli aboutit LA OU
// la strategie stricte a echoue, c'est un signal de desynchronisation reelle
// (recipient stocke != chat_id du callback) — log ERROR distinctif
// "dlr_recipient_mismatch" (pas noye dans le bruit WARN habituel), avec les
// deux valeurs.
func (s *Service) resolveTelegramMessageIDWithChatID(ctx context.Context, corr telegramCorrelation) string {
	const strictQuery = `SELECT id, recipient FROM messaging.messages
	                       WHERE external_id = $1 AND provider_id = $2 AND recipient = $3`
	var strictRows []telegramCorrelatedRow
	if err := s.DB.Select(ctx, &strictRows, strictQuery, corr.Value, telegramProviderID, corr.ChatID); err != nil {
		s.Logger.Warn("webhooks/telegram: lecture messaging.messages (correlation stricte) echouee",
			"correlation_column", corr.Column, "correlation_value", corr.Value, "err", err)
		return ""
	}
	if len(strictRows) > 0 {
		if len(strictRows) > 1 {
			s.Logger.Warn("webhooks/telegram: plusieurs messages correles (correlation stricte, index non-unique)",
				"correlation_value", corr.Value, "chat_id", corr.ChatID, "rows", len(strictRows))
		}
		return strictRows[0].ID
	}

	// B2 : repli (external_id, provider_id) seul — recipient n'est PAS
	// toujours le chat_id (voir doc de tete de fichier).
	const fallbackQuery = `SELECT id, recipient FROM messaging.messages
	                         WHERE external_id = $1 AND provider_id = $2`
	var fallbackRows []telegramCorrelatedRow
	if err := s.DB.Select(ctx, &fallbackRows, fallbackQuery, corr.Value, telegramProviderID); err != nil {
		s.Logger.Warn("webhooks/telegram: lecture messaging.messages (repli B2) echouee",
			"correlation_column", corr.Column, "correlation_value", corr.Value, "err", err)
		return ""
	}
	if len(fallbackRows) == 0 {
		s.Logger.Warn("webhooks/telegram: aucun message correle (ni correlation stricte ni repli)",
			"correlation_column", corr.Column, "correlation_value", corr.Value, "chat_id", corr.ChatID)
		return ""
	}
	if len(fallbackRows) > 1 {
		s.Logger.Warn("webhooks/telegram: plusieurs messages correles (repli B2, index non-unique)",
			"correlation_value", corr.Value, "rows", len(fallbackRows))
	}

	// Le repli a reussi LA OU la correlation stricte (avec recipient=chat_id)
	// a echoue : desynchronisation reelle entre le recipient stocke et le
	// chat_id du callback — jamais du bruit, toujours actionnable. ERROR
	// (pas WARN), champ distinctif "dlr_recipient_mismatch" (B2, point 3).
	s.Logger.Error("webhooks/telegram: dlr_recipient_mismatch — repli (external_id, provider_id) reussi la ou la correlation stricte (recipient=chat_id) a echoue",
		"dlr_recipient_mismatch", true,
		"correlation_value", corr.Value,
		"chat_id_attendu", corr.ChatID,
		"recipient_en_base", fallbackRows[0].Recipient,
		"message_id", fallbackRows[0].ID,
	)
	return fallbackRows[0].ID
}

// resolveTelegramMessageIDNoChatID gere le cas ou chat.id est absent/nul sur
// ce callback (improbable pour un vrai callback Telegram) : correlation par
// (external_id, provider_id) SEUL directement — aucune tentative stricte
// n'est possible faute de chat_id, donc pas de log "mismatch" (ce n'est pas
// une desynchronisation constatee, juste l'absence d'un filtre optionnel) —
// seul le WARN "risque de collision inter-chats" deja documente s'applique.
func (s *Service) resolveTelegramMessageIDNoChatID(ctx context.Context, corr telegramCorrelation) string {
	s.Logger.Warn("webhooks/telegram: chat.id absent/nul — correlation (external_id, provider_id) seule, risque de collision inter-chats (D-M13)",
		"telegram_message_id_correlation_value", corr.Value)

	const query = `SELECT id, recipient FROM messaging.messages WHERE external_id = $1 AND provider_id = $2`
	var rows []telegramCorrelatedRow
	if err := s.DB.Select(ctx, &rows, query, corr.Value, telegramProviderID); err != nil {
		s.Logger.Warn("webhooks/telegram: lecture messaging.messages echouee",
			"correlation_column", corr.Column, "correlation_value", corr.Value, "err", err)
		return ""
	}
	if len(rows) == 0 {
		s.Logger.Warn("webhooks/telegram: aucun message correle",
			"correlation_column", corr.Column, "correlation_value", corr.Value)
		return ""
	}
	if len(rows) > 1 {
		s.Logger.Warn("webhooks/telegram: plusieurs messages correles pour la meme valeur (index non-unique)",
			"correlation_column", corr.Column, "correlation_value", corr.Value, "rows", len(rows))
	}
	return rows[0].ID
}

// resolveTelegramMessageIDCompat implemente le chemin de compatibilite
// fleece_message_id (herite de l'ancien workaround pre-migration 0018) :
// SELECT id FROM messaging.messages WHERE id = $1 — confirme simplement
// l'existence de la ligne (id est la cle primaire, au plus 1 resultat).
func (s *Service) resolveTelegramMessageIDCompat(ctx context.Context, corr telegramCorrelation) string {
	const query = `SELECT id FROM messaging.messages WHERE id = $1`
	var rows []telegramCorrelatedRow
	if err := s.DB.Select(ctx, &rows, query, corr.Value); err != nil {
		s.Logger.Warn("webhooks/telegram: lecture messaging.messages (compat fleece_message_id) echouee",
			"correlation_column", corr.Column, "correlation_value", corr.Value, "err", err)
		return ""
	}
	if len(rows) == 0 {
		s.Logger.Warn("webhooks/telegram: aucun message correle (compat fleece_message_id)",
			"correlation_column", corr.Column, "correlation_value", corr.Value)
		return ""
	}
	return rows[0].ID
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
//
// CONTRAT D-M21 (voir commentaire de tete de fichier) : messageID DOIT
// toujours etre non-vide et etre l'UUID Fleece (messaging.messages.id) —
// l'appelant (processTelegramDLR) ne l'invoque que lorsque le SELECT a
// confirme au moins une ligne (B1 : jamais un UPDATE...RETURNING desormais).
// externalID/providerID sont des champs d'enrichissement (evitent un aller-
// retour DB au worker src/core-processor) mais ne remplacent jamais message_id
// comme clef. Garde nil sur s.AMQP (meme pattern que publishMessageSent).
func (s *Service) publishTelegramDLR(ctx context.Context, messageID, externalID, providerID, status, routingKey string) {
	if s.AMQP == nil {
		return
	}

	event := struct {
		MessageID  string `json:"message_id"`
		ExternalID string `json:"external_id,omitempty"`
		ProviderID string `json:"provider_id,omitempty"`
		Status     string `json:"status"`
		Source     string `json:"source"`
	}{
		MessageID:  messageID,
		ExternalID: externalID,
		ProviderID: providerID,
		Status:     status,
		Source:     "telegram",
	}

	body, err := json.Marshal(event)
	if err != nil {
		s.Logger.Warn("webhooks/telegram: marshal amqp event", "err", err)
		return
	}

	// B1, point 4 : si la publication echoue APRES une correlation reussie, le
	// DLR est DEFINITIVEMENT PERDU (aucun retry possible depuis ce handler,
	// aucune autre trace) — ERROR (pas WARN), champ distinctif "dlr_publish_lost".
	if err := s.AMQP.Publish(ctx, "fleece", routingKey, body); err != nil {
		s.Logger.Error("webhooks/telegram: publication AMQP echouee — DLR definitivement perdu",
			"dlr_publish_lost", true,
			"routing_key", routingKey, "message_id", messageID, "err", err)
	}
}
