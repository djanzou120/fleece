package api

// wallet_deduct.go — POST /wallet/{workspaceId}/deduct
//
// Débite le wallet d'un workspace de façon atomique (SELECT ... FOR UPDATE).
//
// Colonnes réelles (migration 0002_wallet.sql) :
//   wallet.wallets : workspace_id, balance, currency, updated_at.
//   wallet.wallet_transactions : id, workspace_id, kind, amount, message_id (nullable), created_at.
//
// Ecarts vs brief global (dettes documentées) :
//   - Le body accepte description, reference_id, mais ces champs NE SONT PAS persistés
//     (colonnes absentes du schéma réel). Migration additive requise pour les ajouter.
//   - reference_id : si c'est un UUID valide de message, on le stocke dans message_id ;
//     sinon message_id=NULL.
//   - kind inséré = "debit" (valeur reconnue par le schéma réel, pas "type").
//   - Pas de colonne status sur wallet_transactions (absent du schéma réel).

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

// walletDeductRequest est le corps JSON de POST /wallet/{workspaceId}/deduct.
// description et reference_id sont acceptés mais non persistés (dette schéma).
type walletDeductRequest struct {
	AmountCents int64  `json:"amount_cents"`
	Description string `json:"description"`  // accepté, non persisté (colonne absente)
	ReferenceID string `json:"reference_id"` // accepté ; si UUID valide → stocké dans message_id
}

// walletDeductResponse est la réponse JSON de POST /wallet/{workspaceId}/deduct (200 OK).
type walletDeductResponse struct {
	NewBalanceCents int64 `json:"new_balance_cents"`
}

// walletBalanceRow est utilisé pour lire le solde dans la transaction.
type walletBalanceRow struct {
	Balance int64 `db:"balance"`
}

// HandleDeductWallet traite POST /wallet/{workspaceId}/deduct.
//
// Pipeline :
//  1. Valider {workspaceId} UUID + amount_cents > 0 → 400 sinon.
//  2. BEGIN transaction (gosql.Tx).
//  3. SELECT balance FROM wallet.wallets WHERE workspace_id=$1 FOR UPDATE via tx.Get.
//     Si sql.ErrNoRows → 404 (rollback implicite via defer).
//  4. Si balance < amount_cents → 402 Payment Required (rollback implicite via defer).
//  5. UPDATE wallet.wallets SET balance = balance - $1, updated_at = now() via tx.Exec.
//  6. INSERT INTO wallet.wallet_transactions (workspace_id, kind, amount, message_id) via tx.Exec.
//  7. COMMIT. 200 + { new_balance_cents }.
func (s *Service) HandleDeductWallet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Valider le workspaceId.
	rawID := r.PathValue("workspaceId")
	if rawID == "" {
		writeError(w, http.StatusBadRequest, "workspaceId requis dans le chemin")
		return
	}
	workspaceID, err := parseUUID(rawID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "workspaceId : format UUID invalide")
		return
	}

	// Décoder le corps JSON.
	var req walletDeductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requete invalide")
		return
	}
	if req.AmountCents <= 0 {
		writeError(w, http.StatusBadRequest, "amount_cents doit etre un entier strictement positif")
		return
	}

	// Résoudre message_id depuis reference_id (optionnel).
	// Si reference_id est un UUID valide → on le stocke dans message_id (colonne existante).
	// Sinon → message_id = NULL (via driver lib/pq : nil pointeur = NULL).
	var messageID *string
	if req.ReferenceID != "" {
		if id, err := parseUUID(req.ReferenceID); err == nil {
			messageID = &id
		}
		// Si reference_id n'est pas un UUID valide, on l'ignore (pas de colonne pour le stocker).
	}

	// 2. BEGIN transaction via gosql.DB.Begin (retourne *gosql.Tx).
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		s.Logger.Error("wallet/deduct: begin tx", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	defer func() { _ = tx.Rollback() }() // no-op si Commit() a déjà été appelé

	// 3. SELECT ... FOR UPDATE — verrou ligne pour éviter les race conditions.
	// Utilise tx.Get (méthode de *gosql.Tx) avec scan dans walletBalanceRow.
	var balRow walletBalanceRow
	if err := tx.Get(ctx, &balRow,
		`SELECT balance FROM wallet.wallets WHERE workspace_id = $1 FOR UPDATE`,
		workspaceID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "wallet introuvable")
			return
		}
		s.Logger.Error("wallet/deduct: select for update", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 4. Vérifier le solde suffisant.
	if balRow.Balance < req.AmountCents {
		writeError(w, http.StatusPaymentRequired, "solde insuffisant")
		return
	}

	// 5. Mettre à jour le solde.
	if err := tx.Exec(ctx,
		`UPDATE wallet.wallets
		    SET balance    = balance - $1,
		        updated_at = now()
		  WHERE workspace_id = $2`,
		req.AmountCents, workspaceID,
	); err != nil {
		s.Logger.Error("wallet/deduct: update balance", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 6. Insérer la transaction dans le ledger.
	// kind = "debit" (valeur reconnue, migration 0002).
	// description et reference_id du body ne sont PAS persistés (colonnes absentes — dette).
	if err := tx.Exec(ctx,
		`INSERT INTO wallet.wallet_transactions (workspace_id, kind, amount, message_id)
		 VALUES ($1, 'debit', $2, $3)`,
		workspaceID, req.AmountCents, messageID,
	); err != nil {
		s.Logger.Error("wallet/deduct: insert transaction", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 7. Commit.
	if err := tx.Commit(); err != nil {
		s.Logger.Error("wallet/deduct: commit", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	newBalance := balRow.Balance - req.AmountCents
	writeJSON(w, http.StatusOK, walletDeductResponse{NewBalanceCents: newBalance})
}
