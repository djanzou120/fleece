package intelligenceprocessor

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
)

// delivery_outcome_test.go — teste processDeliveryOutcome (mecanique COMMUNE
// a on_message_delivered.go/on_message_failed.go : transaction unique,
// lecture autonome, idempotence partielle documentee D-M25). Les tests
// specifiques a chaque routing key (message_id invalide, garde anti-wallet,
// etc.) vivent dans on_message_delivered_test.go/on_message_failed_test.go.

// deliveredPipelineQuerySteps/execSteps construisent la sequence complete
// attendue par processDeliveryOutcome pour delivered=true, "nouveau contact"
// (aucun score existant, pas de correlation campagne) :
//
//	Query #1 : SELECT messaging.messages (loadMessage)
//	Query #2 : SELECT contact_channel_scores (upsertChannelScore, 0 ligne)
//	Exec  #1 : INSERT contact_channel_scores (upsertChannelScore)
//	Exec  #2 : INSERT contacts (upsertContactCounters)
//	Exec  #3 : INSERT analytics.message_daily (upsertMessageDaily)
//	Query #3 : UPDATE campaign_recipients ... RETURNING campaign_id (0 ligne, pas de campagne)
func newContactFixtureConn(createdAt driver.Value) *fakeConn {
	return &fakeConn{
		querySteps: []queryStep{
			{rows: messageRowFixtureWithCost(int64(10), testMessageCreatedAt)},
			{rows: fakeRowsOfCols([]string{"score", "sample_size"})},
			{rows: fakeRowsOfCols([]string{"campaign_id"})},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}},
			{result: fakeExecResult{rows: 1}},
			{result: fakeExecResult{rows: 1}},
		},
	}
}

func TestProcessDeliveryOutcome_nominal_delivered_fullTransactionShape(t *testing.T) {
	conn := newContactFixtureConn(testMessageCreatedAt)
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := processDeliveryOutcome(context.Background(), svc, evt, true); err != nil {
		t.Fatalf("processDeliveryOutcome(delivered) erreur inattendue : %v", err)
	}

	if conn.beginCount != 1 {
		t.Fatalf("beginCount = %d, voulu 1 (une seule transaction)", conn.beginCount)
	}
	if conn.commitCount != 1 {
		t.Fatalf("commitCount = %d, voulu 1", conn.commitCount)
	}
	if len(conn.queries) != 6 {
		t.Fatalf("nombre total de requetes = %d, voulu 6 : %+v", len(conn.queries), conn.queries)
	}

	if !strings.Contains(conn.queries[0], "messaging.messages") {
		t.Errorf("requete #1 inattendue : %q", conn.queries[0])
	}
	if !strings.Contains(conn.queries[1], "contact_intel.contact_channel_scores") {
		t.Errorf("requete #2 inattendue : %q", conn.queries[1])
	}

	idx, args, found := findQueryContaining(conn, "analytics.message_daily")
	if !found {
		t.Fatal("aucune requete analytics.message_daily emise")
	}
	_ = idx
	if got := args[5].Value; got != int64(1) { // deliveredDelta
		t.Errorf("deliveredDelta = %v, voulu 1", got)
	}
	if got := args[4].Value; got != int64(0) { // sentDelta -- JAMAIS touche ici
		t.Errorf("sentDelta = %v, voulu 0 (ce handler n'incremente jamais sent)", got)
	}
	if got := args[6].Value; got != int64(0) { // failedDelta
		t.Errorf("failedDelta = %v, voulu 0", got)
	}
	if got := args[8].Value.(int64); got < 0 {
		t.Errorf("delivery_latency_ms_sum = %v, jamais negatif", got)
	}
}

func TestProcessDeliveryOutcome_nominal_failed_fullTransactionShape(t *testing.T) {
	conn := newContactFixtureConn(testMessageCreatedAt)
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := processDeliveryOutcome(context.Background(), svc, evt, false); err != nil {
		t.Fatalf("processDeliveryOutcome(failed) erreur inattendue : %v", err)
	}

	_, args, found := findQueryContaining(conn, "analytics.message_daily")
	if !found {
		t.Fatal("aucune requete analytics.message_daily emise")
	}
	if got := args[6].Value; got != int64(1) { // failedDelta
		t.Errorf("failedDelta = %v, voulu 1", got)
	}
	if got := args[5].Value; got != int64(0) { // deliveredDelta -- JAMAIS touche
		t.Errorf("deliveredDelta = %v, voulu 0 (message.failed n'incremente jamais delivered)", got)
	}
	if got := args[8].Value; got != int64(0) { // latence -- jamais pour un echec
		t.Errorf("delivery_latency_ms_sum = %v, voulu 0 (pas de latence sur un echec)", got)
	}
}

// TestProcessDeliveryOutcome_dayIsSendDate_notProcessingDate verifie que
// `day` derive TOUJOURS de messaging.messages.created_at (jour d'ENVOI),
// jamais de l'instant de traitement du DLR (voir analytics_aggregate.go).
func TestProcessDeliveryOutcome_dayIsSendDate_notProcessingDate(t *testing.T) {
	conn := newContactFixtureConn(testMessageCreatedAt)
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := processDeliveryOutcome(context.Background(), svc, evt, true); err != nil {
		t.Fatalf("processDeliveryOutcome() erreur inattendue : %v", err)
	}

	_, args, found := findQueryContaining(conn, "analytics.message_daily")
	if !found {
		t.Fatal("aucune requete analytics.message_daily emise")
	}
	if got := args[0].Value; got != "2024-03-03" {
		t.Errorf("day = %v, voulu %q (jour d'ENVOI, testMessageCreatedAt), jamais le jour du traitement", got, "2024-03-03")
	}
}

// TestProcessDeliveryOutcome_messageNotFound_permanentRejection_committed
// verifie le rejet permanent (message inconnu) : le SELECT ne trouve rien ->
// commit (rien a annuler) puis permanentError -> pas de requeue.
func TestProcessDeliveryOutcome_messageNotFound_permanentRejection_committed(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id", "recipient", "channel", "cost", "created_at"})}, // 0 ligne
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	err := processDeliveryOutcome(context.Background(), svc, evt, true)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("processDeliveryOutcome() sur message introuvable = %v, voulu une permanentError", err)
	}
	if conn.commitCount != 1 {
		t.Errorf("commitCount = %d, voulu 1 (rien a annuler, commit propre malgre le rejet)", conn.commitCount)
	}
	if len(conn.queries) != 1 {
		t.Errorf("nombre de requetes = %d, voulu 1 (aucune ecriture tentee sur message inconnu)", len(conn.queries))
	}
}

// TestProcessDeliveryOutcome_dbErrorOnLoad_rollback verifie qu'une panne
// technique sur la lecture autonome remonte une erreur "nue" et NE COMMIT
// JAMAIS.
func TestProcessDeliveryOutcome_dbErrorOnLoad_rollback(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	err := processDeliveryOutcome(context.Background(), svc, evt, true)
	if err == nil {
		t.Fatal("processDeliveryOutcome() attendu en erreur sur panne DB")
	}
	if isPermanentError(err) {
		t.Errorf("erreur DB classee permanente a tort : %v", err)
	}
	if conn.commitCount != 0 {
		t.Errorf("commitCount = %d, voulu 0 (aucun commit sur erreur)", conn.commitCount)
	}
}

// TestProcessDeliveryOutcome_dbErrorMidTransaction_rollback verifie qu'une
// panne survenant APRES la lecture autonome (ex. pendant l'upsert contact
// intelligence) abandonne toute la transaction : AUCUN commit partiel,
// meme si le SELECT initial a reussi.
func TestProcessDeliveryOutcome_dbErrorMidTransaction_rollback(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: messageRowFixtureWithCost(int64(10), testMessageCreatedAt)},
			{err: errors.New("connexion perdue pendant l'upsert contact_intel")},
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	err := processDeliveryOutcome(context.Background(), svc, evt, true)
	if err == nil {
		t.Fatal("processDeliveryOutcome() attendu en erreur")
	}
	if isPermanentError(err) {
		t.Errorf("erreur DB classee permanente a tort : %v", err)
	}
	if conn.commitCount != 0 {
		t.Errorf("commitCount = %d, voulu 0 (aucun commit partiel)", conn.commitCount)
	}
	// La lecture (messaging.messages) a bien eu lieu, mais aucune ecriture
	// (contact_intel/analytics/campaign) n'a jamais ete commitee.
	if conn.execPos != 0 {
		t.Errorf("execPos = %d, voulu 0 (aucun Exec commite avant l'erreur)", conn.execPos)
	}
}

// TestProcessDeliveryOutcome_channelNull_usesUnknown couvre la regression E2
// dans le pipeline transactionnel (delivery_outcome.go) : channel NULL en
// base ne doit jamais faire echouer loadMessage (scan sql.NullString) ni
// propager une chaine vide vers contact_intel/analytics (colonnes NOT NULL,
// membres de cle primaire) -- repli sur unknownChannel ("unknown").
func TestProcessDeliveryOutcome_channelNull_usesUnknown(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: messageRowFixtureWithChannel(nil)},
			{rows: fakeRowsOfCols([]string{"score", "sample_size"})},
			{rows: fakeRowsOfCols([]string{"campaign_id"})},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}},
			{result: fakeExecResult{rows: 1}},
			{result: fakeExecResult{rows: 1}},
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := processDeliveryOutcome(context.Background(), svc, evt, true); err != nil {
		t.Fatalf("processDeliveryOutcome() erreur inattendue (channel NULL, regression E2) : %v", err)
	}

	if _, args, found := findQueryContaining(conn, "contact_intel.contact_channel_scores"); !found {
		t.Fatal("aucune requete contact_channel_scores emise")
	} else if got := args[1].Value; got != unknownChannel {
		t.Errorf("channel (contact_channel_scores) = %v, voulu %q", got, unknownChannel)
	}

	if _, args, found := findQueryContaining(conn, "analytics.message_daily"); !found {
		t.Fatal("aucune requete analytics.message_daily emise")
	} else if got := args[3].Value; got != unknownChannel {
		t.Errorf("channel (analytics.message_daily) = %v, voulu %q", got, unknownChannel)
	}
}

// TestProcessDeliveryOutcome_campaignCorrelation_touched verifie que le
// destinataire de campagne est bien correle quand campaign_recipients
// retourne une ligne (message issu d'une campagne).
func TestProcessDeliveryOutcome_campaignCorrelation_touched(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: messageRowFixtureWithCost(int64(10), testMessageCreatedAt)},
			{rows: fakeRowsOfCols([]string{"score", "sample_size"})},
			{rows: fakeRowsOfCols([]string{"campaign_id"}, []driver.Value{testCampaignID})},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}},
			{result: fakeExecResult{rows: 1}},
			{result: fakeExecResult{rows: 1}},
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := processDeliveryOutcome(context.Background(), svc, evt, true); err != nil {
		t.Fatalf("processDeliveryOutcome() erreur inattendue : %v", err)
	}
	if _, args, found := findQueryContaining(conn, "campaign.campaign_recipients"); !found {
		t.Error("aucune requete campaign_recipients emise")
	} else if args[0].Value != "delivered" {
		t.Errorf("statut cible inattendu : %v, voulu %q", args[0].Value, "delivered")
	}
}

// TestProcessDeliveryOutcome_campaignCorrelation_notTouched verifie le cas
// (normal, majoritaire) ou le message n'est pas issu d'une campagne : 0 ligne
// RETURNING, pas d'erreur, pas de log "correle" (non verifiable directement,
// mais le chemin ne doit jamais echouer).
func TestProcessDeliveryOutcome_campaignCorrelation_notTouched(t *testing.T) {
	conn := newContactFixtureConn(testMessageCreatedAt)
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := processDeliveryOutcome(context.Background(), svc, evt, true); err != nil {
		t.Fatalf("processDeliveryOutcome() erreur inattendue : %v", err)
	}
	if conn.commitCount != 1 {
		t.Errorf("commitCount = %d, voulu 1", conn.commitCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// DETTE D-M25 (documentee dans delivery_outcome.go, §Idempotence) : ce test
// REFLETE LE COMPORTEMENT REEL du worker, PAS un comportement ideal. Un
// redelivery AMQP d'un message.delivered/message.failed DEJA traite avec
// succes double-compte les compteurs additifs (contact_intel + analytics).
// Ne PAS "corriger" ce test pour le faire paraitre idempotent : c'est
// precisement la limite documentee que le PM doit tracer comme dette.
// ═══════════════════════════════════════════════════════════════════════════

// TestProcessDeliveryOutcome_redelivery_D25_doubleCompte documente que deux
// traitements successifs du MEME message.delivered (redelivery AMQP apres
// commit + perte d'Ack, cf. delivery_outcome.go) executent CHACUN une
// ecriture additive complete (2 upserts contact_intel, 2 upserts analytics) —
// AUCUNE garde de deduplication n'empeche ce double-comptage (contrairement a
// la correlation campagne, qui elle EST idempotente par construction).
func TestProcessDeliveryOutcome_redelivery_D25_doubleCompte(t *testing.T) {
	// Chaque "traitement" consomme sa propre sequence complete de query/exec
	// steps (rien n'est partage/deduplique entre les deux passages).
	querySteps := []queryStep{
		{rows: messageRowFixtureWithCost(int64(10), testMessageCreatedAt)},
		{rows: fakeRowsOfCols([]string{"score", "sample_size"})},
		{rows: fakeRowsOfCols([]string{"campaign_id"})},
		{rows: messageRowFixtureWithCost(int64(10), testMessageCreatedAt)},
		{rows: fakeRowsOfCols([]string{"score", "sample_size"}, []driver.Value{int64(90), int64(1)})}, // 2e passage : score deja ecrit par le 1er
		{rows: fakeRowsOfCols([]string{"campaign_id"})},
	}
	execSteps := []execStep{
		{result: fakeExecResult{rows: 1}}, {result: fakeExecResult{rows: 1}}, {result: fakeExecResult{rows: 1}},
		{result: fakeExecResult{rows: 1}}, {result: fakeExecResult{rows: 1}}, {result: fakeExecResult{rows: 1}},
	}
	conn := &fakeConn{querySteps: querySteps, execSteps: execSteps}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}

	// 1er traitement (reel).
	if err := processDeliveryOutcome(context.Background(), svc, evt, true); err != nil {
		t.Fatalf("processDeliveryOutcome() 1er traitement erreur inattendue : %v", err)
	}
	// 2e traitement (redelivery AMQP du MEME evenement, apres un Ack perdu) :
	// le code REUSSIT sans erreur ET reapplique integralement les deltas
	// additifs -- c'est la limite D-M25, pas un bug de ce test.
	if err := processDeliveryOutcome(context.Background(), svc, evt, true); err != nil {
		t.Fatalf("processDeliveryOutcome() 2e traitement (redelivery) erreur inattendue : %v", err)
	}

	if conn.commitCount != 2 {
		t.Fatalf("commitCount = %d, voulu 2 (D-M25 : chaque redelivery est CHACUNE commitee integralement, aucune garde ne bloque le 2e passage)", conn.commitCount)
	}
	if countQueriesContaining(conn, "analytics.message_daily") != 2 {
		t.Errorf("nombre d'UPSERT analytics.message_daily = %d, voulu 2 (D-M25 : double-comptage reel, pas de deduplication)", countQueriesContaining(conn, "analytics.message_daily"))
	}
	if countQueriesContaining(conn, "contact_intel.contacts") != 2 {
		t.Errorf("nombre d'UPSERT contact_intel.contacts = %d, voulu 2 (D-M25 : double-comptage reel)", countQueriesContaining(conn, "contact_intel.contacts"))
	}
}
