package intelligenceprocessor

// analytics_aggregate.go — UPSERT additif de analytics.message_daily, partage
// par on_message_sent.go (hors transaction), on_message_delivered.go et
// on_message_failed.go (DANS la transaction unique, via delivery_outcome.go).
//
// Colonnes exactes (verifiees dans migrations/0009_analytics.sql +
// migrations/0017_analytics_kpi.sql avant d'ecrire ce fichier) :
//
//	analytics.message_daily(day, workspace_id, country, channel,
//	  sent, delivered, failed, cost, delivery_latency_ms_sum)
//	PRIMARY KEY (day, workspace_id, country, channel)
//
// REPARTITION DES COMPTEURS (impose par le PM, evite le double-comptage,
// vigilance T-019) :
//
//	sent, cost                        -> on_message_sent.go UNIQUEMENT
//	delivered, delivery_latency_ms_sum -> on_message_delivered.go UNIQUEMENT
//	failed                              -> on_message_failed.go UNIQUEMENT
//
// day = created_at::date (le jour d'ENVOI, jamais le jour du DLR) : un DLR
// tardif (ex. Telegram qui repond plusieurs heures apres l'envoi) ne doit
// jamais faire apparaitre un "delivered" sur un jour different de celui ou le
// "sent" correspondant a ete compte — sinon la serie temporelle
// sent/delivered/failed d'un meme jour ne serait plus coherente (ex.
// delivery_rate > 100% un jour donne si des delivered "migrent" vers le jour
// suivant). day est donc TOUJOURS derive de messaging.messages.created_at
// (relu par message_id), jamais de time.Now() au moment du traitement de
// l'evenement.
import (
	"context"
	"time"

	gosql "fleece/src/go/sql"
)

// Garde de non-regression : *gosql.DB et *gosql.Tx doivent satisfaire
// analyticsExecer.
var (
	_ analyticsExecer = (*gosql.DB)(nil)
	_ analyticsExecer = (*gosql.Tx)(nil)
)

// analyticsExecer est satisfaite par *gosql.DB (on_message_sent.go, hors
// transaction) et *gosql.Tx (delivery_outcome.go, DANS la transaction unique)
// — permet de partager upsertMessageDaily sans dupliquer le SQL.
type analyticsExecer interface {
	Exec(ctx context.Context, query string, args ...interface{}) error
}

// upsertMessageDailySQL est LA requete d'agregation additive. Les deltas
// sentDelta/deliveredDelta/failedDelta/costDelta/latencyDelta valent 0 pour
// tout compteur non concerne par l'appelant (voir repartition ci-dessus) —
// aucun appelant n'incremente jamais deux familles de compteurs a la fois.
const upsertMessageDailySQL = `
	INSERT INTO analytics.message_daily
	       (day, workspace_id, country, channel, sent, delivered, failed, cost, delivery_latency_ms_sum)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT (day, workspace_id, country, channel) DO UPDATE
	  SET sent                     = analytics.message_daily.sent + EXCLUDED.sent,
	      delivered                = analytics.message_daily.delivered + EXCLUDED.delivered,
	      failed                   = analytics.message_daily.failed + EXCLUDED.failed,
	      cost                     = analytics.message_daily.cost + EXCLUDED.cost,
	      delivery_latency_ms_sum  = analytics.message_daily.delivery_latency_ms_sum + EXCLUDED.delivery_latency_ms_sum`

// upsertMessageDaily applique un delta additif sur la ligne
// (day, workspace_id, country, channel) de analytics.message_daily. day est
// formate en "YYYY-MM-DD" (colonne SQL `date`) — voir doc de tete de fichier
// pour la semantique de day (toujours la date d'ENVOI).
func upsertMessageDaily(
	ctx context.Context,
	ex analyticsExecer,
	day time.Time,
	workspaceID, country, channel string,
	sentDelta, deliveredDelta, failedDelta, costDelta, latencyDelta int64,
) error {
	return ex.Exec(ctx, upsertMessageDailySQL,
		day.UTC().Format("2006-01-02"), workspaceID, country, channel,
		sentDelta, deliveredDelta, failedDelta, costDelta, latencyDelta,
	)
}

// deliveryLatencyMs calcule la latence de livraison en millisecondes entre
// l'envoi (createdAt, relu depuis messaging.messages) et l'instant de
// traitement de l'evenement message.delivered (now). Fonction pure, testable
// isolement. Bornee a >= 0 (une horloge legerement desynchronisee entre
// composants, ou un now() < createdAt du fait d'une correction d'horloge, ne
// doit jamais produire une somme negative dans delivery_latency_ms_sum — la
// MV analytics.kpi_daily n'a aucune protection contre une somme negative, et
// une latence negative n'a de toute facon aucun sens metier).
func deliveryLatencyMs(createdAt, now time.Time) int64 {
	d := now.Sub(createdAt).Milliseconds()
	if d < 0 {
		return 0
	}
	return d
}
