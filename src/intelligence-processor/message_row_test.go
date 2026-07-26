package intelligenceprocessor

// message_row_test.go — teste channelOrUnknown (fonction pure) et couvre la
// regression E2 (Phase 3) : messaging.messages.channel est NULLABLE
// (migrations/0003_messaging.sql, colonne "channel text" sans NOT NULL) ; un
// scan direct dans un string plantait des qu'une ligne heritee sans channel
// connu etait lue par loadMessage, classee erreur transitoire par les
// handlers appelants -> Nack(requeue=true) -> boucle chaude sans fin.
//
// TestLoadMessage_channelNull_doesNotErrorOnScan est le test qui AURAIT
// echoue avant le correctif (Channel string -> erreur de scan database/sql
// "converting NULL to string is unsupported" des que la colonne channel
// valait NULL) : il prouve que loadMessage scanne desormais une ligne
// channel=NULL sans erreur technique.

import (
	"context"
	"database/sql"
	"testing"
)

func TestChannelOrUnknown(t *testing.T) {
	cases := []struct {
		name string
		row  messageRow
		want string
	}{
		{"channel valide -> valeur telle quelle", messageRow{Channel: sql.NullString{String: "sms", Valid: true}}, "sms"},
		{"channel NULL -> unknown", messageRow{Channel: sql.NullString{Valid: false}}, "unknown"},
		{"channel vide mais non-NULL -> unknown (meme convention que country.go)", messageRow{Channel: sql.NullString{String: "", Valid: true}}, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.row.channelOrUnknown(); got != tc.want {
				t.Errorf("channelOrUnknown() = %q, voulu %q", got, tc.want)
			}
		})
	}
}

// TestLoadMessage_channelNull_doesNotErrorOnScan verifie que loadMessage (le
// SELECT autonome partage par on_message_sent.go/delivery_outcome.go) scanne
// sans erreur une ligne messaging.messages dont channel est NULL — c'est LE
// test qui echouait (erreur de scan database/sql) avant le passage de
// messageRow.Channel a sql.NullString.
func TestLoadMessage_channelNull_doesNotErrorOnScan(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: messageRowFixtureWithChannel(nil)},
	}}
	db := newFakeGosqlDB(t, conn)

	row, err := loadMessage(context.Background(), db, testMessageID)
	if err != nil {
		t.Fatalf("loadMessage() erreur inattendue sur channel NULL (regression E2) : %v", err)
	}
	if row.Channel.Valid {
		t.Errorf("row.Channel.Valid = true, voulu false (channel NULL en base)")
	}
	if got := row.channelOrUnknown(); got != unknownChannel {
		t.Errorf("row.channelOrUnknown() = %q, voulu %q", got, unknownChannel)
	}
}
