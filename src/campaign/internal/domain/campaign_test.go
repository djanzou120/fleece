package domain_test

import (
	"errors"
	"testing"
	"time"

	"fleece/src/campaign/internal/domain"
)

// ─── Machine a etats ─────────────────────────────────────────────────────────

func TestCampaign_Schedule_OK(t *testing.T) {
	c := domain.NewCampaign("c1", "ws1", "Test", "lowest_cost")
	c.MessageBody = "Bonjour !"

	at := time.Now().Add(time.Hour)
	if err := c.Schedule(at); err != nil {
		t.Fatalf("Schedule inattendu: %v", err)
	}
	if c.Status != domain.StatusScheduled {
		t.Errorf("status attendu scheduled, obtenu %s", c.Status)
	}
	if c.ScheduledAt == nil || !c.ScheduledAt.Equal(at.UTC()) {
		t.Errorf("scheduled_at incorrect: %v", c.ScheduledAt)
	}
}

func TestCampaign_Schedule_MissingMessageBody(t *testing.T) {
	c := domain.NewCampaign("c1", "ws1", "Test", "lowest_cost")
	// message_body vide
	err := c.Schedule(time.Now().Add(time.Hour))
	if !errors.Is(err, domain.ErrMissingMessageBody) {
		t.Errorf("erreur attendue ErrMissingMessageBody, obtenu %v", err)
	}
	if c.Status != domain.StatusDraft {
		t.Errorf("le statut ne doit pas changer apres echec: %s", c.Status)
	}
}

func TestCampaign_Schedule_InvalidTransition_FromScheduled(t *testing.T) {
	c := domain.NewCampaign("c1", "ws1", "Test", "lowest_cost")
	c.MessageBody = "msg"
	_ = c.Schedule(time.Now().Add(time.Hour))

	// Tenter de planifier a nouveau depuis scheduled
	err := c.Schedule(time.Now().Add(2 * time.Hour))
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("erreur attendue ErrInvalidTransition, obtenu %v", err)
	}
}

func TestCampaign_StartRun_OK(t *testing.T) {
	c := scheduledCampaign(t)
	if err := c.StartRun(); err != nil {
		t.Fatalf("StartRun inattendu: %v", err)
	}
	if c.Status != domain.StatusRunning {
		t.Errorf("status attendu running, obtenu %s", c.Status)
	}
}

func TestCampaign_StartRun_InvalidTransition(t *testing.T) {
	c := domain.NewCampaign("c1", "ws1", "Test", "lowest_cost")
	err := c.StartRun()
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("erreur attendue ErrInvalidTransition, obtenu %v", err)
	}
}

func TestCampaign_Complete_OK(t *testing.T) {
	c := runningCampaign(t)
	if err := c.Complete(); err != nil {
		t.Fatalf("Complete inattendu: %v", err)
	}
	if c.Status != domain.StatusCompleted {
		t.Errorf("status attendu completed, obtenu %s", c.Status)
	}
}

func TestCampaign_Fail_OK(t *testing.T) {
	c := runningCampaign(t)
	if err := c.Fail(); err != nil {
		t.Fatalf("Fail inattendu: %v", err)
	}
	if c.Status != domain.StatusFailed {
		t.Errorf("status attendu failed, obtenu %s", c.Status)
	}
}

func TestCampaign_Pause_And_Resume(t *testing.T) {
	c := runningCampaign(t)

	if err := c.Pause(); err != nil {
		t.Fatalf("Pause inattendu: %v", err)
	}
	if c.Status != domain.StatusPaused {
		t.Errorf("status attendu paused, obtenu %s", c.Status)
	}

	if err := c.Resume(); err != nil {
		t.Fatalf("Resume inattendu: %v", err)
	}
	if c.Status != domain.StatusRunning {
		t.Errorf("status attendu running apres resume, obtenu %s", c.Status)
	}
}

func TestCampaign_Pause_InvalidTransition(t *testing.T) {
	c := domain.NewCampaign("c1", "ws1", "Test", "lowest_cost")
	err := c.Pause()
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("erreur attendue ErrInvalidTransition, obtenu %v", err)
	}
}

func TestCampaign_Cancel_FromDraft(t *testing.T) {
	c := domain.NewCampaign("c1", "ws1", "Test", "lowest_cost")
	if err := c.Cancel(); err != nil {
		t.Fatalf("Cancel inattendu: %v", err)
	}
	if c.Status != domain.StatusCancelled {
		t.Errorf("status attendu cancelled, obtenu %s", c.Status)
	}
}

func TestCampaign_Cancel_FromScheduled(t *testing.T) {
	c := scheduledCampaign(t)
	if err := c.Cancel(); err != nil {
		t.Fatalf("Cancel inattendu: %v", err)
	}
	if c.Status != domain.StatusCancelled {
		t.Errorf("status attendu cancelled, obtenu %s", c.Status)
	}
}

func TestCampaign_Cancel_FromPaused(t *testing.T) {
	c := runningCampaign(t)
	_ = c.Pause()
	if err := c.Cancel(); err != nil {
		t.Fatalf("Cancel inattendu: %v", err)
	}
	if c.Status != domain.StatusCancelled {
		t.Errorf("status attendu cancelled, obtenu %s", c.Status)
	}
}

func TestCampaign_Cancel_FromRunning_Invalid(t *testing.T) {
	c := runningCampaign(t)
	err := c.Cancel()
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("erreur attendue ErrInvalidTransition depuis running, obtenu %v", err)
	}
}

func TestCampaign_Cancel_FromCompleted_Invalid(t *testing.T) {
	c := runningCampaign(t)
	_ = c.Complete()
	err := c.Cancel()
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("erreur attendue ErrInvalidTransition depuis completed, obtenu %v", err)
	}
}

// ─── Money ────────────────────────────────────────────────────────────────────

func TestMoney_Multiply(t *testing.T) {
	m, err := domain.NewMoney(100, "XAF")
	if err != nil {
		t.Fatal(err)
	}
	result, err := m.Multiply(50)
	if err != nil {
		t.Fatalf("Multiply inattendu: %v", err)
	}
	if result.Amount != 5000 {
		t.Errorf("attendu 5000 centimes, obtenu %d", result.Amount)
	}
	if result.Currency != "XAF" {
		t.Errorf("devise incorrecte: %s", result.Currency)
	}
}

func TestMoney_Multiply_Zero(t *testing.T) {
	m, _ := domain.NewMoney(100, "XAF")
	result, err := m.Multiply(0)
	if err != nil {
		t.Fatalf("Multiply(0) inattendu: %v", err)
	}
	if result.Amount != 0 {
		t.Errorf("attendu 0, obtenu %d", result.Amount)
	}
}

func TestMoney_Negative_Error(t *testing.T) {
	_, err := domain.NewMoney(-1, "XAF")
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("erreur attendue ErrInvalidAmount, obtenu %v", err)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func scheduledCampaign(t *testing.T) *domain.Campaign {
	t.Helper()
	c := domain.NewCampaign("c1", "ws1", "Test", "lowest_cost")
	c.MessageBody = "Bonjour !"
	if err := c.Schedule(time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("helper scheduledCampaign: %v", err)
	}
	return c
}

func runningCampaign(t *testing.T) *domain.Campaign {
	t.Helper()
	c := scheduledCampaign(t)
	if err := c.StartRun(); err != nil {
		t.Fatalf("helper runningCampaign: %v", err)
	}
	return c
}
