package usecases

import (
	"context"
	"testing"

	"fleece/src/provider/internal/domain"
)

// mockEventPublisher is already defined in dispatch_test.go (same package).

func TestHandleDLR_Delivered_PublishesDeliveredEvent(t *testing.T) {
	pub := &mockEventPublisher{}
	uc := &HandleDLR{Publisher: pub}

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
	uc := &HandleDLR{Publisher: pub}

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
	uc := &HandleDLR{Publisher: pub}

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
	uc := &HandleDLR{Publisher: pub}

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
	uc := &HandleDLR{Publisher: pub}

	// StatusSent n'est pas un statut DLR final valide.
	err := uc.Execute(context.Background(), "ext-sent", domain.StatusSent)
	if err == nil {
		t.Fatal("attendu une erreur pour statut DLR invalide (StatusSent)")
	}
}

func TestHandleDLR_EmptyExternalID_ReturnsError(t *testing.T) {
	pub := &mockEventPublisher{}
	uc := &HandleDLR{Publisher: pub}

	err := uc.Execute(context.Background(), "", domain.StatusDelivered)
	if err == nil {
		t.Fatal("attendu une erreur pour externalId vide")
	}
}
