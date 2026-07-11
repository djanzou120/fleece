package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"fleece/src/contact-intelligence/internal/domain"
)

// timeNow retourne un instant fixe pour les tests.
func timeNow() time.Time {
	return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
}

func TestRecordDeliveryOutcome_NouveauContact_Succes(t *testing.T) {
	contactRepo := newMockContactRepo()
	scoreRepo := newMockChannelScoreRepo()
	store := &mockOutcomeStore{}

	uc := RecordDeliveryOutcome{
		Contacts:      contactRepo,
		ChannelScores: scoreRepo,
		Store:         store,
	}

	err := uc.Execute(context.Background(), "+237690000010", domain.ChannelWhatsApp, true)
	if err != nil {
		t.Fatalf("inattendu: %v", err)
	}

	// Une entree doit avoir ete persistee atomiquement.
	if len(store.recorded) != 1 {
		t.Fatalf("attendu 1 entree atomique, obtenu %d", len(store.recorded))
	}

	rec := store.recorded[0]

	// L'outcome doit etre succes sur whatsapp.
	if rec.outcome.Phone != "+237690000010" {
		t.Errorf("phone attendu +237690000010, obtenu %s", rec.outcome.Phone)
	}
	if rec.outcome.Channel != domain.ChannelWhatsApp {
		t.Errorf("channel attendu whatsapp, obtenu %s", rec.outcome.Channel)
	}
	if !rec.outcome.Success {
		t.Error("success attendu true")
	}

	// Le channel_score doit etre initialise et superieur au prior.
	if rec.channelScore.Score.Value() <= 50 {
		t.Errorf("score canal apres succes attendu > 50, obtenu %d", rec.channelScore.Score.Value())
	}
	if rec.channelScore.SampleSize != 1 {
		t.Errorf("sample_size attendu 1, obtenu %d", rec.channelScore.SampleSize)
	}

	// Le contact doit avoir success_count=1.
	if rec.contact.SuccessCount != 1 {
		t.Errorf("success_count attendu 1, obtenu %d", rec.contact.SuccessCount)
	}
	if rec.contact.FailureCount != 0 {
		t.Errorf("failure_count attendu 0, obtenu %d", rec.contact.FailureCount)
	}
}

func TestRecordDeliveryOutcome_NouveauContact_Echec(t *testing.T) {
	contactRepo := newMockContactRepo()
	scoreRepo := newMockChannelScoreRepo()
	store := &mockOutcomeStore{}

	uc := RecordDeliveryOutcome{
		Contacts:      contactRepo,
		ChannelScores: scoreRepo,
		Store:         store,
	}

	err := uc.Execute(context.Background(), "+237690000011", domain.ChannelSMS, false)
	if err != nil {
		t.Fatalf("inattendu: %v", err)
	}

	rec := store.recorded[0]

	// Score inferieur au prior sur un echec.
	if rec.channelScore.Score.Value() >= 50 {
		t.Errorf("score canal apres echec attendu < 50, obtenu %d", rec.channelScore.Score.Value())
	}
	// failure_count incremente.
	if rec.contact.FailureCount != 1 {
		t.Errorf("failure_count attendu 1, obtenu %d", rec.contact.FailureCount)
	}
	if rec.contact.SuccessCount != 0 {
		t.Errorf("success_count attendu 0, obtenu %d", rec.contact.SuccessCount)
	}
}

func TestRecordDeliveryOutcome_ContactExistant(t *testing.T) {
	contactRepo := newMockContactRepo()
	scoreRepo := newMockChannelScoreRepo()
	store := &mockOutcomeStore{}

	// Pre-charger un contact existant avec deja un historique.
	existing, _ := domain.NewContact("+237690000012", "CM")
	existing.SuccessCount = 5
	existing.FailureCount = 2
	existing.DeliveryScore = domain.ComputeScore(5, 2)
	contactRepo.contacts[existing.Phone] = existing

	uc := RecordDeliveryOutcome{
		Contacts:      contactRepo,
		ChannelScores: scoreRepo,
		Store:         store,
	}

	err := uc.Execute(context.Background(), "+237690000012", domain.ChannelWhatsApp, true)
	if err != nil {
		t.Fatalf("inattendu: %v", err)
	}

	rec := store.recorded[0]

	// Le contact doit avoir success_count incremente de 5 a 6.
	if rec.contact.SuccessCount != 6 {
		t.Errorf("success_count attendu 6, obtenu %d", rec.contact.SuccessCount)
	}
	if rec.contact.FailureCount != 2 {
		t.Errorf("failure_count attendu 2, obtenu %d", rec.contact.FailureCount)
	}
}

func TestRecordDeliveryOutcome_PhoneVide(t *testing.T) {
	contactRepo := newMockContactRepo()
	scoreRepo := newMockChannelScoreRepo()
	store := &mockOutcomeStore{}

	uc := RecordDeliveryOutcome{
		Contacts:      contactRepo,
		ChannelScores: scoreRepo,
		Store:         store,
	}

	err := uc.Execute(context.Background(), "", domain.ChannelWhatsApp, true)
	if err == nil {
		t.Fatal("attendu ErrEmptyPhone")
	}
	if !errors.Is(err, domain.ErrEmptyPhone) {
		t.Errorf("attendu ErrEmptyPhone, obtenu %v", err)
	}
	if len(store.recorded) != 0 {
		t.Errorf("aucune persistance attendue sur erreur de validation")
	}
}

func TestRecordDeliveryOutcome_CanalInvalide(t *testing.T) {
	contactRepo := newMockContactRepo()
	scoreRepo := newMockChannelScoreRepo()
	store := &mockOutcomeStore{}

	uc := RecordDeliveryOutcome{
		Contacts:      contactRepo,
		ChannelScores: scoreRepo,
		Store:         store,
	}

	err := uc.Execute(context.Background(), "+237690000013", domain.Channel("pigeon-voyageur"), true)
	if err == nil {
		t.Fatal("attendu ErrInvalidChannel")
	}
	if !errors.Is(err, domain.ErrInvalidChannel) {
		t.Errorf("attendu ErrInvalidChannel, obtenu %v", err)
	}
}

func TestRecordDeliveryOutcome_ErreurStore(t *testing.T) {
	contactRepo := newMockContactRepo()
	scoreRepo := newMockChannelScoreRepo()
	store := &mockOutcomeStore{err: errInternal}

	uc := RecordDeliveryOutcome{
		Contacts:      contactRepo,
		ChannelScores: scoreRepo,
		Store:         store,
	}

	err := uc.Execute(context.Background(), "+237690000014", domain.ChannelSMS, true)
	if err == nil {
		t.Fatal("attendu une erreur du store")
	}
}

func TestRecordDeliveryOutcome_ScoreConvergeVersVraiTaux(t *testing.T) {
	// Verifie que le scoring converge : 8 succes sur 10 → score proche de 80.
	contactRepo := newMockContactRepo()
	scoreRepo := newMockChannelScoreRepo()

	// Simuler des outcomes successifs via le domaine directement.
	cs, _ := domain.NewChannelScore("+test", domain.ChannelWhatsApp)
	for i := 0; i < 8; i++ {
		cs.RecordOutcome(true, time.Now())
	}
	for i := 0; i < 2; i++ {
		cs.RecordOutcome(false, time.Now())
	}

	// 8 succes, 2 echecs, 10 total → Laplace : (8+1)/(8+2+2)*100 = 75.
	expected := 75
	got := cs.Score.Value()
	if got != expected {
		t.Errorf("apres 8/10 succes: attendu %d, obtenu %d", expected, got)
	}
	_ = contactRepo
	_ = scoreRepo
}
