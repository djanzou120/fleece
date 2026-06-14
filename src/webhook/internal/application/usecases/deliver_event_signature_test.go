package usecases

import (
	"context"
	"net/http"
	"testing"

	"fleece/src/webhook/internal/domain"
)

// mockSignatureCapturingDispatcher capture la signature envoyee pour la verifier.
type mockSignatureCapturingDispatcher struct {
	statusCode int
	lastSig    string
	lastURL    string
	lastBody   []byte
}

func (m *mockSignatureCapturingDispatcher) Dispatch(_ context.Context, url string, body []byte, signature string) (int, error) {
	m.lastURL = url
	m.lastBody = body
	m.lastSig = signature
	return m.statusCode, nil
}

// TestDeliverEvent_SignatureHMACValide verifie que DeliverEvent calcule et transmet
// une signature HMAC-SHA256 valide (non vide, verifiable avec domain.Verify).
//
// C'est le critere securite principal : la signature doit etre calculee avec le
// secret de l'endpoint et etre verifiable par le destinataire.
func TestDeliverEvent_SignatureHMACValide(t *testing.T) {
	secret := "secret-hmac-test-12345"
	ep := &domain.WebhookEndpoint{
		ID:          "ep-sig-1",
		WorkspaceID: "ws-sig-1",
		URL:         "https://example.com/hook",
		Secret:      secret,
		Events:      []string{"message.delivered"},
		Active:      true,
	}
	endpointRepo := &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{ep}}
	deliveryRepo := &mockDeliveryRepo{}
	disp := &mockSignatureCapturingDispatcher{statusCode: http.StatusOK}

	uc := &DeliverEvent{
		Endpoints:  endpointRepo,
		Deliveries: deliveryRepo,
		Dispatcher: disp,
	}

	payload := []byte(`{"event":"message.delivered","workspace_id":"ws-sig-1","message_id":"m-001"}`)

	if err := uc.Execute(context.Background(), "ws-sig-1", "message.delivered", payload); err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// La signature ne doit pas etre vide.
	if disp.lastSig == "" {
		t.Fatal("la signature transmise au dispatcher ne doit pas etre vide")
	}

	// La signature doit etre un hexadecimal HMAC-SHA256 de 64 chars (256 bits).
	if len(disp.lastSig) != 64 {
		t.Errorf("signature attendue de longueur 64 (HMAC-SHA256 hex), obtenue %d: %q",
			len(disp.lastSig), disp.lastSig)
	}

	// La signature doit etre verifiable avec domain.Verify (comparaison en temps constant).
	if !domain.Verify(secret, payload, disp.lastSig) {
		t.Errorf("domain.Verify echoue : la signature %q n'est pas valide pour ce secret et ce payload",
			disp.lastSig)
	}
}

// TestDeliverEvent_SignatureInvalidePourMauvaisSecret verifie que la signature transmise
// ne peut pas etre verifiee avec un secret different.
func TestDeliverEvent_SignatureInvalidePourMauvaisSecret(t *testing.T) {
	secret := "bon-secret"
	ep := &domain.WebhookEndpoint{
		ID:          "ep-sig-2",
		WorkspaceID: "ws-sig-2",
		URL:         "https://example.com/hook",
		Secret:      secret,
		Events:      []string{"wallet.debited"},
		Active:      true,
	}
	endpointRepo := &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{ep}}
	deliveryRepo := &mockDeliveryRepo{}
	disp := &mockSignatureCapturingDispatcher{statusCode: http.StatusOK}

	uc := &DeliverEvent{
		Endpoints:  endpointRepo,
		Deliveries: deliveryRepo,
		Dispatcher: disp,
	}

	payload := []byte(`{"event":"wallet.debited","workspace_id":"ws-sig-2"}`)
	if err := uc.Execute(context.Background(), "ws-sig-2", "wallet.debited", payload); err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// Avec un mauvais secret, Verify doit retourner false.
	if domain.Verify("mauvais-secret", payload, disp.lastSig) {
		t.Error("domain.Verify ne doit pas valider la signature avec un mauvais secret")
	}
}

// TestDeliverEvent_SignatureInvalidePourPayloadModifie verifie qu'un payload altere
// ne passe pas la verification de signature.
func TestDeliverEvent_SignatureInvalidePourPayloadModifie(t *testing.T) {
	secret := "secret-tamper"
	ep := &domain.WebhookEndpoint{
		ID:          "ep-sig-3",
		WorkspaceID: "ws-sig-3",
		URL:         "https://example.com/hook",
		Secret:      secret,
		Events:      []string{"message.sent"},
		Active:      true,
	}
	endpointRepo := &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{ep}}
	deliveryRepo := &mockDeliveryRepo{}
	disp := &mockSignatureCapturingDispatcher{statusCode: http.StatusOK}

	uc := &DeliverEvent{
		Endpoints:  endpointRepo,
		Deliveries: deliveryRepo,
		Dispatcher: disp,
	}

	originalPayload := []byte(`{"event":"message.sent","workspace_id":"ws-sig-3"}`)
	if err := uc.Execute(context.Background(), "ws-sig-3", "message.sent", originalPayload); err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	capturedSig := disp.lastSig

	// Un payload modifie ne doit pas passer la verification avec la signature originale.
	payloadAltere := []byte(`{"event":"message.delivered","workspace_id":"ws-sig-3"}`)
	if domain.Verify(secret, payloadAltere, capturedSig) {
		t.Error("domain.Verify ne doit pas valider la signature avec un payload altere")
	}
}

// TestDeliverEvent_SignatureDifferenteParEndpoint verifie que deux endpoints avec des
// secrets differents recoivent des signatures differentes pour le meme payload.
func TestDeliverEvent_SignatureDifferenteParEndpoint(t *testing.T) {
	payload := []byte(`{"event":"message.delivered","workspace_id":"ws-multi"}`)

	ep1 := &domain.WebhookEndpoint{
		ID: "ep-multi-1", WorkspaceID: "ws-multi",
		URL: "https://a.com/hook", Secret: "secret-A",
		Events: []string{"message.delivered"}, Active: true,
	}
	ep2 := &domain.WebhookEndpoint{
		ID: "ep-multi-2", WorkspaceID: "ws-multi",
		URL: "https://b.com/hook", Secret: "secret-B",
		Events: []string{"message.delivered"}, Active: true,
	}

	// Tester ep1
	disp1 := &mockSignatureCapturingDispatcher{statusCode: http.StatusOK}
	uc1 := &DeliverEvent{
		Endpoints:  &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{ep1}},
		Deliveries: &mockDeliveryRepo{},
		Dispatcher: disp1,
	}
	if err := uc1.Execute(context.Background(), "ws-multi", "message.delivered", payload); err != nil {
		t.Fatalf("Execute ep1 inattendu: %v", err)
	}

	// Tester ep2
	disp2 := &mockSignatureCapturingDispatcher{statusCode: http.StatusOK}
	uc2 := &DeliverEvent{
		Endpoints:  &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{ep2}},
		Deliveries: &mockDeliveryRepo{},
		Dispatcher: disp2,
	}
	if err := uc2.Execute(context.Background(), "ws-multi", "message.delivered", payload); err != nil {
		t.Fatalf("Execute ep2 inattendu: %v", err)
	}

	// Les signatures doivent etre differentes (secrets differents).
	if disp1.lastSig == disp2.lastSig {
		t.Error("deux endpoints avec des secrets differents doivent generer des signatures differentes")
	}

	// Chacune doit etre valide pour son propre secret.
	if !domain.Verify("secret-A", payload, disp1.lastSig) {
		t.Error("signature ep1 invalide avec secret-A")
	}
	if !domain.Verify("secret-B", payload, disp2.lastSig) {
		t.Error("signature ep2 invalide avec secret-B")
	}
	// Et invalide avec l'autre secret.
	if domain.Verify("secret-B", payload, disp1.lastSig) {
		t.Error("signature ep1 ne doit pas etre valide avec secret-B")
	}
}
