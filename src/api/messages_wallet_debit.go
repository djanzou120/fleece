package api

// messages_wallet_debit.go — débit du wallet à l'envoi d'un message (D-M36).
//
// ══════════════════════════════════════════════════════════════════════════════
// LE DÉFAUT CORRIGÉ : LE REMBOURSEMENT CRÉAIT DE LA MONNAIE
// ══════════════════════════════════════════════════════════════════════════════
//
// `POST /messages` ne débitait RIEN. Aucun appelant de `POST /wallet/{id}/deduct`
// n'existait — ni Go, ni graphql-api, ni rest-api — et aucun trigger SQL ne
// suppléait. Pendant ce temps, `core-processor/on_message_failed.go` crédite
// `balance = balance + cost` à partir de `messaging.messages.cost` dès qu'un DLR
// d'échec arrive, et écrit une ligne de ledger `kind='refund'`.
//
// Autrement dit : chaque échec de message payant **créditait un wallet qui
// n'avait jamais été débité**. De la monnaie apparaissait, et le ledger
// affichait des `refund` sans `debit` correspondant. Le défaut était masqué
// par le fait que seul Telegram publie aujourd'hui des DLR, avec `cost = 0` —
// il devenait exploitable dès le branchement d'un provider payant, et
// `SMSTwilio.EstimateCost` retourne déjà 25.
//
// Corollaire relevé par la dette et corrigé ici même : le commentaire de
// `on_message_failed.go` justifiait son traitement du wallet introuvable par
// « le débit initial a dû réussir à l'envoi, voir wallet_deduct.go ». Cette
// prémisse était **factuellement fausse** — il n'y avait pas de débit initial.
// Elle devient vraie avec ce fichier.
//
// ══════════════════════════════════════════════════════════════════════════════
// OÙ LE DÉBIT A LIEU, ET POURQUOI LÀ
// ══════════════════════════════════════════════════════════════════════════════
//
// Le débit est appliqué **dans la même transaction que l'INSERT du message**
// (voir HandleSendMessage). C'est le point qui rend l'invariant vérifiable :
// une ligne `messaging.messages` avec `cost > 0` existe **si et seulement si**
// le débit correspondant a été committé. Le remboursement de `core-processor`,
// qui part de `messaging.messages.cost`, ne peut donc plus rembourser un débit
// qui n'a pas eu lieu.
//
// Un débit hors transaction (par exemple via un appel HTTP à
// `POST /wallet/{id}/deduct`) aurait rouvert le même trou sous une autre forme :
// débit committé puis INSERT en échec ⇒ argent prélevé sans message ; ou
// l'inverse ⇒ le bug d'origine. L'appel HTTP interne serait de surcroît absurde
// ici : `src/api` est le service qui EXPOSE cet endpoint, il s'appellerait
// lui-même par le réseau pour écrire dans une base à laquelle il est déjà
// connecté.
//
// ══════════════════════════════════════════════════════════════════════════════
// ORDRE DES OPÉRATIONS ET FENÊTRE RÉSIDUELLE (ASSUMÉE, DOCUMENTÉE)
// ══════════════════════════════════════════════════════════════════════════════
//
// Le coût RÉEL n'est connu qu'APRÈS la réponse du provider (`result.Cost`),
// alors que la garantie « prépayé » exige de refuser AVANT d'envoyer. D'où deux
// temps :
//
//  1. **Pré-vol** (assertSufficientBalance, appelé AVANT provider.Send) : lecture
//     seule du solde, comparée à `EstimateCost`. Insuffisant ⇒ **402 Payment
//     Required**, aucun message envoyé, aucune écriture. C'est ce qui tient la
//     promesse du prépaiement.
//  2. **Débit réel** (debitWalletForMessage, dans la transaction de l'INSERT) :
//     montant exact retourné par le provider.
//
// FENÊTRE RÉSIDUELLE : entre le pré-vol et le débit, un envoi concurrent peut
// avoir consommé le solde. Le message est alors DÉJÀ PARTI — on ne peut pas le
// rappeler. Décision retenue : **laisser le solde devenir négatif** et logguer
// en ERROR, plutôt que de renoncer au débit. Renoncer recréerait très exactement
// le bug corrigé ici (un refund sans debit) ; un solde négatif est au contraire
// une dette exacte, visible, et régularisable au prochain top-up. La seule façon
// de fermer complètement cette fenêtre est une **réservation** (débiter
// l'estimation avant l'envoi, puis régulariser l'écart) — évolution V1, qui
// suppose un état « réservé » dans le ledger.
//
// GRATUITÉ : `cost == 0` (Telegram aujourd'hui) ne déclenche NI pré-vol NI
// débit — pas de 402 sur un canal gratuit, pas de ligne de ledger à 0, pas de
// verrou pris sur le wallet.

import (
	"context"
	"database/sql"
	"errors"

	gosql "fleece/src/go/sql"
)

// errInsufficientBalance signale un solde insuffisant au pré-vol. Traduite en
// 402 Payment Required par HandleSendMessage.
var errInsufficientBalance = errors.New("solde insuffisant")

// errWalletNotFound signale l'absence de wallet pour le workspace.
var errWalletNotFound = errors.New("wallet introuvable")

// assertSufficientBalance vérifie, EN LECTURE SEULE et AVANT tout envoi, que le
// workspace peut payer `estimated`. Aucun verrou n'est pris : c'est un pré-vol,
// pas une réservation (voir doc de tête de fichier).
//
// estimated <= 0 court-circuite : un canal gratuit n'a pas de solde à vérifier.
func (s *Service) assertSufficientBalance(ctx context.Context, workspaceID string, estimated int64) error {
	if estimated <= 0 {
		return nil
	}

	var row struct {
		Balance int64 `db:"balance"`
	}
	err := s.DB.Get(ctx, &row,
		`SELECT balance FROM wallet.wallets WHERE workspace_id = $1`,
		workspaceID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return errWalletNotFound
	}
	if err != nil {
		return err
	}
	if row.Balance < estimated {
		return errInsufficientBalance
	}
	return nil
}

// debitWalletForMessage applique le débit DANS la transaction fournie : mise à
// jour du solde puis ligne de ledger `kind='debit'`, rattachée au message.
//
// Le couple (UPDATE balance, INSERT ledger) reproduit exactement ce que fait le
// remboursement de core-processor en miroir (`kind='refund'`, même colonne
// `message_id`) : sans la ligne de ledger, le journal afficherait des refunds
// sans debit correspondant — le symptôme comptable du bug corrigé ici.
//
// `SELECT ... FOR UPDATE` n'est pas nécessaire : `balance = balance - $1` est
// résolu par PostgreSQL sur la version courante de la ligne, qu'il verrouille
// pour la durée de la transaction. Un décrément relatif est correct sous
// concurrence, là où un `SET balance = <valeur lue>` ne le serait pas.
//
// Retourne errWalletNotFound si aucun wallet n'existe (RETURNING ne ramène
// alors aucune ligne) — l'appelant décide quoi en faire, le message étant déjà
// parti à ce stade.
//
// cost <= 0 court-circuite : pas de débit, pas de ligne de ledger.
func debitWalletForMessage(ctx context.Context, tx *gosql.Tx, workspaceID, messageID string, cost int64) error {
	if cost <= 0 {
		return nil
	}

	// RETURNING + Get pour détecter « 0 ligne affectée » sans ExecRows sur
	// *gosql.Tx (absent — D-M16), même technique que on_message_failed.go.
	var updated struct {
		Balance int64 `db:"balance"`
	}
	err := tx.Get(ctx, &updated,
		`UPDATE wallet.wallets SET balance = balance - $1, updated_at = now()
		   WHERE workspace_id = $2
		 RETURNING balance`,
		cost, workspaceID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return errWalletNotFound
	}
	if err != nil {
		return err
	}

	return tx.Exec(ctx,
		`INSERT INTO wallet.wallet_transactions (workspace_id, kind, amount, message_id)
		 VALUES ($1, 'debit', $2, $3)`,
		workspaceID, cost, messageID,
	)
}

// persistSentMessage écrit le message ET son débit dans UNE SEULE transaction
// (D-M36) — voir doc de tête de fichier pour l'invariant que cela garantit.
//
// Le message est DÉJÀ PARTI quand cette fonction est appelée : on ne peut plus
// l'annuler. C'est ce qui dicte le traitement des cas dégradés ci-dessous.
func (s *Service) persistSentMessage(
	ctx context.Context,
	msgID string,
	req sendMessageRequest,
	providerID, channel, status string,
	result ProviderResult,
) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // no-op après un Commit réussi

	if err := insertMessageTx(ctx, tx, msgID, req.WorkspaceID, req.Recipient, req.Body,
		providerID, channel, status, result.ExternalID, result.Cost); err != nil {
		return err
	}

	if err := debitWalletForMessage(ctx, tx, req.WorkspaceID, msgID, result.Cost); err != nil {
		if errors.Is(err, errWalletNotFound) {
			// Le pré-vol avait trouvé un wallet (sinon on aurait répondu 402
			// avant d'envoyer) : sa disparition entre-temps est une anomalie de
			// données, pas une panne transitoire. Le message est parti — on
			// préserve sa trace plutôt que de tout annuler, et on logge pour
			// investigation. Le remboursement éventuel rencontrera la même
			// anomalie et la journalisera de la même façon.
			s.Logger.Error("messages: debit impossible, wallet introuvable (anomalie de donnees)",
				"workspace_id", req.WorkspaceID, "message_id", msgID, "cost", result.Cost)
		} else {
			return err
		}
	}

	return tx.Commit()
}
