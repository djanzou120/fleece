package api

// analytics_get_kpis.go — GET /analytics/kpis?workspace_id=&from=&to=&channel=&country=
//
// Retourne les KPIs quotidiens de la vue materialisee analytics.kpi_daily.
//
// Parametres :
//   - workspace_id (requis, UUID) : filtre obligatoire pour l'isolation D11.
//   - from (optionnel, format 2006-01-02) : debut de la periode.
//     Defaut : 30 jours avant today.
//   - to (optionnel, format 2006-01-02) : fin de la periode.
//     Defaut : today.
//   - channel (optionnel) : filtre sur la colonne channel de kpi_daily.
//   - country (optionnel) : filtre sur la colonne country de kpi_daily.
//
// Gestion de avg_delivery_ms :
//   La colonne est NULLABLE dans kpi_daily (NULL si delivered=0 pour ce jour/dimension).
//   Elle est scannee en *float64 et serialisee en JSON :
//     - NULL Postgres → null JSON (omitempty non utilise : on expose null explicitement
//       pour permettre aux clients de distinguer "pas de livraison" vs "latence = 0").
//   Mapping numeric(16,3) → float64 : lib/pq supporte nativement ce mapping.
//
// Isolation D11 : touche UNIQUEMENT le schema analytics. Aucun acces cross-schema.

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// kpiRow represente une ligne de la vue materialisee analytics.kpi_daily.
// Les colonnes numeric sont mappees en float64 via lib/pq (compatible numeric(10,6) et
// numeric(16,3)). avg_delivery_ms est *float64 pour refleter la nullabilite SQL
// (NULL si delivered=0 pour cette dimension/jour).
type kpiRow struct {
	Day           time.Time `db:"day"             json:"day"`
	WorkspaceID   string    `db:"workspace_id"    json:"workspace_id"`
	Country       string    `db:"country"         json:"country"`
	Channel       string    `db:"channel"         json:"channel"`
	Sent          int64     `db:"sent"            json:"sent"`
	Delivered     int64     `db:"delivered"       json:"delivered"`
	Failed        int64     `db:"failed"          json:"failed"`
	Cost          int64     `db:"cost"            json:"cost"`
	DeliveryRate  float64   `db:"delivery_rate"   json:"delivery_rate"`
	FailureRate   float64   `db:"failure_rate"    json:"failure_rate"`
	AvgCostPerMsg float64   `db:"avg_cost_per_msg" json:"avg_cost_per_msg"`
	AvgDeliveryMs *float64  `db:"avg_delivery_ms" json:"avg_delivery_ms"`
}

// HandleGetKpis traite GET /analytics/kpis
//
// Pipeline :
//  1. Valider workspace_id (requis, UUID).
//  2. Parser from/to (dates "2006-01-02") ; defaut : [today-30j, today].
//  3. Filtres optionnels : channel, country.
//  4. Requete SELECT sur analytics.kpi_daily avec filtres dynamiques.
//  5. 200 + tableau JSON (tableau vide si aucun resultat).
func (s *Service) HandleGetKpis(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	// 1. workspace_id requis.
	workspaceID := q.Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id est requis")
		return
	}
	if _, err := parseUUID(workspaceID); err != nil {
		writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
		return
	}

	// 2. Dates from/to (format 2006-01-02). Defaut : 30 derniers jours.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	fromDate := today.AddDate(0, 0, -30)
	toDate := today

	if raw := q.Get("from"); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "from : format de date invalide, attendu YYYY-MM-DD")
			return
		}
		fromDate = t
	}
	if raw := q.Get("to"); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "to : format de date invalide, attendu YYYY-MM-DD")
			return
		}
		toDate = t
	}

	if fromDate.After(toDate) {
		writeError(w, http.StatusBadRequest, "from doit etre anterieur ou egal a to")
		return
	}

	// 3. Filtres optionnels.
	channel := q.Get("channel")
	country := q.Get("country")

	// 4. Construction de la requete avec filtres dynamiques.
	query, args := buildKpiQuery(workspaceID, fromDate, toDate, channel, country)

	var rows []kpiRow
	if err := s.DB.Select(ctx, &rows, query, args...); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.Logger.Error("analytics/kpis: select", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
	}

	// 5. Retourner un tableau vide plutot que null si aucun resultat.
	if rows == nil {
		rows = []kpiRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// buildKpiQuery construit la requete SELECT sur analytics.kpi_daily avec les filtres optionnels.
// workspace_id, from et to sont toujours presents. channel et country sont optionnels.
func buildKpiQuery(workspaceID string, from, to time.Time, channel, country string) (string, []any) {
	base := `SELECT day, workspace_id, country, channel, sent, delivered, failed, cost,
	                delivery_rate, failure_rate, avg_cost_per_msg, avg_delivery_ms
	           FROM analytics.kpi_daily
	          WHERE workspace_id = $1 AND day BETWEEN $2 AND $3`

	args := []any{workspaceID, from.Format("2006-01-02"), to.Format("2006-01-02")}
	idx := 4 // prochain placeholder apres $1, $2, $3

	if channel != "" {
		base += fmt.Sprintf(" AND channel = $%d", idx)
		args = append(args, channel)
		idx++
	}
	if country != "" {
		base += fmt.Sprintf(" AND country = $%d", idx)
		args = append(args, country)
		idx++ //nolint:ineffassign // conserver le pattern pour extensibilite
	}

	base += " ORDER BY day DESC"
	return base, args
}
