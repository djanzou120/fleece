package intelligenceprocessor

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
)

// campaign_correlation_test.go — teste correlateCampaignRecipient isolement
// de processDeliveryOutcome (deja couvert par delivery_outcome_test.go pour
// le cas "touched"/"not touched" a travers la transaction complete).

func TestCorrelateCampaignRecipient_touched(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"campaign_id"}, []driver.Value{testCampaignID})},
	}}
	db := newFakeGosqlDB(t, conn)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	touched, campaignID, err := correlateCampaignRecipient(context.Background(), tx, testMessageID, "delivered")
	if err != nil {
		t.Fatalf("correlateCampaignRecipient() erreur inattendue : %v", err)
	}
	if !touched {
		t.Fatal("touched = false, voulu true")
	}
	if campaignID != testCampaignID {
		t.Errorf("campaignID = %q, voulu %q", campaignID, testCampaignID)
	}
}

func TestCorrelateCampaignRecipient_notTouched_noRows(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"campaign_id"})}, // 0 ligne : pas une campagne, ou deja terminal
	}}
	db := newFakeGosqlDB(t, conn)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	touched, campaignID, err := correlateCampaignRecipient(context.Background(), tx, testMessageID, "failed")
	if err != nil {
		t.Fatalf("correlateCampaignRecipient() erreur inattendue (0 ligne n'est pas une erreur) : %v", err)
	}
	if touched {
		t.Fatal("touched = true, voulu false")
	}
	if campaignID != "" {
		t.Errorf("campaignID = %q, voulu vide", campaignID)
	}
}

func TestCorrelateCampaignRecipient_dbError(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{{err: errors.New("connexion perdue")}}}
	db := newFakeGosqlDB(t, conn)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, _, err = correlateCampaignRecipient(context.Background(), tx, testMessageID, "delivered")
	if err == nil {
		t.Fatal("correlateCampaignRecipient() attendu en erreur")
	}
}

// TestCorrelateCampaignRecipient_idempotenceGuard_realIdempotence verifie que
// la garde SQL "status NOT IN ('delivered', 'failed')" est bien presente dans
// la requete emise -- CONTRAIREMENT a la limite D-M25 de
// processDeliveryOutcome, cette correlation EST reellement idempotente (voir
// doc de tete de fichier campaign_correlation.go).
func TestCorrelateCampaignRecipient_idempotenceGuard_present(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"campaign_id"}, []driver.Value{testCampaignID})},
	}}
	db := newFakeGosqlDB(t, conn)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, _, err := correlateCampaignRecipient(context.Background(), tx, testMessageID, "delivered"); err != nil {
		t.Fatalf("correlateCampaignRecipient() erreur inattendue : %v", err)
	}
	if got := conn.lastQuery(); !strings.Contains(got, "status NOT IN ('delivered', 'failed')") {
		t.Errorf("garde d'idempotence absente de la requete : %q", got)
	}
}
