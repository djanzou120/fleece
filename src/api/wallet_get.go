package api

// wallet_get.go — GET /wallet/{workspaceId}
//
// Retourne le wallet d'un workspace.
//
// Colonnes réelles (migration 0002_wallet.sql) :
//   workspace_id uuid PRIMARY KEY, balance bigint, currency text, updated_at timestamptz.
//
// Ecarts vs brief global : le brief mentionne "balance_cents" — la colonne réelle est "balance".
// Ce handler expose "balance" (nom colonne réel) ET "balance_cents" comme alias pour la
// compatibilité avec les clients qui attendent le nom du brief.

import (
	"database/sql"
	"errors"
	"net/http"
	"time"
)

// walletRow est le résultat d'un SELECT sur wallet.wallets.
// Colonnes exactes de la migration 0002 :
// workspace_id, balance, currency, updated_at.
// Pas de colonne id, pas de created_at (contrairement au brief global).
type walletRow struct {
	WorkspaceID string    `db:"workspace_id" json:"workspace_id"`
	Balance     int64     `db:"balance"      json:"balance"`
	Currency    string    `db:"currency"     json:"currency"`
	UpdatedAt   time.Time `db:"updated_at"   json:"updated_at"`
}

// walletGetResponse est la réponse JSON de GET /wallet/{workspaceId}.
// On expose les deux noms (balance et balance_cents) pour la compatibilité.
// balance_cents = balance (même valeur, deux clés JSON pour documenter l'unité).
type walletGetResponse struct {
	WorkspaceID  string    `json:"workspace_id"`
	Balance      int64     `json:"balance"`       // nom réel de la colonne
	BalanceCents int64     `json:"balance_cents"` // alias documenté : même valeur que balance
	Currency     string    `json:"currency"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// HandleGetWallet traite GET /wallet/{workspaceId}.
//
// Pipeline :
//  1. Extraire {workspaceId} depuis PathValue → valider UUID → 400 sinon.
//  2. SELECT wallet.wallets WHERE workspace_id=$1 → 404 si sql.ErrNoRows.
//  3. 200 + JSON.
func (s *Service) HandleGetWallet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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

	var row walletRow
	if err := s.DB.Get(ctx, &row,
		`SELECT workspace_id, balance, currency, updated_at
		   FROM wallet.wallets
		  WHERE workspace_id = $1`,
		workspaceID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "wallet introuvable")
			return
		}
		s.Logger.Error("wallet: get by workspace_id", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	writeJSON(w, http.StatusOK, walletGetResponse{
		WorkspaceID:  row.WorkspaceID,
		Balance:      row.Balance,
		BalanceCents: row.Balance, // même valeur, alias documenté
		Currency:     row.Currency,
		UpdatedAt:    row.UpdatedAt,
	})
}
