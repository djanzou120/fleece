package api

// wallet_transactions.go — GET /wallet/{workspaceId}/transactions
//
// Retourne la liste paginée des transactions d'un workspace.
//
// Colonnes réelles (migration 0002_wallet.sql) :
//   id bigserial PK, workspace_id uuid, kind text, amount bigint,
//   message_id uuid (nullable), created_at timestamptz.
//
// Ecarts vs brief global :
//   - La colonne s'appelle "kind" (valeurs : debit|credit|refund), PAS "type".
//   - Pas de colonnes description, reference_id, status (absentes du schéma réel).
//     Ces colonnes seraient une migration additive (dette documentée).

import (
	"net/http"
	"strconv"
	"time"
)

// walletTransactionRow est le résultat d'un SELECT sur wallet.wallet_transactions.
// Colonnes exactes de la migration 0002 :
// id, workspace_id, kind, amount, message_id (nullable), created_at.
type walletTransactionRow struct {
	ID          int64     `db:"id"           json:"id"`
	WorkspaceID string    `db:"workspace_id" json:"workspace_id"`
	Kind        string    `db:"kind"         json:"kind"` // "debit"|"credit"|"refund"
	Amount      int64     `db:"amount"       json:"amount"`
	MessageID   *string   `db:"message_id"   json:"message_id"` // nullable → *string
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
}

// HandleListWalletTransactions traite GET /wallet/{workspaceId}/transactions.
//
// Paramètres de pagination :
//
//	?limit=<n>   défaut 50, borne max 200.
//	?offset=<n>  défaut 0.
//
// Pipeline :
//  1. Valider {workspaceId} UUID → 400 sinon.
//  2. Parser limit/offset depuis les query params.
//  3. SELECT wallet_transactions WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3.
//  4. 200 + tableau JSON (toujours un tableau, jamais null).
func (s *Service) HandleListWalletTransactions(w http.ResponseWriter, r *http.Request) {
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

	// 2. Parser limit et offset depuis les query params.
	limit := 50
	offset := 0

	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		n, err := strconv.Atoi(rawLimit)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "limit doit etre un entier positif")
			return
		}
		if n > 200 {
			n = 200 // borne max
		}
		limit = n
	}

	if rawOffset := r.URL.Query().Get("offset"); rawOffset != "" {
		n, err := strconv.Atoi(rawOffset)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset doit etre un entier positif ou nul")
			return
		}
		offset = n
	}

	// 3. SELECT depuis wallet.wallet_transactions.
	// message_id est nullable (uuid) → scanné comme *string via db tag.
	// La colonne est "kind" (pas "type") — conforme migration 0002.
	var rows []walletTransactionRow
	if err := s.DB.Select(ctx, &rows,
		`SELECT id, workspace_id, kind, amount, message_id::text, created_at
		   FROM wallet.wallet_transactions
		  WHERE workspace_id = $1
		  ORDER BY created_at DESC
		  LIMIT $2 OFFSET $3`,
		workspaceID, limit, offset,
	); err != nil {
		s.Logger.Error("wallet: list transactions", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 4. Garantir un tableau JSON non-null même si vide.
	if rows == nil {
		rows = []walletTransactionRow{}
	}

	writeJSON(w, http.StatusOK, rows)
}
