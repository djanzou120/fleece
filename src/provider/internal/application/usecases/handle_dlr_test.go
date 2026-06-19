package usecases

import (
	"context"
	"testing"

	"fleece/src/provider/internal/domain"
)

// mockEventPublisher is already defined in dispatch_test.go (same package).
// mockProviderRepository is already defined in dispatch_test.go (same package).

func TestHandleDLR_Delivered_PublishesDeliveredEvent(t *testing.T) {
	pub := &mockEventPublisher{}
	repo := &mockProviderRepository{}
	uc := &HandleDLR{Publisher: pub, Repo: repo}

	err := uc.Execute(context.Background(), "ext-123", domain.StatusDelivered)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	if len(pub.events) == 0 {
		t.Fatal("aucun evenement publie")
	}
	if pub.events[0] != "provider.delivered" {
		t.Errorf("attendu 'provider.delivered', obtenu '%s'", pub.events[0])
	}
}

func TestHandleDLR_Failed_PublishesFailedEvent(t *testing.T) {
	pub := &mockEventPublisher{}
	repo := &mockProviderRepository{}
	uc := &HandleDLR{Publisher: pub, Repo: repo}

	err := uc.Execute(context.Background(), "ext-456", domain.StatusFailed)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	if len(pub.events) == 0 {
		t.Fatal("aucun evenement publie")
	}
	if pub.events[0] != "provider.failed" {
		t.Errorf("attendu 'provider.failed', obtenu '%s'", pub.events[0])
	}
}

func TestHandleDLR_Rejected_PublishesFailedEvent(t *testing.T) {
	pub := &mockEventPublisher{}
	repo := &mockProviderRepository{}
	uc := &HandleDLR{Publisher: pub, Repo: repo}

	err := uc.Execute(context.Background(), "ext-789", domain.StatusRejected)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	if len(pub.events) == 0 {
		t.Fatal("aucun evenement publie")
	}
	if pub.events[0] != "provider.failed" {
		t.Errorf("attendu 'provider.failed' pour Rejected, obtenu '%s'", pub.events[0])
	}
}

func TestHandleDLR_InvalidStatus_ReturnsError(t *testing.T) {
	pub := &mockEventPublisher{}
	repo := &mockProviderRepository{}
	uc := &HandleDLR{Publisher: pub, Repo: repo}

	err := uc.Execute(context.Background(), "ext-999", domain.StatusPending)
	if err == nil {
		t.Fatal("attendu une erreur pour statut DLR invalide (StatusPending)")
	}

	// Aucun evenement publie.
	if len(pub.events) != 0 {
		t.Errorf("aucun evenement attendu pour statut invalide, obtenu %v", pub.events)
	}
}

func TestHandleDLR_SentStatus_ReturnsError(t *testing.T) {
	pub := &mockEventPublisher{}
	repo := &mockProviderRepository{}
	uc := &HandleDLR{Publisher: pub, Repo: repo}

	// StatusSent n'est pas un statut DLR final valide.
	err := uc.Execute(context.Background(), "ext-sent", domain.StatusSent)
	if err == nil {
		t.Fatal("attendu une erreur pour statut DLR invalide (StatusSent)")
	}
}

func TestHandleDLR_EmptyExternalID_ReturnsError(t *testing.T) {
	pub := &mockEventPublisher{}
	repo := &mockProviderRepository{}
	uc := &HandleDLR{Publisher: pub, Repo: repo}

	err := uc.Execute(context.Background(), "", domain.StatusDelivered)
	if err == nil {
		t.Fatal("attendu une erreur pour externalId vide")
	}
}

// TestHandleDLR_Delivered_PersistsStatus verifie que UpdateStatus est appele
// avec le bon externalId et le statut "delivered" apres un DLR de livraison.
func TestHandleDLR_Delivered_PersistsStatus(t *testing.T) {
	pub := &mockEventPublisher{}
	repo := &mockProviderRepository{}
	uc := &HandleDLR{Publisher: pub, Repo: repo}

	err := uc.Execute(context.Background(), "ext-del-1", domain.StatusDelivered)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	if len(repo.statusUpdates) != 1 {
		t.Fatalf("attendu 1 UpdateStatus, obtenu %d", len(repo.statusUpdates))
	}
	u := repo.statusUpdates[0]
	if u.externalId != "ext-del-1" {
		t.Errorf("externalId attendu 'ext-del-1', obtenu '%s'", u.externalId)
	}
	if u.status != string(domain.StatusDelivered) {
		t.Errorf("status attendu 'delivered', obtenu '%s'", u.status)
	}
}

// TestHandleDLR_Failed_PersistsStatus verifie que UpdateStatus est appele
// avec le statut "failed" apres un DLR d'echec.
func TestHandleDLR_Failed_PersistsStatus(t *testing.T) {
	pub := &mockEventPublisher{}
	repo := &mockProviderRepository{}
	uc := &HandleDLR{Publisher: pub, Repo: repo}

	err := uc.Execute(context.Background(), "ext-fail-1", domain.StatusFailed)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	if len(repo.statusUpdates) != 1 {
		t.Fatalf("attendu 1 UpdateStatus, obtenu %d", len(repo.statusUpdates))
	}
	u := repo.statusUpdates[0]
	if u.status != string(domain.StatusFailed) {
		t.Errorf("status attendu 'failed', obtenu '%s'", u.status)
	}
}

// TestHandleDLR_Rejected_PersistsRejectedStatus verifie que UpdateStatus est appele
// avec le statut "rejected" (et non "failed") apres un DLR de rejet.
func TestHandleDLR_Rejected_PersistsRejectedStatus(t *testing.T) {
	pub := &mockEventPublisher{}
	repo := &mockProviderRepository{}
	uc := &HandleDLR{Publisher: pub, Repo: repo}

	err := uc.Execute(context.Background(), "ext-rej-1", domain.StatusRejected)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	if len(repo.statusUpdates) != 1 {
		t.Fatalf("attendu 1 UpdateStatus, obtenu %d", len(repo.statusUpdates))
	}
	u := repo.statusUpdates[0]
	if u.status != string(domain.StatusRejected) {
		t.Errorf("status attendu 'rejected', obtenu '%s'", u.status)
	}
}

// TestHandleDLR_NilRepo_NoUpdateStatusPanic verifie que HandleDLR ne panique pas
// si Repo est nil (cas dev sans base de donnees).
func TestHandleDLR_NilRepo_NoUpdateStatusPanic(t *testing.T) {
	pub := &mockEventPublisher{}
	uc := &HandleDLR{Publisher: pub, Repo: nil}

	err := uc.Execute(context.Background(), "ext-noRepo", domain.StatusDelivered)
	if err != nil {
		t.Fatalf("Execute inattendu avec Repo nil: %v", err)
	}
}
