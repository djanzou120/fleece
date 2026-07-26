package coreprocessor

// on_message_failed.go — handler de l'evenement AMQP "message.failed"
// (routing key "message.failed", voir consumer.go pour le contrat exact de
// l'evenement, D-M21).
//
// Effet : transition messaging.messages.status -> 'failed', ET remboursement
// du wallet du workspace SI le message avait un cout > 0 (Telegram, le seul
// provider cable a ce jour, est gratuit — cost NULL/0 est donc le cas
// MAJORITAIRE aujourd'hui : c'est normal, pas un bug).
//
// IDEMPOTENCE : identique a on_message_delivered.go — garde SQL "AND status
// NOT IN ('delivered', 'failed', 'rejected')". Le remboursement n'est
// applique QUE si l'UPDATE a REELLEMENT transitionne la ligne (RETURNING a
// renvoye une ligne) : un double-traitement (redelivery AMQP) ne rembourse
// donc jamais deux fois.
//
// ATOMICITE : marquage du statut + remboursement se font DANS LA MEME
// transaction SQL (s.DB.Begin -> *gosql.Tx, defer Rollback, Commit explicite).
// Sans cela, un crash entre les deux operations laisserait soit un message
// 'failed' non rembourse, soit (si le crash survient apres un premier
// remboursement partiellement commite puis rejoue) un double remboursement.
//
// SOURCE DE VERITE DU MONTANT : le cout rembourse est TOUJOURS relu depuis
// messaging.messages.cost (colonne base, RETURNING dans l'UPDATE), JAMAIS
// depuis le payload AMQP (messageEvent ne porte d'ailleurs aucun champ cout —
// voir consumer.go) : un evenement forge ou rejoue avec un cout arbitraire ne
// doit jamais pouvoir crediter un wallet au-dela de ce qui a reellement ete
// debite a l'envoi.
//
// PAS DE PUBLICATION "wallet.refund" (decision PM, Phase 3) : le worker ECRIT
// le remboursement directement en base, dans la transaction ci-dessus, et NE
// PUBLIE AUCUN evenement supplementaire. Publier un evenement "en plus" de
// l'ecriture DB introduirait un second chemin de remboursement possible (si un
// jour un consumer "wallet.refund" apparait, le meme remboursement serait
// applique deux fois : une fois ici en DB, une fois par ce futur consumer).
// Le plan de migration initial mentionnait "emit wallet.refund si applicable"
// (voir .ia/MIGRATION_PLAN.md M-020) — cet ecart est deliberement enterine
// ici : ecriture DB transactionnelle uniquement.
import (
	"context"
	"database/sql"
	"errors"
)

// failedMessageRow scanne le RETURNING de l'UPDATE messaging.messages :
// workspace_id (pour cibler le wallet a rembourser) et cost (source de
// verite du montant, jamais le payload AMQP — voir doc de tete de fichier).
// cost est nullable (messages non factures ou en echec precoce, voir
// migration 0018) : NULL != 0, sql.NullInt64 distingue les deux.
type failedMessageRow struct {
	WorkspaceID string        `db:"workspace_id"`
	Cost        sql.NullInt64 `db:"cost"`
}

// shouldRefundCost decide si un remboursement doit etre applique, a partir du
// SEUL cout lu en base (jamais du payload AMQP). Fonction pure, testable
// isolement :
//   - cost NULL (message jamais facture, ou en echec avant tout cout connu) -> false.
//   - cost == 0 (provider gratuit, ex. Telegram) -> false. C'est le cas
//     majoritaire aujourd'hui — ne pas le traiter comme une anomalie.
//   - cost > 0 -> true.
func shouldRefundCost(cost sql.NullInt64) bool {
	return cost.Valid && cost.Int64 > 0
}

// handleMessageFailed traite l'evenement "message.failed".
func handleMessageFailed(ctx context.Context, s *Service, evt messageEvent) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		// Erreur technique (DB indisponible) : transitoire -> requeue.
		return err
	}
	// No-op si Commit() a deja ete appele (tx.Rollback apres Commit est
	// documente sans effet par database/sql — meme pattern que
	// wallet_deduct.go/wallet_topup.go).
	defer func() { _ = tx.Rollback() }()

	// 1. UPDATE ... RETURNING workspace_id, cost — transition ET lecture du
	// montant a rembourser en une seule requete (evite une fenetre de course
	// entre un SELECT et l'UPDATE).
	var row failedMessageRow
	err = tx.Get(ctx, &row,
		`UPDATE messaging.messages SET status = 'failed'
		   WHERE id = $1 AND status NOT IN ('delivered', 'failed', 'rejected')
		 RETURNING workspace_id, cost`,
		evt.MessageID,
	)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Idempotence : deja traite (statut terminal atteint par un
		// traitement precedent) ou message inexistant. 0 ligne affectee ->
		// AUCUN remboursement (c'est precisement ce qui rend le
		// remboursement idempotent). Pas une erreur : Ack normal.
		s.Logger.Info("core-processor: message.failed ignore (deja traite ou introuvable)",
			"message_id", evt.MessageID, "external_id", evt.ExternalID, "provider_id", evt.ProviderID)
		if cErr := tx.Commit(); cErr != nil {
			return cErr
		}
		return nil
	case err != nil:
		// Erreur technique (DB indisponible pendant la tx) : transitoire -> requeue.
		return err
	}

	// 2. Decider du remboursement a partir du SEUL cout relu en base.
	if !shouldRefundCost(row.Cost) {
		s.Logger.Info("core-processor: message marque failed, aucun remboursement (cost NULL ou 0)",
			"message_id", evt.MessageID, "workspace_id", row.WorkspaceID)
		return tx.Commit()
	}
	cost := row.Cost.Int64

	// 3. Crediter le wallet du workspace. RETURNING balance permet de
	// detecter "0 ligne affectee" (wallet introuvable) sans dependre d'un
	// ExecRows sur *gosql.Tx (absent — D-M16 — RETURNING+Get produit le meme
	// resultat sans etendre la lib partagee).
	var newBalance struct {
		Balance int64 `db:"balance"`
	}
	err = tx.Get(ctx, &newBalance,
		`UPDATE wallet.wallets SET balance = balance + $1, updated_at = now()
		   WHERE workspace_id = $2
		 RETURNING balance`,
		cost, row.WorkspaceID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Anomalie : un message facture (cost > 0) implique necessairement
		// qu'un wallet existe (le debit initial a du reussir a l'envoi, voir
		// wallet_deduct.go). Un wallet introuvable ici est une incoherence de
		// donnees, pas une panne transitoire — requeuer ne la resoudra
		// jamais. On logge en ERROR pour investigation et on COMMIT quand
		// meme la transition de statut deja appliquee (etape 1) plutot que
		// de la faire echouer indefiniment : la transition messaging.messages
		// reste correcte et observable, seul le remboursement est
		// manquant (traçable via ce log).
		s.Logger.Error("core-processor: remboursement impossible, wallet introuvable (anomalie de donnees)",
			"message_id", evt.MessageID, "workspace_id", row.WorkspaceID, "cost", cost)
		return tx.Commit()
	}
	if err != nil {
		return err // transitoire -> requeue
	}

	// 4. Ecrire le remboursement dans le ledger. kind="refund" : valeur
	// explicitement anticipee par le schema (migration 0002_wallet.sql,
	// commentaire "Ledger append-only : debit | credit | refund"), au meme
	// titre que "debit"/"credit" utilises par wallet_deduct.go/wallet_topup.go.
	// message_id (uuid nullable, migration 0002) rattache la ligne de ledger
	// au message rembourse — meme convention que wallet_deduct.go
	// (reference_id -> message_id si UUID valide).
	if err := tx.Exec(ctx,
		`INSERT INTO wallet.wallet_transactions (workspace_id, kind, amount, message_id)
		 VALUES ($1, 'refund', $2, $3)`,
		row.WorkspaceID, cost, evt.MessageID,
	); err != nil {
		return err // transitoire -> requeue
	}

	// 5. Commit — transition de statut + remboursement appliques atomiquement.
	if err := tx.Commit(); err != nil {
		return err
	}

	s.Logger.Info("core-processor: message marque failed, remboursement applique",
		"message_id", evt.MessageID, "workspace_id", row.WorkspaceID,
		"cost", cost, "new_balance", newBalance.Balance)
	return nil
}
