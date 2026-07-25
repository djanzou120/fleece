package api

// webhooks_mtn.go — POST /webhooks/mtn (callback MTN Mobile Money)
//
// Ce handler recoit les notifications de paiement de MTN MoMo. Il partage la
// logique de validation HMAC (validateHMACSignature) et de reconciliation wallet
// (reconcileWalletTransactionStatus) avec le handler OM — les deux sont definies
// dans webhooks_om.go. Seul le vocabulaire operateur differe (mapMTNStatus).
//
// RECONCILIATION WALLET (migration additive 0019_wallet_reconciliation.sql) :
//   Identique a OM (voir webhooks_om.go) : le handler execute
//     UPDATE wallet.wallet_transactions SET status = $1 WHERE reference_id = $2
//   ou $2 = payload.ReferenceID (l'identifiant de transaction cote MTN, utilise
//   comme cle de correlation — meme convention que TransactionID pour OM) et
//   $1 = mapMTNStatus(payload.Status).
//
// IMPORTANT — PAS DE CREDIT DE SOLDE ICI (D8, .ia/MEMORY.md) :
//   Voir webhooks_om.go : le crédit automatique du solde depuis un callback
//   operateur rejouable est reporte a V2 (probleme d'idempotence non trivial).
//   Ce handler se limite a la mise a jour du statut d'une transaction existante.
//
// DECISION DE VALIDATION HMAC :
//   Identique a OM (voir webhooks_om.go) :
//   - Service.MTNWebhookSecret configure : header absent → 401 ; invalide → 401 ;
//     valide → 200.
//   - Service.MTNWebhookSecret vide : callback accepte, log WARN explicite.
//
// SECRET HMAC :
//   Injecte via Service.MTNWebhookSecret (composition root cmd/api/main.go,
//   variable d'environnement MTN_WEBHOOK_SECRET). Plus de constante codee en dur.

import (
	"encoding/json"
	"io"
	"net/http"
)

// mtnPayload represente le corps JSON envoye par MTN Mobile Money lors d'un callback.
// Le format exact de l'API MTN MoMo est proprietaire et susceptible d'evoluer.
// TODO(production): affiner cette struct selon la documentation officielle MTN MoMo API.
type mtnPayload struct {
	// ReferenceID est l'identifiant de la transaction cote MTN. Utilise comme cle
	// de correlation avec wallet.wallet_transactions.reference_id (migration 0019).
	ReferenceID string `json:"reference_id"`
	// ExternalID est l'identifiant fourni par Fleece lors de l'initiation du paiement.
	ExternalID string `json:"external_id,omitempty"`
	// Status est le statut de la transaction : "SUCCESSFUL", "FAILED", "PENDING".
	// Mappe vers le vocabulaire interne via mapMTNStatus.
	Status string `json:"status"`
	// Amount est le montant en unites de la devise locale.
	Amount string `json:"amount"`
	// Currency est la devise (ex. "XAF", "EUR").
	Currency string `json:"currency"`
	// PayerMessage est le libelle du paiement.
	PayerMessage string `json:"payer_message,omitempty"`
	// MessageID est un identifiant optionnel permettant de corréler la transaction
	// avec un message Fleece (messaging.messages.id). Non exploite pour l'instant
	// (aucun flux d'envoi de message ne declenche de paiement MTN en V1) — conserve
	// pour compatibilite ascendante du payload.
	MessageID string `json:"message_id,omitempty"`
}

// HandleWebhookMTN traite POST /webhooks/mtn.
//
// Pipeline :
//  1. Lire le body complet (necessaire pour la validation HMAC).
//  2. Valider la signature HMAC-SHA256 si le header X-MTN-Signature est present.
//  3. Parser le payload JSON.
//  4. Logger le callback pour traceabilite.
//  5. Reconcilier le statut de la transaction wallet correlee (reference_id),
//     sans jamais crediter le solde (voir IMPORTANT ci-dessus).
//  6. Repondre 200 (requis par MTN MoMo pour eviter les replays).
func (s *Service) HandleWebhookMTN(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		s.Logger.Warn("webhooks/mtn: lecture body echouee", "err", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Politique de validation HMAC (identique a OM — voir webhooks_om.go).
	sig := r.Header.Get("X-MTN-Signature")
	switch {
	case s.MTNWebhookSecret == "":
		s.Logger.Warn("webhooks/mtn: MTNWebhookSecret non configure — callback accepte sans verification")
	case sig == "":
		s.Logger.Warn("webhooks/mtn: signature requise (secret configure) mais header absent")
		writeError(w, http.StatusUnauthorized, "signature requise")
		return
	case !validateHMACSignature(rawBody, sig, s.MTNWebhookSecret):
		s.Logger.Warn("webhooks/mtn: signature HMAC invalide", "sig_received", sig)
		writeError(w, http.StatusUnauthorized, "signature invalide")
		return
	default:
		s.Logger.Info("webhooks/mtn: signature HMAC validee")
	}

	// Parser le payload JSON.
	var payload mtnPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		// MTN ne doit pas reessayer sur un payload malformed → 200 quand meme.
		s.Logger.Warn("webhooks/mtn: payload JSON invalide", "err", err, "raw", string(rawBody))
		w.WriteHeader(http.StatusOK)
		return
	}

	// Logger le callback pour traceabilite.
	s.Logger.Info("webhooks/mtn: callback recu",
		"reference_id", payload.ReferenceID,
		"external_id", payload.ExternalID,
		"status", payload.Status,
		"amount", payload.Amount,
		"currency", payload.Currency,
		"message_id", payload.MessageID,
	)

	// Reconciliation best-effort du statut de la transaction wallet correlee.
	// Ne credite jamais le solde (voir IMPORTANT en tete de fichier) — met a jour
	// uniquement wallet.wallet_transactions.status.
	reconcileWalletTransactionStatus(ctx, s, "mtn", payload.ReferenceID, mapMTNStatus(payload.Status))

	// Repondre 200 systematiquement (evite les replays MTN MoMo).
	w.WriteHeader(http.StatusOK)
}

// mapMTNStatus convertit le statut de transaction MTN Mobile Money (vocabulaire
// operateur) vers le vocabulaire interne persiste dans
// wallet.wallet_transactions.status (migration 0019, defaut 'completed').
//
// Fonction pure, testable isolement : aucune I/O, aucun effet de bord.
// Vocabulaire distinct de mapOMStatus ("SUCCESSFUL" et non "SUCCESS") — les deux
// fonctions sont gardees separees plutot que factorisees pour ne pas coupler les
// vocabulaires de deux operateurs independants qui peuvent diverger a tout moment.
//
//	"SUCCESSFUL" → "completed"
//	"FAILED"     → "failed"
//	"PENDING"    → "pending"
//	autre        → ""  (statut non reconnu — voir reconcileWalletTransactionStatus)
func mapMTNStatus(operatorStatus string) string {
	switch operatorStatus {
	case "SUCCESSFUL":
		return "completed"
	case "FAILED":
		return "failed"
	case "PENDING":
		return "pending"
	default:
		return ""
	}
}
