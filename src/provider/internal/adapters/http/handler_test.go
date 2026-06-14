package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fleece/src/provider/internal/domain"
)

// --- Mocks des use cases ---

type mockDispatch struct {
	externalId string
	err        error
	calledWith domain.OutboundMessage
}

func (m *mockDispatch) Execute(_ context.Context, msg domain.OutboundMessage) (string, error) {
	m.calledWith = msg
	return m.externalId, m.err
}

type mockHandleDLR struct {
	err        error
	calledWith struct {
		externalId string
		status     domain.DeliveryStatus
	}
}

func (m *mockHandleDLR) Execute(_ context.Context, externalId string, status domain.DeliveryStatus) error {
	m.calledWith.externalId = externalId
	m.calledWith.status = status
	return m.err
}

// Assertions de conformite a la compilation.
var _ dispatchUseCase = (*mockDispatch)(nil)
var _ handleDLRUseCase = (*mockHandleDLR)(nil)

// --- Helpers ---

func newHandler(d dispatchUseCase, dlr handleDLRUseCase) *ProviderHandler {
	return NewProviderHandler(d, dlr)
}

func postJSON(t *testing.T, handler http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// =============================================================================
// Tests POST /dispatch
// =============================================================================

func TestHandler_Dispatch_Success(t *testing.T) {
	d := &mockDispatch{externalId: "ext-abc-123"}
	h := newHandler(d, &mockHandleDLR{})

	reqBody := DispatchRequest{
		WorkspaceID: "ws-1",
		ProviderID:  "whatsapp-meta",
		MessageID:   "msg-uuid-1",
		Channel:     "whatsapp",
		Recipient:   "+237600000001",
		Content:     "Hello Fleece",
	}

	w := postJSON(t, h.Dispatch, reqBody)

	if w.Code != http.StatusOK {
		t.Errorf("attendu 200, obtenu %d", w.Code)
	}

	var resp DispatchResponse
	decodeResponse(t, w, &resp)

	if resp.ExternalID != "ext-abc-123" {
		t.Errorf("attendu external_id='ext-abc-123', obtenu '%s'", resp.ExternalID)
	}

	// Verifier que le use case a recu les bons champs.
	if d.calledWith.Id != "msg-uuid-1" {
		t.Errorf("MessageID mal transmis: %s", d.calledWith.Id)
	}
	if d.calledWith.ProviderId != "whatsapp-meta" {
		t.Errorf("ProviderID mal transmis: %s", d.calledWith.ProviderId)
	}
}

func TestHandler_Dispatch_InvalidJSON(t *testing.T) {
	h := newHandler(&mockDispatch{}, &mockHandleDLR{})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{invalid-json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Dispatch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", w.Code)
	}
}

func TestHandler_Dispatch_MissingWorkspaceID(t *testing.T) {
	h := newHandler(&mockDispatch{}, &mockHandleDLR{})

	reqBody := DispatchRequest{
		// WorkspaceID manquant
		ProviderID: "whatsapp-meta",
		MessageID:  "msg-uuid-2",
		Channel:    "whatsapp",
		Recipient:  "+237600000001",
		Content:    "Hello",
	}

	w := postJSON(t, h.Dispatch, reqBody)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", w.Code)
	}

	var resp errorResponse
	decodeResponse(t, w, &resp)
	if resp.Error == "" {
		t.Error("attendu message d'erreur non vide")
	}
}

func TestHandler_Dispatch_MissingProviderID(t *testing.T) {
	h := newHandler(&mockDispatch{}, &mockHandleDLR{})

	reqBody := DispatchRequest{
		WorkspaceID: "ws-1",
		// ProviderID manquant
		MessageID: "msg-uuid-3",
		Channel:   "whatsapp",
		Recipient: "+237600000001",
		Content:   "Hello",
	}

	w := postJSON(t, h.Dispatch, reqBody)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", w.Code)
	}
}

func TestHandler_Dispatch_ProviderNotFound_Returns404(t *testing.T) {
	d := &mockDispatch{err: fmt.Errorf("dispatch: %w: absent", domain.ErrProviderNotFound)}
	h := newHandler(d, &mockHandleDLR{})

	reqBody := DispatchRequest{
		WorkspaceID: "ws-1",
		ProviderID:  "absent",
		MessageID:   "msg-uuid-4",
		Channel:     "sms",
		Recipient:   "+237600000001",
		Content:     "Hello",
	}

	w := postJSON(t, h.Dispatch, reqBody)

	if w.Code != http.StatusNotFound {
		t.Errorf("attendu 404, obtenu %d", w.Code)
	}
}

func TestHandler_Dispatch_SendFailed_Returns502(t *testing.T) {
	d := &mockDispatch{err: fmt.Errorf("dispatch: %w: echec", domain.ErrSendFailed)}
	h := newHandler(d, &mockHandleDLR{})

	reqBody := DispatchRequest{
		WorkspaceID: "ws-1",
		ProviderID:  "provider-1",
		MessageID:   "msg-uuid-5",
		Channel:     "sms",
		Recipient:   "+237600000001",
		Content:     "Hello",
	}

	w := postJSON(t, h.Dispatch, reqBody)

	if w.Code != http.StatusBadGateway {
		t.Errorf("attendu 502, obtenu %d", w.Code)
	}
}

func TestHandler_Dispatch_ProviderUnavailable_Returns503(t *testing.T) {
	d := &mockDispatch{err: domain.ErrProviderUnavailable}
	h := newHandler(d, &mockHandleDLR{})

	reqBody := DispatchRequest{
		WorkspaceID: "ws-1",
		ProviderID:  "provider-1",
		MessageID:   "msg-uuid-6",
		Channel:     "sms",
		Recipient:   "+237600000001",
		Content:     "Hello",
	}

	w := postJSON(t, h.Dispatch, reqBody)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("attendu 503, obtenu %d", w.Code)
	}
}

func TestHandler_Dispatch_UnknownError_Returns500(t *testing.T) {
	d := &mockDispatch{err: errors.New("erreur inconnue")}
	h := newHandler(d, &mockHandleDLR{})

	reqBody := DispatchRequest{
		WorkspaceID: "ws-1",
		ProviderID:  "provider-1",
		MessageID:   "msg-uuid-7",
		Channel:     "sms",
		Recipient:   "+237600000001",
		Content:     "Hello",
	}

	w := postJSON(t, h.Dispatch, reqBody)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("attendu 500, obtenu %d", w.Code)
	}
}

// =============================================================================
// Tests POST /dlr
// =============================================================================

func TestHandler_DLR_Success_Delivered(t *testing.T) {
	dlr := &mockHandleDLR{}
	h := newHandler(&mockDispatch{}, dlr)

	reqBody := DLRRequest{
		ExternalID: "ext-wamid-123",
		Status:     "delivered",
	}

	w := postJSON(t, h.DLR, reqBody)

	if w.Code != http.StatusNoContent {
		t.Errorf("attendu 204, obtenu %d", w.Code)
	}
	if dlr.calledWith.externalId != "ext-wamid-123" {
		t.Errorf("externalId mal transmis: %s", dlr.calledWith.externalId)
	}
	if dlr.calledWith.status != domain.StatusDelivered {
		t.Errorf("status mal transmis: %s", dlr.calledWith.status)
	}
}

func TestHandler_DLR_InvalidJSON(t *testing.T) {
	h := newHandler(&mockDispatch{}, &mockHandleDLR{})

	req := httptest.NewRequest(http.MethodPost, "/dlr", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.DLR(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", w.Code)
	}
}

func TestHandler_DLR_MissingExternalID(t *testing.T) {
	h := newHandler(&mockDispatch{}, &mockHandleDLR{})

	reqBody := DLRRequest{
		// ExternalID manquant
		Status: "delivered",
	}

	w := postJSON(t, h.DLR, reqBody)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", w.Code)
	}
}

func TestHandler_DLR_MissingStatus(t *testing.T) {
	h := newHandler(&mockDispatch{}, &mockHandleDLR{})

	reqBody := DLRRequest{
		ExternalID: "ext-123",
		// Status manquant
	}

	w := postJSON(t, h.DLR, reqBody)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", w.Code)
	}
}

func TestHandler_DLR_UseCaseError_Returns500(t *testing.T) {
	dlr := &mockHandleDLR{err: errors.New("statut DLR invalide: unknown")}
	h := newHandler(&mockDispatch{}, dlr)

	reqBody := DLRRequest{
		ExternalID: "ext-123",
		Status:     "unknown_status",
	}

	w := postJSON(t, h.DLR, reqBody)

	// handleError ne connait pas "statut DLR invalide" → 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("attendu 500, obtenu %d", w.Code)
	}
}

func TestHandler_DLR_Failed_Returns204(t *testing.T) {
	dlr := &mockHandleDLR{}
	h := newHandler(&mockDispatch{}, dlr)

	reqBody := DLRRequest{
		ExternalID: "SM-twilio-456",
		Status:     "failed",
	}

	w := postJSON(t, h.DLR, reqBody)

	if w.Code != http.StatusNoContent {
		t.Errorf("attendu 204, obtenu %d", w.Code)
	}
	if dlr.calledWith.status != domain.StatusFailed {
		t.Errorf("attendu StatusFailed, obtenu %s", dlr.calledWith.status)
	}
}
