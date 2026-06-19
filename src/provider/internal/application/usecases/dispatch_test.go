package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"fleece/src/provider/internal/application/ports/output"
	"fleece/src/provider/internal/domain"
)

// --- Mocks des ports ---

// mockProvider implemente output.Provider pour les tests.
type mockProvider struct {
	sendExternalId string
	sendErr        error
	estimateCost   domain.Money
	estimateErr    error
	dlrStatus      domain.DeliveryStatus
	dlrErr         error
	sendCalled     bool
}

func (m *mockProvider) Send(_ context.Context, _ domain.OutboundMessage) (string, error) {
	m.sendCalled = true
	return m.sendExternalId, m.sendErr
}

func (m *mockProvider) EstimateCost(_ context.Context, _, _ string) (domain.Money, error) {
	return m.estimateCost, m.estimateErr
}

func (m *mockProvider) GetDeliveryStatus(_ context.Context, _ string) (domain.DeliveryStatus, error) {
	return m.dlrStatus, m.dlrErr
}

// mockProviderRepository implemente output.ProviderRepository pour les tests.
type mockProviderRepository struct {
	saved         []saveCall
	saveErr       error
	statusUpdates []statusUpdate
	updateErr     error
}

type saveCall struct {
	internalId string
	providerId string
	externalId string
}

type statusUpdate struct {
	externalId string
	status     string
}

func (m *mockProviderRepository) Save(_ context.Context, internalId, providerId, externalId string) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = append(m.saved, saveCall{internalId, providerId, externalId})
	return nil
}

func (m *mockProviderRepository) FindByInternalID(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (m *mockProviderRepository) UpdateStatus(_ context.Context, externalId string, status string, _ time.Time) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.statusUpdates = append(m.statusUpdates, statusUpdate{externalId, status})
	return nil
}

// mockEventPublisher implemente output.EventPublisher pour les tests.
type mockEventPublisher struct {
	events []string
}

func (m *mockEventPublisher) Publish(_ context.Context, event string, _ any) error {
	m.events = append(m.events, event)
	return nil
}

// Assertions de conformite aux interfaces (compile-time).
var _ output.Provider = (*mockProvider)(nil)
var _ output.ProviderRepository = (*mockProviderRepository)(nil)
var _ output.EventPublisher = (*mockEventPublisher)(nil)

// --- Tests Dispatch ---

func TestDispatch_Success(t *testing.T) {
	p := &mockProvider{sendExternalId: "ext-123"}
	repo := &mockProviderRepository{}
	pub := &mockEventPublisher{}

	uc := &Dispatch{
		Registry:  map[string]output.Provider{"provider-1": p},
		Repo:      repo,
		Publisher: pub,
	}

	msg := domain.OutboundMessage{
		Id:         "internal-uuid-1",
		ProviderId: "provider-1",
		Channel:    "whatsapp",
		Recipient:  "+237600000001",
		Content:    "Hello Fleece",
	}

	externalId, err := uc.Execute(context.Background(), msg)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// Send doit avoir ete appele.
	if !p.sendCalled {
		t.Error("attendu Send appele, pas appele")
	}

	// L'externalId retourne doit correspondre.
	if externalId != "ext-123" {
		t.Errorf("externalId attendu 'ext-123', obtenu '%s'", externalId)
	}

	// Save doit avoir ete appele avec les bons parametres.
	if len(repo.saved) != 1 {
		t.Fatalf("attendu 1 Save, obtenu %d", len(repo.saved))
	}
	s := repo.saved[0]
	if s.internalId != msg.Id || s.providerId != msg.ProviderId || s.externalId != externalId {
		t.Errorf("Save appele avec mauvais params: %+v", s)
	}

	// Evenement "provider.sent" doit avoir ete publie.
	if len(pub.events) == 0 || pub.events[len(pub.events)-1] != "provider.sent" {
		t.Errorf("attendu evenement provider.sent, obtenu %v", pub.events)
	}
}

func TestDispatch_ProviderNotFound(t *testing.T) {
	repo := &mockProviderRepository{}
	pub := &mockEventPublisher{}

	uc := &Dispatch{
		Registry:  map[string]output.Provider{},
		Repo:      repo,
		Publisher: pub,
	}

	msg := domain.OutboundMessage{
		Id:         "internal-uuid-2",
		ProviderId: "absent",
		Channel:    "sms",
		Recipient:  "+237600000002",
		Content:    "Test",
	}

	_, err := uc.Execute(context.Background(), msg)
	if err == nil {
		t.Fatal("attendu ErrProviderNotFound")
	}
	if !errors.Is(err, domain.ErrProviderNotFound) {
		t.Errorf("attendu ErrProviderNotFound, obtenu %v", err)
	}

	// Aucun Save ni evenement.
	if len(repo.saved) != 0 {
		t.Errorf("aucun Save attendu, obtenu %d", len(repo.saved))
	}
}

func TestDispatch_SendFails(t *testing.T) {
	sendErr := errors.New("connexion reseau echouee")
	p := &mockProvider{sendErr: sendErr}
	repo := &mockProviderRepository{}
	pub := &mockEventPublisher{}

	uc := &Dispatch{
		Registry:  map[string]output.Provider{"provider-2": p},
		Repo:      repo,
		Publisher: pub,
	}

	msg := domain.OutboundMessage{
		Id:         "internal-uuid-3",
		ProviderId: "provider-2",
		Channel:    "sms",
		Recipient:  "+237600000003",
		Content:    "Test",
	}

	_, err := uc.Execute(context.Background(), msg)
	if err == nil {
		t.Fatal("attendu ErrSendFailed")
	}
	if !errors.Is(err, domain.ErrSendFailed) {
		t.Errorf("attendu ErrSendFailed, obtenu %v", err)
	}

	// L'evenement "provider.failed" doit avoir ete publie.
	if len(pub.events) == 0 || pub.events[0] != "provider.failed" {
		t.Errorf("attendu evenement provider.failed, obtenu %v", pub.events)
	}

	// Aucun Save car Send a echoue.
	if len(repo.saved) != 0 {
		t.Errorf("aucun Save attendu apres echec Send, obtenu %d", len(repo.saved))
	}
}
