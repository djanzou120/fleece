package api

// wallet_topup.go — POST /wallet/{workspaceId}/topup
//
// Crédite le wallet d'un workspace (UPSERT).
//
// Choix : UPSERT (INSERT ... ON CONFLICT DO UPDATE) plutôt que 404 si le wallet
// n'existe pas. Raison : le premier topup d'un nouveau workspace crée le wallet
// automatiquement, ce qui est l'expérience attendue pour une plateforme prépayée
// (D8). L'alternative 404 forcerait un appel de création préalable — complexité
// inutile côté client.
//
// Colonnes réelles (migration 0002_wallet.sql, complétées par la migration
// additive 0019_wallet_reconciliation.sql) :
//   wallet.wallets : workspace_id, balance, currency, updated_at.
//   wallet.wallet_transactions : id, workspace_id, kind, amount, message_id,
//     created_at, status (défaut 'completed'), reference_id (nullable).
//
// Ecarts vs brief global (dettes documentées) :
//   - description accepté dans le body mais non persisté (colonne absente).
//   - kind inséré = "credit" (valeur reconnue, pas de colonne "type").
//   - La devise du UPSERT est 'XAF' par défaut (défaut migration 0002).
//   - reference_id est désormais accepté en entrée (optionnel) et persisté tel
//     quel — il sert de clé de corrélation pour les callbacks opérateur (voir
//     webhooks_om.go / webhooks_mtn.go, mécanisme de réconciliation). Absent du
//     body → NULL, et le comportement reste strictement celui d'avant la
//     migration 0019 (status='completed' par défaut, reference_id NULL).
//   - Aucune intégration PSP réelle : ce topup reste synchrone et immédiat ; le
//     champ reference_id ne fait qu'ouvrir la porte à une réconciliation
//     ultérieure, il ne déclenche aucun appel sortant.

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// walletTopupRequest est le corps JSON de POST /wallet/{workspaceId}/topup.
// description est accepté mais non persisté (colonne absente — dette schéma).
// reference_id est optionnel : absent/vide → NULL en base (rétrocompatibilité
// stricte avec le comportement pré-migration 0019).
type walletTopupRequest struct {
	AmountCents int64  `json:"amount_cents"`
	Description string `json:"description"`  // accepté, non persisté (colonne absente)
	ReferenceID string `json:"reference_id"` // optionnel — clé de corrélation callback opérateur
}

// walletTopupResponse est la réponse JSON de POST /wallet/{workspaceId}/topup (200 OK).
type walletTopupResponse struct {
	NewBalanceCents int64 `json:"new_balance_cents"`
}

// walletNewBalanceRow est utilisé pour scanner le RETURNING balance du UPSERT.
type walletNewBalanceRow struct {
	Balance int64 `db:"balance"`
}

// HandleTopupWallet traite POST /wallet/{workspaceId}/topup.
//
// Pipeline :
//  1. Valider {workspaceId} UUID + amount_cents > 0 → 400 sinon.
//  2. BEGIN transaction (gosql.Tx).
//  3. UPSERT wallet.wallets via tx.Get (INSERT ... ON CONFLICT DO UPDATE RETURNING balance).
//     Crée le wallet si inexistant, sinon incrémente le solde.
//  4. INSERT wallet.wallet_transactions (kind='credit') via tx.Exec.
//  5. COMMIT. 200 + { new_balance_cents }.
//
// Note : description du body est ignoré côté DB (colonne absente — migration additive requise).
func (s *Service) HandleTopupWallet(w http.ResponseWriter, r *http.Request) {
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
	var req walletTopupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requete invalide")
		return
	}
	if req.AmountCents <= 0 {
		writeError(w, http.StatusBadRequest, "amount_cents doit etre un entier strictement positif")
		return
	}

	// 2. BEGIN transaction via gosql.DB.Begin (retourne *gosql.Tx).
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		s.Logger.Error("wallet/topup: begin tx", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	defer func() { _ = tx.Rollback() }() // no-op si Commit() a déjà été appelé

	// 3. UPSERT wallet.wallets avec RETURNING balance.
	// INSERT si le wallet n'existe pas (premier topup crée le wallet).
	// ON CONFLICT : incrémente le solde et met à jour updated_at.
	// La devise par défaut 'XAF' est celle de la migration 0002.
	// description n'est pas persisté (colonne absente — dette documentée).
	// tx.Get supporte les requêtes RETURNING via sqlx.GetContext.
	var newBalRow walletNewBalanceRow
	if err := tx.Get(ctx, &newBalRow,
		`INSERT INTO wallet.wallets (workspace_id, balance, currency, updated_at)
		 VALUES ($1, $2, 'XAF', now())
		 ON CONFLICT (workspace_id) DO UPDATE
		    SET balance    = wallet.wallets.balance + $2,
		        updated_at = now()
		 RETURNING balance`,
		workspaceID, req.AmountCents,
	); err != nil {
		s.Logger.Error("wallet/topup: upsert wallet", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 4. Insérer la transaction dans le ledger.
	// kind = "credit" (valeur reconnue, migration 0002).
	// message_id = NULL (topup n'est pas lié à un message).
	// description du body n'est pas persisté (colonne absente — dette documentée).
	// reference_id = NULL si absent du body (rétrocompatibilité stricte) ; sinon
	// la valeur fournie par le client, utilisée pour la réconciliation callback
	// opérateur (voir webhooks_om.go / webhooks_mtn.go). status garde le défaut
	// 'completed' de la migration 0019 (topup synchrone, jamais 'pending' ici).
	var refID sql.NullString
	if req.ReferenceID != "" {
		refID = sql.NullString{String: req.ReferenceID, Valid: true}
	}
	if err := tx.Exec(ctx,
		`INSERT INTO wallet.wallet_transactions (workspace_id, kind, amount, message_id, reference_id)
		 VALUES ($1, 'credit', $2, NULL, $3)`,
		workspaceID, req.AmountCents, refID,
	); err != nil {
		s.Logger.Error("wallet/topup: insert transaction", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 5. Commit.
	if err := tx.Commit(); err != nil {
		s.Logger.Error("wallet/topup: commit", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	writeJSON(w, http.StatusOK, walletTopupResponse{NewBalanceCents: newBalRow.Balance})
}
