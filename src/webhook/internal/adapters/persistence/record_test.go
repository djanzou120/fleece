package persistence

import (
	"testing"
	"time"

	"fleece/src/webhook/internal/domain"
)

// --- Tests des mappings record.go (colonnes ajoutees par migration 0010) ---

// TestDeliveryRecord_ToEntity_WithPayloadAndNextRetryAt verifie que toEntity mappe
// correctement Payload et NextRetryAt quand NextRetryAt est non-nil.
func TestDeliveryRecord_ToEntity_WithPayloadAndNextRetryAt(t *testing.T) {
	retryAt := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	payload := []byte(`{"event":"message.delivered"}`)

	rec := deliveryRecord{
		ID:          42,
		EndpointID:  "ep-abc",
		Event:       "message.delivered",
		Status:      string(domain.StatusFailed),
		Attempts:    2,
		Payload:     payload,
		NextRetryAt: &retryAt,
		CreatedAt:   time.Now(),
	}

	entity := rec.toEntity()

	if entity.ID != 42 {
		t.Errorf("ID attendu 42, obtenu %d", entity.ID)
	}
	if string(entity.Payload) != string(payload) {
		t.Errorf("Payload attendu %s, obtenu %s", payload, entity.Payload)
	}
	if !entity.NextRetryAt.Equal(retryAt) {
		t.Errorf("NextRetryAt attendu %v, obtenu %v", retryAt, entity.NextRetryAt)
	}
	if entity.Status != domain.StatusFailed {
		t.Errorf("Status attendu %s, obtenu %s", domain.StatusFailed, entity.Status)
	}
}

// TestDeliveryRecord_ToEntity_NilNextRetryAt verifie que NextRetryAt reste zero
// quand le champ nullable est nil.
func TestDeliveryRecord_ToEntity_NilNextRetryAt(t *testing.T) {
	rec := deliveryRecord{
		ID:          1,
		EndpointID:  "ep-1",
		Event:       "message.delivered",
		Status:      string(domain.StatusDelivered),
		Attempts:    1,
		Payload:     nil,
		NextRetryAt: nil, // nullable null
	}

	entity := rec.toEntity()

	if !entity.NextRetryAt.IsZero() {
		t.Errorf("NextRetryAt attendu zero, obtenu %v", entity.NextRetryAt)
	}
	if entity.Payload != nil {
		t.Errorf("Payload attendu nil, obtenu %v", entity.Payload)
	}
}

// TestFromDelivery_NonZeroNextRetryAt verifie que fromDelivery positionne NextRetryAt
// quand la valeur est non-zero.
func TestFromDelivery_NonZeroNextRetryAt(t *testing.T) {
	retryAt := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	payload := []byte(`{"x":1}`)

	d := &domain.WebhookDelivery{
		ID:          10,
		EndpointID:  "ep-x",
		Event:       "wallet.debited",
		Status:      domain.StatusFailed,
		Attempts:    1,
		Payload:     payload,
		NextRetryAt: retryAt,
	}

	rec := fromDelivery(d)

	if rec.NextRetryAt == nil {
		t.Fatal("NextRetryAt attendu non-nil pour une valeur non-zero")
	}
	if !rec.NextRetryAt.Equal(retryAt) {
		t.Errorf("NextRetryAt attendu %v, obtenu %v", retryAt, *rec.NextRetryAt)
	}
	if string(rec.Payload) != string(payload) {
		t.Errorf("Payload attendu %s, obtenu %s", payload, rec.Payload)
	}
}

// TestFromDelivery_ZeroNextRetryAt verifie que fromDelivery laisse NextRetryAt nil
// quand la valeur domaine est zero.
func TestFromDelivery_ZeroNextRetryAt(t *testing.T) {
	d := &domain.WebhookDelivery{
		ID:          11,
		EndpointID:  "ep-y",
		Event:       "message.sent",
		Status:      domain.StatusDelivered,
		Attempts:    1,
		Payload:     []byte(`{}`),
		NextRetryAt: time.Time{}, // zero
	}

	rec := fromDelivery(d)

	if rec.NextRetryAt != nil {
		t.Errorf("NextRetryAt attendu nil pour une valeur zero, obtenu %v", rec.NextRetryAt)
	}
}

// TestDeliveryRecord_RoundTrip_PayloadAndNextRetryAt verifie que
// fromDelivery -> toEntity preserve Payload et NextRetryAt sans perte.
func TestDeliveryRecord_RoundTrip_PayloadAndNextRetryAt(t *testing.T) {
	retryAt := time.Date(2026, 7, 1, 12, 30, 0, 0, time.UTC)
	payload := []byte(`{"event":"message.failed","workspace_id":"ws-99"}`)

	original := &domain.WebhookDelivery{
		ID:          99,
		EndpointID:  "ep-99",
		Event:       "message.failed",
		Status:      domain.StatusFailed,
		Attempts:    3,
		Payload:     payload,
		NextRetryAt: retryAt,
	}

	rec := fromDelivery(original)
	reconstructed := rec.toEntity()

	if string(reconstructed.Payload) != string(original.Payload) {
		t.Errorf("Payload round-trip: attendu %s, obtenu %s", original.Payload, reconstructed.Payload)
	}
	if !reconstructed.NextRetryAt.Equal(original.NextRetryAt) {
		t.Errorf("NextRetryAt round-trip: attendu %v, obtenu %v", original.NextRetryAt, reconstructed.NextRetryAt)
	}
	if reconstructed.Status != original.Status {
		t.Errorf("Status round-trip: attendu %s, obtenu %s", original.Status, reconstructed.Status)
	}
}

// --- Tests des mappings endpointRecord (colonne active) ---

// TestEndpointRecord_ToEntity_Active verifie que le champ Active est mappe
// dans les deux sens (active=true et active=false).
func TestEndpointRecord_ToEntity_Active(t *testing.T) {
	cases := []struct {
		active bool
	}{
		{true},
		{false},
	}
	for _, tc := range cases {
		rec := endpointRecord{
			ID:          "ep-1",
			WorkspaceID: "ws-1",
			URL:         "https://example.com/wh",
			Secret:      "sec",
			Events:      []string{"message.delivered"},
			Active:      tc.active,
		}
		entity := rec.toEntity()
		if entity.Active != tc.active {
			t.Errorf("toEntity: Active attendu %v, obtenu %v", tc.active, entity.Active)
		}

		// Aller-retour fromEndpoint.
		rec2 := fromEndpoint(entity)
		if rec2.Active != tc.active {
			t.Errorf("fromEndpoint: Active attendu %v, obtenu %v", tc.active, rec2.Active)
		}
	}
}
