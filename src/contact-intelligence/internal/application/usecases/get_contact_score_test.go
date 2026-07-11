package usecases

import (
	"context"
	"errors"
	"testing"

	"fleece/src/contact-intelligence/internal/domain"
)

func TestGetContactScore_ContactInconnu(t *testing.T) {
	contactRepo := newMockContactRepo() // vide
	scoreRepo := newMockChannelScoreRepo()

	uc := GetContactScore{
		Contacts:      contactRepo,
		ChannelScores: scoreRepo,
	}

	result, err := uc.Execute(context.Background(), "+237690000000")
	if err != nil {
		t.Fatalf("inattendu: %v", err)
	}
	if result.Found {
		t.Error("attendu Found=false pour contact inconnu")
	}
	if result.DeliveryScore.Value() != 50 {
		t.Errorf("score neutre attendu 50, obtenu %d", result.DeliveryScore.Value())
	}
	if result.ChannelScores != nil {
		t.Errorf("channel_scores doit etre nil pour contact inconnu, obtenu %v", result.ChannelScores)
	}
}

func TestGetContactScore_ContactConnu(t *testing.T) {
	contactRepo := newMockContactRepo()
	scoreRepo := newMockChannelScoreRepo()

	// Preparer un contact existant.
	contact, _ := domain.NewContact("+237690000001", "CM")
	_ = contact.RecordOutcome(domain.ChannelWhatsApp, true, timeNow())
	_ = contact.RecordOutcome(domain.ChannelWhatsApp, true, timeNow())
	contactRepo.contacts[contact.Phone] = contact

	// Preparer un score existant.
	cs, _ := domain.NewChannelScore("+237690000001", domain.ChannelWhatsApp)
	cs.RecordOutcome(true, timeNow())
	scoreRepo.scores["+237690000001:whatsapp"] = cs

	uc := GetContactScore{
		Contacts:      contactRepo,
		ChannelScores: scoreRepo,
	}

	result, err := uc.Execute(context.Background(), "+237690000001")
	if err != nil {
		t.Fatalf("inattendu: %v", err)
	}
	if !result.Found {
		t.Error("attendu Found=true pour contact connu")
	}
	if len(result.ChannelScores) == 0 {
		t.Error("attendu au moins un channel_score")
	}
	if result.DeliveryScore.Value() <= 50 {
		t.Errorf("score global attendu > 50 apres succes, obtenu %d", result.DeliveryScore.Value())
	}
}

func TestGetContactScore_PhoneVide(t *testing.T) {
	contactRepo := newMockContactRepo()
	scoreRepo := newMockChannelScoreRepo()

	uc := GetContactScore{
		Contacts:      contactRepo,
		ChannelScores: scoreRepo,
	}

	_, err := uc.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("attendu ErrEmptyPhone")
	}
	if !errors.Is(err, domain.ErrEmptyPhone) {
		t.Errorf("attendu ErrEmptyPhone, obtenu %v", err)
	}
}

func TestGetContactScore_ErreurRepository(t *testing.T) {
	contactRepo := newMockContactRepo()
	contactRepo.getErr = errInternal
	scoreRepo := newMockChannelScoreRepo()

	uc := GetContactScore{
		Contacts:      contactRepo,
		ChannelScores: scoreRepo,
	}

	_, err := uc.Execute(context.Background(), "+237690000002")
	if err == nil {
		t.Fatal("attendu une erreur")
	}
}
