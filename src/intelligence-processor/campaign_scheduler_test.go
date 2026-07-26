package intelligenceprocessor

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
)

// campaign_scheduler_test.go — teste runSchedulerTick/startCampaignRun.
// Critere de QA explicite M-022 : l'anti-double-run passe par UNE SEULE
// requete "UPDATE ... WHERE status='scheduled' AND scheduled_at <= now()
// RETURNING ..." -- jamais un SELECT separe suivi d'un UPDATE.

// TestRunSchedulerTick_noneDue verifie le cas 0 campagne due : une seule
// requete emise (l'UPDATE ... RETURNING lui-meme), rien d'autre.
func TestRunSchedulerTick_noneDue(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"id", "workspace_id", "message_body", "channel_strategy", "rate_limit"})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), AMQP: nil}

	svc.runSchedulerTick(context.Background())

	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 (aucune campagne due, aucun run demarre) : %+v", len(conn.queries), conn.queries)
	}
	assertSingleUpdateReturning(t, conn.queries[0])
}

// TestRunSchedulerTick_singleQuery_notSelectThenUpdate est LE test du critere
// de QA M-022 : verifie EXPLICITEMENT que la premiere (et dans ce cas
// unique tant qu'aucune campagne n'est due) requete emise est un UPDATE ...
// RETURNING, PAS un SELECT.
func TestRunSchedulerTick_singleQuery_notSelectThenUpdate(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"id", "workspace_id", "message_body", "channel_strategy", "rate_limit"})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.runSchedulerTick(context.Background())

	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes de decouverte des campagnes dues = %d, voulu EXACTEMENT 1 (pas de SELECT prealable)", len(conn.queries))
	}
	q := strings.ToUpper(strings.TrimSpace(conn.queries[0]))
	if strings.HasPrefix(q, "SELECT") {
		t.Fatalf("la premiere requete est un SELECT (%q) -- viole le critere M-022 (anti-double-run doit etre UNE SEULE requete UPDATE...RETURNING)", conn.queries[0])
	}
	if !strings.HasPrefix(q, "UPDATE") {
		t.Fatalf("la premiere requete devrait etre un UPDATE...RETURNING, obtenu : %q", conn.queries[0])
	}
}

// TestRunSchedulerTick_nDue verifie que chaque campagne capturee par l'UPDATE
// declenche bien un démarrage de run (count + insert campaign_runs), pour
// plusieurs campagnes (N=2) dans le meme tick.
func TestRunSchedulerTick_nDue(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		// UPDATE ... RETURNING -> 2 campagnes capturees.
		{rows: fakeRowsOfCols(
			[]string{"id", "workspace_id", "message_body", "channel_strategy", "rate_limit"},
			[]driver.Value{"campaign-1", testWorkspaceID, "Hello", "highest_delivery", int64(0)},
			[]driver.Value{"campaign-2", testWorkspaceID, "Hello2", "cheapest", int64(60)},
		)},
		// startCampaignRun(campaign-1) : count puis insert run.
		{rows: fakeRowsOf("count", int64(3))},
		{rows: fakeRowsOf("id", int64(101))},
		// startCampaignRun(campaign-2) : count puis insert run.
		{rows: fakeRowsOf("count", int64(5))},
		{rows: fakeRowsOf("id", int64(102))},
	}}
	// AMQP nil : publishCampaignRun retourne errPublisherUnavailable pour
	// chaque campagne -- le tick doit journaliser l'erreur et CONTINUER (ne
	// jamais interrompre le traitement des autres campagnes dues).
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), AMQP: nil}

	svc.runSchedulerTick(context.Background())

	if len(conn.queries) != 5 {
		t.Fatalf("nombre de requetes = %d, voulu 5 (1 UPDATE + 2x(count+insert)) : %+v", len(conn.queries), conn.queries)
	}
	assertSingleUpdateReturning(t, conn.queries[0])
	if !strings.Contains(conn.queries[1], "COUNT(*)") || !strings.Contains(conn.queries[1], "campaign.campaign_recipients") {
		t.Errorf("requete #2 (count campagne-1) inattendue : %q", conn.queries[1])
	}
	if !strings.Contains(conn.queries[2], "INSERT INTO campaign.campaign_runs") {
		t.Errorf("requete #3 (insert run campagne-1) inattendue : %q", conn.queries[2])
	}
	if !strings.Contains(conn.queries[3], "COUNT(*)") {
		t.Errorf("requete #4 (count campagne-2) inattendue : %q", conn.queries[3])
	}
	if !strings.Contains(conn.queries[4], "INSERT INTO campaign.campaign_runs") {
		t.Errorf("requete #5 (insert run campagne-2) inattendue : %q", conn.queries[4])
	}
}

// TestRunSchedulerTick_dbError_doesNotCrash verifie qu'une panne DB sur la
// requete d'anti-double-run est journalisee (ERROR) sans jamais paniquer --
// le tick suivant retentera (ticker toujours actif).
func TestRunSchedulerTick_dbError_doesNotCrash(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.runSchedulerTick(context.Background()) // ne doit pas paniquer
}

// TestStartCampaignRun_publishFails_logsButReturnsErr verifie qu'un echec de
// publication AMQP (ici : AMQP non configure) est bien remonte comme erreur
// par startCampaignRun (utile pour observabilite du tick), MAIS que le run a
// deja ete persiste en base (limite documentee dans campaign_scheduler.go).
func TestStartCampaignRun_publishFails_logsButReturnsErr(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOf("count", int64(1))},
		{rows: fakeRowsOf("id", int64(55))},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), AMQP: nil}

	row := scheduledCampaignRow{ID: testCampaignID, WorkspaceID: testWorkspaceID, MessageBody: "hi", ChannelStrategy: "cheapest", RateLimit: 0}
	err := svc.startCampaignRun(context.Background(), row)
	if err == nil || !errors.Is(err, errPublisherUnavailable) {
		t.Fatalf("startCampaignRun() = %v, voulu errPublisherUnavailable", err)
	}
	// La ligne campaign_runs a bien ete creee (2 requetes emises) malgre
	// l'echec de publication.
	if len(conn.queries) != 2 {
		t.Fatalf("nombre de requetes = %d, voulu 2 (count + insert run, deja commitees individuellement)", len(conn.queries))
	}
}

// TestStartCampaignRun_dbErrorOnCount / OnInsert verifient la propagation
// d'une panne technique a chaque etape.
func TestStartCampaignRun_dbErrorOnCount(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	row := scheduledCampaignRow{ID: testCampaignID}
	if err := svc.startCampaignRun(context.Background(), row); err == nil {
		t.Fatal("startCampaignRun() attendu en erreur (panne count)")
	}
}

func TestStartCampaignRun_dbErrorOnInsert(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOf("count", int64(1))},
		{err: errors.New("connexion perdue")},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	row := scheduledCampaignRow{ID: testCampaignID}
	if err := svc.startCampaignRun(context.Background(), row); err == nil {
		t.Fatal("startCampaignRun() attendu en erreur (panne insert run)")
	}
}

// TestPublishCampaignRun_nilAMQP verifie que publishCampaignRun retourne une
// erreur explicite (jamais de panique) quand s.AMQP est nil.
func TestPublishCampaignRun_nilAMQP(t *testing.T) {
	svc := &Service{Logger: testLogger(), AMQP: nil}
	if err := svc.publishCampaignRun(context.Background(), testCampaignID, 1); !errors.Is(err, errPublisherUnavailable) {
		t.Errorf("publishCampaignRun(AMQP nil) = %v, voulu errPublisherUnavailable", err)
	}
}

// TestRunCampaignScheduler_stopsOnContextCancel verifie l'arret propre du
// ticker sur annulation de contexte, avec un intervalle tres court (jamais de
// sleep bloquant de test > ~100ms).
func TestRunCampaignScheduler_stopsOnContextCancel(t *testing.T) {
	conn := &fakeConn{} // aucun step programme : comportement par defaut (0 ligne), tolere par runSchedulerTick.
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunCampaignScheduler(ctx, time.Millisecond)
		close(done)
	}()

	// Laisse au moins un tick s'executer (intervalle 1ms), puis arrete.
	time.Sleep(5 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunCampaignScheduler() n'a pas retourne apres annulation du contexte")
	}
}

// assertSingleUpdateReturning verifie qu'une requete est bien un
// UPDATE...RETURNING sur campaign.campaigns (forme attendue de l'anti-double-run).
func assertSingleUpdateReturning(t *testing.T, query string) {
	t.Helper()
	if !strings.Contains(query, "UPDATE campaign.campaigns") {
		t.Errorf("requete inattendue (table) : %q", query)
	}
	if !strings.Contains(query, "status = 'scheduled'") || !strings.Contains(query, "scheduled_at <= now()") {
		t.Errorf("requete inattendue (condition d'eligibilite) : %q", query)
	}
	if !strings.Contains(query, "RETURNING") {
		t.Errorf("requete inattendue (pas de RETURNING, capture non-atomique) : %q", query)
	}
}
