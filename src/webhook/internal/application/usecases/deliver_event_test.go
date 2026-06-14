package usecases

import (
	"context"
	"net/http"
	"testing"

	"fleece/src/webhook/internal/domain"
)

// --- Mocks des ports ---

type mockEndpointRepo struct {
	endpoints []*domain.WebhookEndpoint
	saved     []*domain.WebhookEndpoint
	findErr   error
}

func (m *mockEndpointRepo) Save(_ context.Context, e *domain.WebhookEndpoint) error {
	m.saved = append(m.saved, e)
	return nil
}

func (m *mockEndpointRepo) FindByID(_ context.Context, id string) (*domain.WebhookEndpoint, error) {
	for _, ep := range m.endpoints {
		if ep.ID == id {
			return ep, nil
		}
	}
	return nil, domain.ErrEndpointNotFound
}

func (m *mockEndpointRepo) FindByWorkspaceAndEvent(_ context.Context, workspaceID, event string) ([]*domain.WebhookEndpoint, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var result []*domain.WebhookEndpoint
	for _, ep := range m.endpoints {
		if ep.WorkspaceID == workspaceID && ep.Subscribes(event) {
			result = append(result, ep)
		}
	}
	return result, nil
}

func (m *mockEndpointRepo) ListByWorkspace(_ context.Context, workspaceID string) ([]*domain.WebhookEndpoint, error) {
	var result []*domain.WebhookEndpoint
	for _, ep := range m.endpoints {
		if ep.WorkspaceID == workspaceID {
			result = append(result, ep)
		}
	}
	return result, nil
}

func (m *mockEndpointRepo) Delete(_ context.Context, id string) error {
	for i, ep := range m.endpoints {
		if ep.ID == id {
			m.endpoints = append(m.endpoints[:i], m.endpoints[i+1:]...)
			return nil
		}
	}
	return domain.ErrEndpointNotFound
}

type mockDeliveryRepo struct {
	deliveries []*domain.WebhookDelivery
	saveErr    error
}

func (m *mockDeliveryRepo) Save(_ context.Context, d *domain.WebhookDelivery) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	// Simuler l'attribution d'un ID a l'insert.
	if d.ID == 0 {
		d.ID = int64(len(m.deliveries) + 1)
	}
	// Copier pour eviter les mutations.
	cp := *d
	m.deliveries = append(m.deliveries, &cp)
	return nil
}

func (m *mockDeliveryRepo) FindByID(_ context.Context, id int64) (*domain.WebhookDelivery, error) {
	for _, d := range m.deliveries {
		if d.ID == id {
			cp := *d
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockDeliveryRepo) FindFailedByEndpoint(_ context.Context, endpointID string) ([]*domain.WebhookDelivery, error) {
	var result []*domain.WebhookDelivery
	for _, d := range m.deliveries {
		if d.EndpointID == endpointID && d.Status == domain.StatusFailed {
			cp := *d
			result = append(result, &cp)
		}
	}
	return result, nil
}

type mockHTTPDispatcher struct {
	statusCode int
	err        error
	calls      []string // URLs appelees
}

func (m *mockHTTPDispatcher) Dispatch(_ context.Context, url string, _ []byte, _ string) (int, error) {
	m.calls = append(m.calls, url)
	return m.statusCode, m.err
}

// --- Tests DeliverEvent ---

func TestDeliverEvent_Success(t *testing.T) {
	ep := &domain.WebhookEndpoint{
		ID:          "ep-1",
		WorkspaceID: "ws-1",
		URL:         "https://example.com/webhook",
		Secret:      "secret",
		Events:      []string{"message.delivered"},
		Active:      true,
	}
	endpointRepo := &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{ep}}
	deliveryRepo := &mockDeliveryRepo{}
	disp := &mockHTTPDispatcher{statusCode: http.StatusOK}

	uc := &DeliverEvent{
		Endpoints:  endpointRepo,
		Deliveries: deliveryRepo,
		Dispatcher: disp,
	}

	payload := []byte(`{"event":"message.delivered","workspace_id":"ws-1"}`)
	if err := uc.Execute(context.Background(), "ws-1", "message.delivered", payload); err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// Une delivery doit etre persistee.
	if len(deliveryRepo.deliveries) != 1 {
		t.Fatalf("attendu 1 delivery, obtenu %d", len(deliveryRepo.deliveries))
	}

	// Le statut doit etre Delivered (reponse 200).
	d := deliveryRepo.deliveries[0]
	if d.Status != domain.StatusDelivered {
		t.Errorf("status attendu %s, obtenu %s", domain.StatusDelivered, d.Status)
	}

	// L'URL cible doit avoir ete appelee.
	if len(disp.calls) != 1 || disp.calls[0] != ep.URL {
		t.Errorf("attendu 1 appel vers %s, obtenu %v", ep.URL, disp.calls)
	}
}

func TestDeliverEvent_DispatchFailed(t *testing.T) {
	ep := &domain.WebhookEndpoint{
		ID:          "ep-2",
		WorkspaceID: "ws-2",
		URL:         "https://example.com/webhook",
		Secret:      "secret",
		Events:      []string{"message.failed"},
		Active:      true,
	}
	endpointRepo := &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{ep}}
	deliveryRepo := &mockDeliveryRepo{}
	// Reponse non-2xx : 503.
	disp := &mockHTTPDispatcher{statusCode: http.StatusServiceUnavailable}

	uc := &DeliverEvent{
		Endpoints:  endpointRepo,
		Deliveries: deliveryRepo,
		Dispatcher: disp,
	}

	payload := []byte(`{"event":"message.failed","workspace_id":"ws-2"}`)
	if err := uc.Execute(context.Background(), "ws-2", "message.failed", payload); err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// La delivery doit etre persistee avec le statut Failed.
	if len(deliveryRepo.deliveries) != 1 {
		t.Fatalf("attendu 1 delivery, obtenu %d", len(deliveryRepo.deliveries))
	}

	d := deliveryRepo.deliveries[0]
	if d.Status != domain.StatusFailed {
		t.Errorf("status attendu %s, obtenu %s", domain.StatusFailed, d.Status)
	}

	// NextRetryAt doit etre programme.
	if d.NextRetryAt.IsZero() {
		t.Error("NextRetryAt doit etre programme apres un echec")
	}
}

func TestDeliverEvent_NoEndpoints(t *testing.T) {
	// Aucun endpoint abonne a l'evenement : rien a faire.
	endpointRepo := &mockEndpointRepo{endpoints: nil}
	deliveryRepo := &mockDeliveryRepo{}
	disp := &mockHTTPDispatcher{statusCode: http.StatusOK}

	uc := &DeliverEvent{
		Endpoints:  endpointRepo,
		Deliveries: deliveryRepo,
		Dispatcher: disp,
	}

	if err := uc.Execute(context.Background(), "ws-3", "message.delivered", []byte(`{}`)); err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// Aucune delivery ne doit etre creee.
	if len(deliveryRepo.deliveries) != 0 {
		t.Errorf("attendu 0 delivery, obtenu %d", len(deliveryRepo.deliveries))
	}

	// Le dispatcher ne doit pas etre appele.
	if len(disp.calls) != 0 {
		t.Errorf("attendu 0 appel dispatcher, obtenu %v", disp.calls)
	}
}

func TestDeliverEvent_InactiveEndpointIgnored(t *testing.T) {
	// Endpoint inactif : il ne doit pas recevoir l'evenement.
	epInactif := &domain.WebhookEndpoint{
		ID:          "ep-inactif",
		WorkspaceID: "ws-4",
		URL:         "https://example.com/webhook",
		Secret:      "secret",
		Events:      []string{"message.delivered"},
		Active:      false, // inactif
	}
	// FindByWorkspaceAndEvent respecte Active via Subscribes() dans le mock.
	endpointRepo := &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{epInactif}}
	deliveryRepo := &mockDeliveryRepo{}
	disp := &mockHTTPDispatcher{statusCode: http.StatusOK}

	uc := &DeliverEvent{
		Endpoints:  endpointRepo,
		Deliveries: deliveryRepo,
		Dispatcher: disp,
	}

	if err := uc.Execute(context.Background(), "ws-4", "message.delivered", []byte(`{}`)); err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// Le mock filtre sur Subscribes() qui retourne false pour un endpoint inactif.
	// Aucune delivery ne doit etre creee.
	if len(deliveryRepo.deliveries) != 0 {
		t.Errorf("attendu 0 delivery (endpoint inactif), obtenu %d", len(deliveryRepo.deliveries))
	}
}
