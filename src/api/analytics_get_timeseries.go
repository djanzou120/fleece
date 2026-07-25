package api

// analytics_get_timeseries.go — GET /analytics/timeseries?workspace_id=&from=&to=&channel=
//
// Retourne une serie temporelle quotidienne agregee depuis analytics.message_daily.
//
// Choix timeseries (brut vs agrege) :
//   La table message_daily a une granularite par (day, workspace_id, country, channel).
//   Sans filtre country, plusieurs lignes par jour existent pour le meme workspace.
//   Ce handler retourne des lignes AGREGEES par jour (SUM par dimension temporelle),
//   ce qui produit une vraie timeseries 1 point/jour, directement exploitable par
//   un graphique de dashboard. Un filtre channel optionnel permet de restreindre
//   au canal voulu avant l'agregation.
//   Si le client veut la granularite par (country, channel), il doit utiliser
//   GET /analytics/kpis qui expose la vue kpi_daily (granulaire).
//
// Parametres :
//   - workspace_id (requis, UUID).
//   - from (optionnel, format 2006-01-02). Defaut : 30 jours avant today.
//   - to (optionnel, format 2006-01-02). Defaut : today.
//   - channel (optionnel) : filtre avant agregation.
//
// Isolation D11 : touche UNIQUEMENT le schema analytics. Aucun acces cross-schema.

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// timeseriesRow represente un point de la serie temporelle quotidienne.
// Les volumes sont des bigint agregees (SUM) par jour.
type timeseriesRow struct {
	Day            time.Time `db:"day"               json:"day"`
	SentCount      int64     `db:"sent_count"        json:"sent_count"`
	DeliveredCount int64     `db:"delivered_count"   json:"delivered_count"`
	FailedCount    int64     `db:"failed_count"      json:"failed_count"`
	TotalCostCents int64     `db:"total_cost_cents"  json:"total_cost_cents"`
}

// HandleGetTimeseries traite GET /analytics/timeseries
//
// Pipeline :
//  1. Valider workspace_id (requis, UUID).
//  2. Parser from/to (dates "2006-01-02") ; defaut : [today-30j, today].
//  3. Filtre optionnel : channel.
//  4. Requete SELECT + SUM/GROUP BY day sur analytics.message_daily.
//  5. 200 + tableau JSON [{day, sent_count, delivered_count, failed_count, total_cost_cents}].
func (s *Service) HandleGetTimeseries(w http.ResponseWriter, r *http.Request) {
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

	// 2. Dates from/to. Defaut : 30 derniers jours.
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

	// 3. Filtre channel optionnel.
	channel := q.Get("channel")

	// 4. Construction de la requete avec filtre dynamique.
	query, args := buildTimeseriesQuery(workspaceID, fromDate, toDate, channel)

	var rows []timeseriesRow
	if err := s.DB.Select(ctx, &rows, query, args...); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.Logger.Error("analytics/timeseries: select", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
	}

	// 5. Retourner un tableau vide plutot que null si aucun resultat.
	if rows == nil {
		rows = []timeseriesRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// buildTimeseriesQuery construit la requete SELECT agregee sur analytics.message_daily.
// Retourne 1 ligne par jour (SUM sur toutes les dimensions country/channel du workspace)
// avec un filtre optionnel sur channel applique avant l'agregation.
func buildTimeseriesQuery(workspaceID string, from, to time.Time, channel string) (string, []any) {
	base := `SELECT day,
	                SUM(sent)      AS sent_count,
	                SUM(delivered) AS delivered_count,
	                SUM(failed)    AS failed_count,
	                SUM(cost)      AS total_cost_cents
	           FROM analytics.message_daily
	          WHERE workspace_id = $1 AND day BETWEEN $2 AND $3`

	args := []any{workspaceID, from.Format("2006-01-02"), to.Format("2006-01-02")}
	idx := 4

	if channel != "" {
		base += fmt.Sprintf(" AND channel = $%d", idx)
		args = append(args, channel)
		idx++ //nolint:ineffassign // conserver le pattern pour extensibilite
	}

	base += " GROUP BY day ORDER BY day ASC"
	return base, args
}
