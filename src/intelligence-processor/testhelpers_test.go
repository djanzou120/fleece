package intelligenceprocessor

// testhelpers_test.go — fixtures et helpers partages entre les fichiers de
// test de ce package (evite la duplication de constantes/donnees entre
// consumer_test.go, delivery_outcome_test.go, on_message_*_test.go,
// campaign_scheduler_test.go, on_campaign_run_test.go).

import (
	"database/sql/driver"
	"strings"
	"time"
)

const (
	testMessageID   = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	testWorkspaceID = "550e8400-e29b-41d4-a716-446655440000"
	testCampaignID  = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
	testRecipient   = "+237699112233"
	testChannelFix  = "sms"
)

// testMessageCreatedAt est une date d'ENVOI fixe et DISTINCTE de "aujourd'hui"
// (utilisee pour verifier que `day` derive toujours de created_at, jamais de
// time.Now() — voir analytics_aggregate.go et delivery_outcome.go).
var testMessageCreatedAt = time.Date(2024, time.March, 3, 8, 0, 0, 0, time.UTC)

// messageRowFixture construit un fakeRows a une ligne pour selectMessageSQL
// (messaging.messages), colonnes dans l'ordre exact de messageRow/selectMessageSQL :
// workspace_id, recipient, channel, cost, created_at.
func messageRowFixture() *fakeRows {
	return fakeRowsOfCols(
		[]string{"workspace_id", "recipient", "channel", "cost", "created_at"},
		[]driver.Value{testWorkspaceID, testRecipient, testChannelFix, int64(10), testMessageCreatedAt},
	)
}

// messageRowFixtureWithCost construit la meme fixture qu'au-dessus avec un
// cout et un created_at personnalises (cas ou le test doit controler
// explicitement ces deux valeurs, ex. cost NULL ou une date d'envoi precise).
func messageRowFixtureWithCost(cost driver.Value, createdAt time.Time) *fakeRows {
	return fakeRowsOfCols(
		[]string{"workspace_id", "recipient", "channel", "cost", "created_at"},
		[]driver.Value{testWorkspaceID, testRecipient, testChannelFix, cost, createdAt},
	)
}

// messageRowFixtureWithChannel construit la meme fixture que messageRowFixture
// avec une valeur de channel personnalisee (ex. nil pour simuler une ligne
// heritee avec channel NULL — voir message_row.go, correctif E2).
func messageRowFixtureWithChannel(channel driver.Value) *fakeRows {
	return fakeRowsOfCols(
		[]string{"workspace_id", "recipient", "channel", "cost", "created_at"},
		[]driver.Value{testWorkspaceID, testRecipient, channel, int64(10), testMessageCreatedAt},
	)
}

// findQueryContaining retourne l'index (chronologique, cf. fakeConn.queries)
// de la PREMIERE requete emise contenant substr, et l'argument list associe.
// Utilise pour verifier des proprietes sur une requete precise au sein d'une
// sequence Exec/Query melangee (ex. la ligne INSERT analytics.message_daily
// au milieu d'une transaction), sans dependre de sa position exacte dans
// querySteps/execSteps.
func findQueryContaining(conn *fakeConn, substr string) (idx int, args []driver.NamedValue, found bool) {
	for i, q := range conn.queries {
		if strings.Contains(q, substr) {
			return i, conn.args[i], true
		}
	}
	return -1, nil, false
}

// countQueriesContaining compte le nombre de requetes emises (Exec ou Query,
// chronologique) contenant substr.
func countQueriesContaining(conn *fakeConn, substr string) int {
	n := 0
	for _, q := range conn.queries {
		if strings.Contains(q, substr) {
			n++
		}
	}
	return n
}
