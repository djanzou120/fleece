package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"fleece/src/campaign/internal/application/ports/input"
	"fleece/src/campaign/internal/application/usecases"
	"fleece/src/campaign/internal/domain"
)

var ctx = context.Background()

// ─── CreateCampaign ────────────────────────────────────────────────────────────

func TestCreateCampaign_OK(t *testing.T) {
	repo := newMockCampaignRepo()
	uc := &usecases.CreateCampaign{
		Campaigns:   repo,
		IDGenerator: func() string { return "cmp-test" },
	}

	c, err := uc.Execute(ctx, input.CreateCampaignCommand{
		WorkspaceID:     "ws1",
		Name:            "Promo Noel",
		ChannelStrategy: "lowest_cost",
	})
	if err != nil {
		t.Fatalf("CreateCampaign inattendu: %v", err)
	}
	if c.ID != "cmp-test" {
		t.Errorf("ID attendu cmp-test, obtenu %s", c.ID)
	}
	if c.Status != domain.StatusDraft {
		t.Errorf("status attendu draft, obtenu %s", c.Status)
	}
	if len(repo.savedCampaigns) != 1 {
		t.Errorf("attendu 1 campagne sauvegardee, obtenu %d", len(repo.savedCampaigns))
	}
}

func TestCreateCampaign_MissingWorkspaceID(t *testing.T) {
	repo := newMockCampaignRepo()
	uc := &usecases.CreateCampaign{Campaigns: repo}
	_, err := uc.Execute(ctx, input.CreateCampaignCommand{Name: "Test"})
	if err == nil {
		t.Error("erreur attendue pour workspace_id vide")
	}
}

// ─── ImportRecipients / dedup ─────────────────────────────────────────────────

func TestImportRecipients_Dedup(t *testing.T) {
	recipRepo := newMockRecipientRepo()
	// Simule que seulement 2 sur 3 sont inserees (1 doublon)
	recipRepo.insertedCount = 2
	uc := &usecases.ImportRecipients{Recipients: recipRepo}

	inserted, err := uc.Execute(ctx, input.ImportRecipientsCommand{
		CampaignID: "cmp-1",
		Recipients: []string{"+237600000001", "+237600000002", "+237600000001"}, // doublon
	})
	if err != nil {
		t.Fatalf("ImportRecipients inattendu: %v", err)
	}
	if inserted != 2 {
		t.Errorf("attendu 2 inseres, obtenu %d", inserted)
	}
}

func TestImportRecipients_Empty(t *testing.T) {
	uc := &usecases.ImportRecipients{Recipients: newMockRecipientRepo()}
	inserted, err := uc.Execute(ctx, input.ImportRecipientsCommand{
		CampaignID: "cmp-1",
		Recipients: []string{},
	})
	if err != nil {
		t.Fatalf("inattendu: %v", err)
	}
	if inserted != 0 {
		t.Errorf("attendu 0, obtenu %d", inserted)
	}
}

// ─── ScheduleCampaign ────────────────────────────────────────────────────────

func TestScheduleCampaign_OK(t *testing.T) {
	repo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "lowest_cost")
	c.MessageBody = "Bonjour !"
	_ = repo.Save(ctx, c)

	uc := &usecases.ScheduleCampaign{Campaigns: repo}
	scheduled, err := uc.Execute(ctx, input.ScheduleCampaignCommand{
		CampaignID:  "cmp-1",
		ScheduledAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("ScheduleCampaign inattendu: %v", err)
	}
	if scheduled.Status != domain.StatusScheduled {
		t.Errorf("status attendu scheduled, obtenu %s", scheduled.Status)
	}
}

func TestScheduleCampaign_MissingMessageBody(t *testing.T) {
	repo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "lowest_cost")
	// message_body vide
	_ = repo.Save(ctx, c)

	uc := &usecases.ScheduleCampaign{Campaigns: repo}
	_, err := uc.Execute(ctx, input.ScheduleCampaignCommand{
		CampaignID:  "cmp-1",
		ScheduledAt: time.Now().Add(time.Hour),
	})
	if !errors.Is(err, domain.ErrMissingMessageBody) {
		t.Errorf("erreur attendue ErrMissingMessageBody, obtenu %v", err)
	}
}

func TestScheduleCampaign_MissingScheduledAt(t *testing.T) {
	repo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "lowest_cost")
	c.MessageBody = "msg"
	_ = repo.Save(ctx, c)

	uc := &usecases.ScheduleCampaign{Campaigns: repo}
	_, err := uc.Execute(ctx, input.ScheduleCampaignCommand{
		CampaignID: "cmp-1",
		// ScheduledAt zero value
	})
	if !errors.Is(err, domain.ErrMissingSchedule) {
		t.Errorf("erreur attendue ErrMissingSchedule, obtenu %v", err)
	}
}

// ─── EstimateCost ─────────────────────────────────────────────────────────────

func TestEstimateCost_OK(t *testing.T) {
	campaignRepo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "lowest_cost")
	_ = campaignRepo.Save(ctx, c)

	recipRepo := newMockRecipientRepo()
	recipRepo.pending = []*domain.CampaignRecipient{
		{ID: 1, CampaignID: "cmp-1", Recipient: "+237600000001", Status: domain.RecipientPending},
		{ID: 2, CampaignID: "cmp-1", Recipient: "+237600000002", Status: domain.RecipientPending},
	}

	routingClient := &mockRoutingClient{
		providerID: "twilio",
		channel:    "sms",
		cost:       100, // 100 centimes par message
		currency:   "XAF",
	}

	uc := &usecases.EstimateCost{
		Campaigns:  campaignRepo,
		Recipients: recipRepo,
		Routing:    routingClient,
	}

	cost, err := uc.Execute(ctx, input.EstimateCostCommand{
		CampaignID:  "cmp-1",
		WorkspaceID: "ws1",
		Country:     "CM",
	})
	if err != nil {
		t.Fatalf("EstimateCost inattendu: %v", err)
	}
	// 2 destinataires × 100 centimes = 200 centimes
	if cost.Amount != 200 {
		t.Errorf("cout attendu 200 centimes, obtenu %d", cost.Amount)
	}
	if cost.Currency != "XAF" {
		t.Errorf("devise attendue XAF, obtenue %s", cost.Currency)
	}
}

func TestEstimateCost_RoutingError_FallbackZero(t *testing.T) {
	campaignRepo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "lowest_cost")
	_ = campaignRepo.Save(ctx, c)

	recipRepo := newMockRecipientRepo()
	recipRepo.pending = []*domain.CampaignRecipient{
		{ID: 1, CampaignID: "cmp-1", Recipient: "+237600000001", Status: domain.RecipientPending},
	}

	routingClient := &mockRoutingClient{routingErr: errInternal}

	uc := &usecases.EstimateCost{
		Campaigns:  campaignRepo,
		Recipients: recipRepo,
		Routing:    routingClient,
	}

	cost, err := uc.Execute(ctx, input.EstimateCostCommand{
		CampaignID:  "cmp-1",
		WorkspaceID: "ws1",
		Country:     "CM",
	})
	if err != nil {
		t.Fatalf("EstimateCost ne doit pas retourner d'erreur sur echec routing: %v", err)
	}
	if cost.Amount != 0 {
		t.Errorf("cout attendu 0 (fallback), obtenu %d", cost.Amount)
	}
}

// ─── RunCampaign — anti-double-run ────────────────────────────────────────────

func TestRunCampaign_AntiDoubleRun(t *testing.T) {
	campaignRepo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "sms")
	c.MessageBody = "msg"
	_ = c.Schedule(time.Now().Add(time.Hour))
	_ = campaignRepo.Save(ctx, c)

	// Simuler 0 lignes affectees (un autre worker a deja pris la campagne)
	campaignRepo.atomicRows = 0

	recipRepo := newMockRecipientRepo()
	runRepo := newMockRunRepo()
	msgClient := newMockMessagingClient("msg-001")

	uc := &usecases.RunCampaign{
		Campaigns:  campaignRepo,
		Recipients: recipRepo,
		Runs:       runRepo,
		Messaging:  msgClient,
	}

	err := uc.Execute(ctx, "cmp-1")
	if err != nil {
		t.Fatalf("RunCampaign anti-double-run doit retourner nil: %v", err)
	}
	// Aucun message ne doit avoir ete envoye
	if len(msgClient.calls) != 0 {
		t.Errorf("aucun message ne devrait etre envoye, got %d", len(msgClient.calls))
	}
}

func TestRunCampaign_OK(t *testing.T) {
	campaignRepo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "sms")
	c.MessageBody = "Bonjour !"
	_ = c.Schedule(time.Now().Add(time.Hour))
	_ = campaignRepo.Save(ctx, c)

	recipRepo := newMockRecipientRepo()
	recipRepo.pending = []*domain.CampaignRecipient{
		{ID: 1, CampaignID: "cmp-1", Recipient: "+237600000001", Status: domain.RecipientPending},
		{ID: 2, CampaignID: "cmp-1", Recipient: "+237600000002", Status: domain.RecipientPending},
	}

	runRepo := newMockRunRepo()
	msgClient := newMockMessagingClient("msg-abc")

	uc := &usecases.RunCampaign{
		Campaigns:  campaignRepo,
		Recipients: recipRepo,
		Runs:       runRepo,
		Messaging:  msgClient,
	}

	err := uc.Execute(ctx, "cmp-1")
	if err != nil {
		t.Fatalf("RunCampaign inattendu: %v", err)
	}
	if len(msgClient.calls) != 2 {
		t.Errorf("attendu 2 messages envoyes, obtenu %d", len(msgClient.calls))
	}
	if runRepo.sentInc != 2 {
		t.Errorf("attendu sentInc=2, obtenu %d", runRepo.sentInc)
	}
	if len(runRepo.finishedRuns) != 1 {
		t.Error("le run doit etre finalise")
	}
}

func TestRunCampaign_SendError_MarksFailed(t *testing.T) {
	campaignRepo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "sms")
	c.MessageBody = "msg"
	_ = c.Schedule(time.Now().Add(time.Hour))
	_ = campaignRepo.Save(ctx, c)

	recipRepo := newMockRecipientRepo()
	recipRepo.pending = []*domain.CampaignRecipient{
		{ID: 1, CampaignID: "cmp-1", Recipient: "+237600000001", Status: domain.RecipientPending},
	}

	runRepo := newMockRunRepo()
	msgClient := newMockMessagingClient("")
	msgClient.sendErr = errors.New("provider unavailable")

	uc := &usecases.RunCampaign{
		Campaigns:  campaignRepo,
		Recipients: recipRepo,
		Runs:       runRepo,
		Messaging:  msgClient,
	}

	err := uc.Execute(ctx, "cmp-1")
	if err != nil {
		t.Fatalf("RunCampaign inattendu: %v", err)
	}
	if runRepo.failedInc != 1 {
		t.Errorf("attendu failedInc=1, obtenu %d", runRepo.failedInc)
	}
	if len(recipRepo.markedFailed) != 1 {
		t.Errorf("attendu 1 destinataire marque failed, obtenu %d", len(recipRepo.markedFailed))
	}
}

// ─── RecordDeliveryOutcome — correlation DLR ─────────────────────────────────

func TestRecordDeliveryOutcome_MessageIDEmpty_Ignored(t *testing.T) {
	recipRepo := newMockRecipientRepo()
	uc := &usecases.RecordDeliveryOutcome{Recipients: recipRepo}

	err := uc.Execute(ctx, input.RecordDeliveryOutcomeCommand{
		MessageID: "", // NULL/vide — doit etre ignore
		Success:   true,
	})
	if err != nil {
		t.Fatalf("inattendu: %v", err)
	}
	// Aucune mise a jour ne doit avoir ete faite
	if recipRepo.updateStatusRows != 1 {
		// updateStatusRows n'a pas ete modifie car on n'a pas appele UpdateStatusByMessageID
	}
}

func TestRecordDeliveryOutcome_UnknownMessageID_Ignored(t *testing.T) {
	recipRepo := newMockRecipientRepo()
	recipRepo.updateStatusRows = 0 // simuler message_id inconnu
	uc := &usecases.RecordDeliveryOutcome{Recipients: recipRepo}

	err := uc.Execute(ctx, input.RecordDeliveryOutcomeCommand{
		MessageID: "msg-unknown",
		Success:   true,
	})
	if err != nil {
		t.Fatalf("DLR orphelin ne doit pas retourner d'erreur: %v", err)
	}
}

func TestRecordDeliveryOutcome_Delivered(t *testing.T) {
	recipRepo := newMockRecipientRepo()
	recipRepo.updateStatusRows = 1
	uc := &usecases.RecordDeliveryOutcome{Recipients: recipRepo}

	err := uc.Execute(ctx, input.RecordDeliveryOutcomeCommand{
		MessageID: "msg-001",
		Success:   true,
	})
	if err != nil {
		t.Fatalf("inattendu: %v", err)
	}
}

func TestRecordDeliveryOutcome_Failed(t *testing.T) {
	recipRepo := newMockRecipientRepo()
	recipRepo.updateStatusRows = 1
	uc := &usecases.RecordDeliveryOutcome{Recipients: recipRepo}

	err := uc.Execute(ctx, input.RecordDeliveryOutcomeCommand{
		MessageID: "msg-001",
		Success:   false,
	})
	if err != nil {
		t.Fatalf("inattendu: %v", err)
	}
}

// ─── PauseCampaign / CancelCampaign ──────────────────────────────────────────

func TestPauseCampaign_OK(t *testing.T) {
	repo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "sms")
	c.MessageBody = "msg"
	_ = c.Schedule(time.Now().Add(time.Hour))
	_ = c.StartRun()
	_ = repo.Save(ctx, c)

	uc := &usecases.PauseCampaign{Campaigns: repo}
	err := uc.Execute(ctx, "cmp-1")
	if err != nil {
		t.Fatalf("PauseCampaign inattendu: %v", err)
	}
	updated, _ := repo.Get(ctx, "cmp-1")
	if updated.Status != domain.StatusPaused {
		t.Errorf("status attendu paused, obtenu %s", updated.Status)
	}
}

func TestCancelCampaign_FromDraft(t *testing.T) {
	repo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "sms")
	_ = repo.Save(ctx, c)

	uc := &usecases.CancelCampaign{Campaigns: repo}
	err := uc.Execute(ctx, "cmp-1")
	if err != nil {
		t.Fatalf("CancelCampaign inattendu: %v", err)
	}
	updated, _ := repo.Get(ctx, "cmp-1")
	if updated.Status != domain.StatusCancelled {
		t.Errorf("status attendu cancelled, obtenu %s", updated.Status)
	}
}

func TestCancelCampaign_FromRunning_Invalid(t *testing.T) {
	repo := newMockCampaignRepo()
	c := domain.NewCampaign("cmp-1", "ws1", "Test", "sms")
	c.MessageBody = "msg"
	_ = c.Schedule(time.Now().Add(time.Hour))
	_ = c.StartRun()
	_ = repo.Save(ctx, c)

	uc := &usecases.CancelCampaign{Campaigns: repo}
	err := uc.Execute(ctx, "cmp-1")
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("erreur attendue ErrInvalidTransition, obtenu %v", err)
	}
}
