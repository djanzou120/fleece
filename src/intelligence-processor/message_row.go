package intelligenceprocessor

// message_row.go — lecture AUTONOME de messaging.messages par message_id,
// partagee par on_message_sent.go (hors transaction) et
// on_message_delivered.go/on_message_failed.go (DANS la transaction unique
// imposee par le plan M-021, via delivery_outcome.go).
//
// Principe d'autonomie (voir consumer.go) : le payload AMQP n'apporte que
// message_id. Toutes les donnees d'agregation (workspace_id, recipient,
// channel, cost, created_at) sont TOUJOURS relues ici depuis
// messaging.messages — jamais depuis le payload — pour qu'un evenement
// rejoue ou force ne puisse jamais fausser une agregation.
import (
	"context"
	"database/sql"
	"time"

	gosql "fleece/src/go/sql"
)

// Garde de non-regression : *gosql.DB et *gosql.Tx doivent satisfaire
// dbGetter (utilises respectivement par on_message_sent.go, hors
// transaction, et delivery_outcome.go, dans la transaction unique).
var (
	_ dbGetter = (*gosql.DB)(nil)
	_ dbGetter = (*gosql.Tx)(nil)
)

// messageRow est le resultat de selectMessageSQL. Colonnes issues des
// migrations 0003 (messaging) + 0018 (cost, nullable — provider gratuit ou
// echec precoce, voir src/core-processor/on_message_failed.go).
//
// Channel est sql.NullString (E2, correctif Phase 3) : messaging.messages.channel
// est NULLABLE (migration 0003_messaging.sql : "channel text", sans NOT NULL —
// contrairement a recipient/content/status/created_at, tous NOT NULL). Un
// scan direct dans un string plante des qu'une ligne heritee (ex. donnee
// legacy sans channel connu) est lue : erreur classee transitoire par les
// handlers appelants -> Nack(requeue=true) -> requeue INFINIE sans backoff ni
// DLQ (aucune topologie DLQ/backoff n'existe a ce jour, voir prefetch ajoute
// a src/go/amqp/conn.go pour borner l'impact — DLQ/backoff reels = dette
// D-M28, hors perimetre). Voir channelOrUnknown() pour la valeur de repli
// utilisee par les appelants.
type messageRow struct {
	WorkspaceID string         `db:"workspace_id"`
	Recipient   string         `db:"recipient"`
	Channel     sql.NullString `db:"channel"`
	Cost        sql.NullInt64  `db:"cost"`
	CreatedAt   time.Time      `db:"created_at"`
}

// unknownChannel est la valeur de repli utilisee quand Channel est NULL ou
// vide en base. MEME CONVENTION que unknownCountry (country.go, "unknown") :
// analytics.message_daily.channel et contact_intel.contact_channel_scores.channel/
// contact_intel.contacts.last_channel sont tous NOT NULL et channel fait partie
// de la cle primaire de message_daily et de contact_channel_scores (voir
// migrations 0009_analytics.sql / 0014_contact_intel_scoring.sql) — une chaine
// vide y romprait la lisibilite des agregats/scores exactement pour la meme
// raison que documentee dans country.go : une ligne "channel = ”" est
// indiscernable d'un bug de troncature, alors que NOT NULL l'accepterait
// silencieusement (contrainte SQL non violee par une chaine vide).
const unknownChannel = "unknown"

// channelOrUnknown retourne Channel si non-NULL et non-vide, sinon
// unknownChannel. Utilise par tous les appelants (on_message_sent.go,
// delivery_outcome.go, contact_score.go via delivery_outcome.go) au lieu
// d'acceder au champ Channel directement — evite qu'un futur appelant
// re-introduise le bug de scan NULL->"" implicite ou une panique de scan.
func (r messageRow) channelOrUnknown() string {
	if r.Channel.Valid && r.Channel.String != "" {
		return r.Channel.String
	}
	return unknownChannel
}

// selectMessageSQL est LA requete de lecture autonome, utilisee a l'identique
// par les trois handlers message.* (evite une divergence accidentelle du SQL
// entre on_message_sent.go et delivery_outcome.go).
const selectMessageSQL = `SELECT workspace_id, recipient, channel, cost, created_at FROM messaging.messages WHERE id = $1`

// dbGetter est satisfaite par *gosql.DB (on_message_sent.go, hors
// transaction) et *gosql.Tx (delivery_outcome.go, DANS la transaction unique)
// — permet de partager loadMessage sans dupliquer le SQL ni imposer une
// transaction la ou elle n'est pas exigee par le plan.
type dbGetter interface {
	Get(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

// loadMessage lit un message par son id. Retourne sql.ErrNoRows si l'id est
// inconnu (message forge/rejoue) — les appelants traitent ce cas comme un
// rejet permanent (voir on_message_sent.go/delivery_outcome.go).
func loadMessage(ctx context.Context, g dbGetter, messageID string) (messageRow, error) {
	var row messageRow
	err := g.Get(ctx, &row, selectMessageSQL, messageID)
	return row, err
}
