package api

// webhooks_om.go — POST /webhooks/om (callback Orange Money)
//
// Ce handler recoit les notifications de paiement de l'operateur Orange Money.
// Il valide la signature HMAC-SHA256 si le header X-OM-Signature est present,
// parse le payload OM, puis reconcilie le statut de la transaction wallet
// correspondante (voir "RECONCILIATION WALLET" ci-dessous).
//
// RECONCILIATION WALLET (migration additive 0019_wallet_reconciliation.sql) :
//   wallet.wallet_transactions porte desormais les colonnes "status" (defaut
//   'completed') et "reference_id" (nullable, index non-unique). Le handler
//   execute :
//     UPDATE wallet.wallet_transactions SET status = $1 WHERE reference_id = $2
//   ou $2 = payload.TransactionID et $1 = mapOMStatus(payload.Status) (fonction
//   pure, voir plus bas). Voir reconcileWalletTransactionStatus (webhooks_om.go,
//   partagee avec webhooks_mtn.go) pour le detail du comportement tolerant aux
//   erreurs (transaction_id absent, statut inconnu, 0/>1 ligne affectee, erreur
//   DB — aucun de ces cas ne fait echouer la requete : toujours 200).
//
//   reference_id n'est renseignee cote wallet.wallet_transactions que si le
//   topup a ete cree avec un reference_id (voir wallet_topup.go, champ optionnel
//   du body). Un callback dont le reference_id ne correspond a aucune ligne est
//   donc un cas attendu (topup historique sans reference_id, callback en double,
//   ou callback pour une transaction geree par un autre canal) — pas une erreur.
//
// IMPORTANT — PAS DE CREDIT DE SOLDE ICI (D8, .ia/MEMORY.md) :
//   Ce handler NE credite JAMAIS wallet.wallets.balance. Il se limite a mettre a
//   jour le statut d'une transaction deja inseree par HandleTopupWallet (ou un
//   futur flux d'initiation de paiement). Crediter le solde directement depuis un
//   callback operateur, qui peut etre rejoue par le fournisseur (replays sur
//   timeout/non-200), poserait un probleme d'idempotence non trivial (credit en
//   double). L'integration paiement operateur complete (initiation du paiement +
//   credit automatique et idempotent du solde a la confirmation) est explicitement
//   hors perimetre V1 et reportee a V2.
//
// DECISION DE VALIDATION HMAC (politique explicite, cf. revue d'architecture B2) :
//   - Service.OMWebhookSecret CONFIGURE (non vide) :
//       - Header X-OM-Signature absent  → 401 (signature obligatoire des lors que le
//         secret est configure ; on ne peut pas accepter un callback non authentifie
//         sur un flux de paiement).
//       - Header present + invalide     → 401 (rejet explicite).
//       - Header present + valide       → 200, poursuite du traitement.
//   - Service.OMWebhookSecret VIDE (non configure) :
//       - Aucune validation n'est effectuee (impossible sans cle) ; le callback est
//         accepte avec un log WARN explicite. NE DOIT PAS arriver en production —
//         seul un environnement de dev/test sans OM_WEBHOOK_SECRET tombe dans ce cas.
//
// SECRET HMAC :
//   Injecte via Service.OMWebhookSecret (composition root cmd/api/main.go, variable
//   d'environnement OM_WEBHOOK_SECRET). Plus de constante codee en dur (ecart B2 —
//   un secret de paiement en dur dans le binaire, identique entre OM/MTN, etait une
//   faille de securite critique).

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
)

// omPayload represente le corps JSON envoye par Orange Money lors d'un callback.
// Le format exact de l'API Orange Money est proprietaire et susceptible d'evoluer.
// TODO(production): affiner cette struct selon la documentation officielle OM API.
type omPayload struct {
	// TransactionID est l'identifiant de la transaction cote Orange Money.
	// Utilise comme cle de correlation avec wallet.wallet_transactions.reference_id
	// (migration 0019).
	TransactionID string `json:"transaction_id"`
	// Status est le statut de la transaction : "SUCCESS", "FAILED", "PENDING".
	// Mappe vers le vocabulaire interne via mapOMStatus.
	Status string `json:"status"`
	// Amount est le montant en centimes de la devise locale.
	Amount int64 `json:"amount"`
	// Currency est la devise (ex. "XAF", "EUR").
	Currency string `json:"currency"`
	// MessageID est un identifiant optionnel permettant de corréler la transaction
	// avec un message Fleece (messaging.messages.id). Non exploite pour l'instant
	// (aucun flux d'envoi de message ne declenche de paiement OM en V1) —
	// conserve pour compatibilite ascendante du payload.
	MessageID string `json:"message_id,omitempty"`
	// RawResponse est reserve pour d'eventuels champs supplementaires OM.
	// Le parsing genereux (champs inconnus ignores) permet la compatibilite ascendante.
}

// HandleWebhookOM traite POST /webhooks/om.
//
// Pipeline :
//  1. Lire le body complet (necessaire pour la validation HMAC).
//  2. Appliquer la politique de validation HMAC (voir DECISION ci-dessus) selon
//     que Service.OMWebhookSecret est configure ou non.
//  3. Parser le payload JSON.
//  4. Logger le callback pour traceabilite.
//  5. Reconcilier le statut de la transaction wallet correlee (reference_id),
//     sans jamais crediter le solde (voir RECONCILIATION WALLET ci-dessus).
//  6. Repondre 200 (requis par Orange Money pour eviter les replays).
func (s *Service) HandleWebhookOM(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Lire le body complet (avant tout — le body ne peut etre lu qu'une fois).
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		// Impossibilite de lire le body : on log et on renvoie 200 quand meme
		// pour eviter les replays OM sur une erreur reseau transitoire.
		s.Logger.Warn("webhooks/om: lecture body echouee", "err", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Politique de validation HMAC.
	sig := r.Header.Get("X-OM-Signature")
	switch {
	case s.OMWebhookSecret == "":
		// Secret non configure : validation impossible — log WARN explicite.
		// Ne doit pas arriver en production (OM_WEBHOOK_SECRET manquant).
		s.Logger.Warn("webhooks/om: OMWebhookSecret non configure — callback accepte sans verification")
	case sig == "":
		// Secret configure : la signature devient obligatoire.
		s.Logger.Warn("webhooks/om: signature requise (secret configure) mais header absent")
		writeError(w, http.StatusUnauthorized, "signature requise")
		return
	case !validateHMACSignature(rawBody, sig, s.OMWebhookSecret):
		s.Logger.Warn("webhooks/om: signature HMAC invalide", "sig_received", sig)
		writeError(w, http.StatusUnauthorized, "signature invalide")
		return
	default:
		s.Logger.Info("webhooks/om: signature HMAC validee")
	}

	// 3. Parser le payload JSON.
	var payload omPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		// Payload malformed : on log mais on renvoie 200 pour eviter les replays
		// (Orange Money re-essaierait indefiniment sur un 4xx).
		s.Logger.Warn("webhooks/om: payload JSON invalide", "err", err, "raw", string(rawBody))
		w.WriteHeader(http.StatusOK)
		return
	}

	// 4. Logger le callback pour traceabilite.
	s.Logger.Info("webhooks/om: callback recu",
		"transaction_id", payload.TransactionID,
		"status", payload.Status,
		"amount", payload.Amount,
		"currency", payload.Currency,
		"message_id", payload.MessageID,
	)

	// 5. Reconciliation best-effort du statut de la transaction wallet correlee.
	// Ne credite jamais le solde (voir IMPORTANT ci-dessus) — met a jour uniquement
	// wallet.wallet_transactions.status.
	reconcileWalletTransactionStatus(ctx, s, "om", payload.TransactionID, mapOMStatus(payload.Status))

	// 6. Repondre 200 systematiquement (evite les replays Orange Money).
	w.WriteHeader(http.StatusOK)
}

// mapOMStatus convertit le statut de transaction Orange Money (vocabulaire
// operateur) vers le vocabulaire interne persiste dans
// wallet.wallet_transactions.status (migration 0019, defaut 'completed').
//
// Fonction pure, testable isolement : aucune I/O, aucun effet de bord.
//
//	"SUCCESS" → "completed"
//	"FAILED"  → "failed"
//	"PENDING" → "pending"
//	autre     → ""  (statut non reconnu — l'appelant doit alors s'abstenir de
//	                  toute mise a jour et logger un WARN, voir
//	                  reconcileWalletTransactionStatus)
func mapOMStatus(operatorStatus string) string {
	switch operatorStatus {
	case "SUCCESS":
		return "completed"
	case "FAILED":
		return "failed"
	case "PENDING":
		return "pending"
	default:
		return ""
	}
}

// reconcileWalletTransactionStatus met a jour le statut d'une transaction wallet
// correlee par reference_id, suite a un callback paiement operateur (OM ou MTN).
// Partagee entre webhooks_om.go et webhooks_mtn.go — la logique de reconciliation
// est identique une fois le statut mappe vers le vocabulaire interne ; seul le
// vocabulaire operateur differe (mapOMStatus vs mapMTNStatus).
//
// Comportement (idempotent, tolerant aux erreurs — ne fait JAMAIS echouer la
// requete HTTP, le webhook repond toujours 200) :
//   - transactionID vide                : log WARN, aucun UPDATE.
//   - internalStatus vide (mapping operateur inconnu) : log WARN, aucun UPDATE.
//   - UPDATE echoue (erreur technique)  : log ERROR, la requete repond quand
//     meme 200 (les operateurs rejouent les callbacks sur non-200 — une 500
//     ici provoquerait une tempete de replays sans jamais resoudre l'erreur DB).
//   - 0 ligne affectee                  : log WARN — aucune transaction n'attendait
//     ce callback (reference_id inconnue, topup historique sans reference_id,
//     callback en double apres reconciliation deja faite, OU callback tardif/
//     rejoue sur une transaction deja dans un statut final — voir garde
//     d'idempotence ci-dessous).
//   - >1 ligne affectee                 : log WARN — anomalie de donnees, l'index
//     sur reference_id (migration 0019) est volontairement NON-unique.
//   - 1 ligne affectee                  : log INFO, cas nominal.
//
// GARDE D'IDEMPOTENCE (D-M15) : l'UPDATE ne s'applique que si le statut courant
// de la transaction n'est pas deja final (`status NOT IN ('completed', 'failed')`).
// Un callback operateur tardif ou rejoue (ex. un "FAILED" recu apres qu'un
// "SUCCESS" ait deja ete reconcilie en 'completed', ou l'inverse) ne doit jamais
// ecraser un statut final deja etabli — les operateurs mobile money peuvent
// re-livrer un callback sur timeout/non-200 independamment de l'ordre reel des
// evenements. On choisit une liste explicite de statuts finaux (plutot qu'un
// simple `status <> $1`, qui laisserait passer un UPDATE inoffensif mais inutile
// re-ecrivant la MEME valeur finale, sans le distinguer d'un ecrasement reel) :
// des qu'une transaction atteint 'completed' ou 'failed', plus aucun callback
// ulterieur (meme portant le meme statut) ne la modifie ; 'pending' reste
// toujours mis a jour normalement (etat non final).
//
// NE CREDITE JAMAIS wallet.wallets.balance — voir IMPORTANT en tete de fichier.
func reconcileWalletTransactionStatus(ctx context.Context, s *Service, provider, transactionID, internalStatus string) {
	if transactionID == "" {
		s.Logger.Warn("webhooks/" + provider + ": reconciliation ignoree — identifiant de transaction absent du callback")
		return
	}
	if internalStatus == "" {
		s.Logger.Warn("webhooks/"+provider+": reconciliation ignoree — statut operateur non reconnu",
			"reference_id", transactionID)
		return
	}

	rowsAffected, err := s.DB.ExecRows(ctx,
		`UPDATE wallet.wallet_transactions SET status = $1 WHERE reference_id = $2 AND status NOT IN ('completed', 'failed')`,
		internalStatus, transactionID,
	)
	if err != nil {
		s.Logger.Error("webhooks/"+provider+": update wallet_transactions echoue",
			"reference_id", transactionID, "status", internalStatus, "err", err)
		return
	}

	switch {
	case rowsAffected == 0:
		s.Logger.Warn("webhooks/"+provider+": aucune transaction wallet correspondante (reference_id inconnue)",
			"reference_id", transactionID)
	case rowsAffected > 1:
		s.Logger.Warn("webhooks/"+provider+": plusieurs transactions wallet correspondantes (reference_id non unique)",
			"reference_id", transactionID, "rows_affected", rowsAffected)
	default:
		s.Logger.Info("webhooks/"+provider+": transaction wallet reconciliee",
			"reference_id", transactionID, "status", internalStatus)
	}
}

// validateHMACSignature verifie que la signature hexadecimale attendue correspond
// au HMAC-SHA256 du body avec la cle donnee.
// Utilise hmac.Equal pour une comparaison en temps constant (resistance aux timing attacks).
func validateHMACSignature(body []byte, receivedSig, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	// hmac.Equal travaille sur des []byte — on convertit les deux cotes.
	return hmac.Equal([]byte(expected), []byte(receivedSig))
}
