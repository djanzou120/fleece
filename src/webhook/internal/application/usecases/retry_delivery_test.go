package usecases

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"fleece/src/webhook/internal/domain"
)

// TestBackoffFor verifie la fonction backoffFor pour les tentatives 1..5.
func TestBackoffFor(t *testing.T) {
	cases := []struct {
		attempts int
		expected time.Duration
	}{
		{1, 1 * time.Minute},
		{2, 5 * time.Minute},
		{3, 30 * time.Minute},
		{4, 2 * time.Hour},
		{5, 8 * time.Hour},
		{6, 8 * time.Hour}, // Valeur plafond pour attempts > 5.
		{10, 8 * time.Hour},
	}

	for _, tc := range cases {
		got := backoffFor(tc.attempts)
		if got != tc.expected {
			t.Errorf("backoffFor(%d) = %v, attendu %v", tc.attempts, got, tc.expected)
		}
	}
}

// TestRetryDelivery_Success verifie qu'une delivery en echec passe a Delivered apres succes.
func TestRetryDelivery_Success(t *testing.T) {
	ep := &domain.WebhookEndpoint{
		ID:     "ep-1",
		URL:    "https://example.com/webhook",
		Secret: "secret",
		Active: true,
	}
	delivery := &domain.WebhookDelivery{
		ID:         1,
		EndpointID: "ep-1",
		Event:      "message.delivered",
		Payload:    []byte(`{"event":"message.delivered"}`),
		Status:     domain.StatusFailed,
		Attempts:   1,
	}

	endpointRepo := &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{ep}}
	deliveryRepo := &mockDeliveryRepo{deliveries: []*domain.WebhookDelivery{delivery}}
	disp := &mockHTTPDispatcher{statusCode: http.StatusOK}

	uc := &RetryDelivery{
		Deliveries: deliveryRepo,
		Endpoints:  endpointRepo,
		Dispatcher: disp,
	}

	if err := uc.Execute(context.Background(), 1); err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// La delivery en memoire doit avoir ete mise a jour.
	// La derniere delivery sauvegardee doit etre Delivered.
	last := deliveryRepo.deliveries[len(deliveryRepo.deliveries)-1]
	if last.Status != domain.StatusDelivered {
		t.Errorf("status attendu %s, obtenu %s", domain.StatusDelivered, last.Status)
	}
	if last.Attempts != 2 {
		t.Errorf("attempts attendu 2, obtenu %d", last.Attempts)
	}
}

// TestRetryDelivery_Exhausted verifie le passage en Exhausted apres maxAttempts echecs.
func TestRetryDelivery_Exhausted(t *testing.T) {
	ep := &domain.WebhookEndpoint{
		ID:     "ep-2",
		URL:    "https://example.com/webhook",
		Secret: "secret",
		Active: true,
	}
	// Delivery deja a maxAttempts (5) tentatives.
	delivery := &domain.WebhookDelivery{
		ID:         2,
		EndpointID: "ep-2",
		Event:      "message.failed",
		Payload:    []byte(`{}`),
		Status:     domain.StatusFailed,
		Attempts:   maxAttempts, // 5
	}

	endpointRepo := &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{ep}}
	deliveryRepo := &mockDeliveryRepo{deliveries: []*domain.WebhookDelivery{delivery}}
	disp := &mockHTTPDispatcher{statusCode: http.StatusServiceUnavailable}

	uc := &RetryDelivery{
		Deliveries: deliveryRepo,
		Endpoints:  endpointRepo,
		Dispatcher: disp,
	}

	err := uc.Execute(context.Background(), 2)
	if err == nil {
		t.Fatal("attendu ErrMaxRetriesExceeded, obtenu nil")
	}
	if !errors.Is(err, domain.ErrMaxRetriesExceeded) {
		t.Errorf("attendu ErrMaxRetriesExceeded, obtenu %v", err)
	}

	// La delivery doit etre passee en Exhausted.
	last := deliveryRepo.deliveries[len(deliveryRepo.deliveries)-1]
	if last.Status != domain.StatusExhausted {
		t.Errorf("status attendu %s, obtenu %s", domain.StatusExhausted, last.Status)
	}
}

// TestRetryDelivery_ProgressiveExhaustion simule maxAttempts echecs successifs
// et verifie que la 6eme tentative retourne ErrMaxRetriesExceeded.
func TestRetryDelivery_ProgressiveExhaustion(t *testing.T) {
	ep := &domain.WebhookEndpoint{
		ID:     "ep-3",
		URL:    "https://example.com/webhook",
		Secret: "secret",
		Active: true,
	}

	endpointRepo := &mockEndpointRepo{endpoints: []*domain.WebhookEndpoint{ep}}
	disp := &mockHTTPDispatcher{statusCode: http.StatusInternalServerError}

	// Delivery initiale apres le premier echec (attempts=1).
	delivery := &domain.WebhookDelivery{
		ID:         3,
		EndpointID: "ep-3",
		Event:      "wallet.debited",
		Payload:    []byte(`{}`),
		Status:     domain.StatusFailed,
		Attempts:   1,
	}
	deliveryRepo := &mockDeliveryRepo{deliveries: []*domain.WebhookDelivery{delivery}}

	uc := &RetryDelivery{
		Deliveries: deliveryRepo,
		Endpoints:  endpointRepo,
		Dispatcher: disp,
	}

	// Tentatives 2 a 4 : doivent echouer sans ErrMaxRetriesExceeded.
	for i := 2; i < maxAttempts; i++ {
		if err := uc.Execute(context.Background(), 3); err != nil {
			t.Fatalf("tentative %d: erreur inattendue: %v", i, err)
		}
		// Mettre a jour la livraison en memoire pour la prochaine tentative.
		last := deliveryRepo.deliveries[len(deliveryRepo.deliveries)-1]
		// Synchroniser l'etat de la delivery initiale avec la derniere sauvegardee.
		delivery.Attempts = last.Attempts
		delivery.Status = last.Status
		deliveryRepo.deliveries[0] = delivery
	}

	// Tentative maxAttempts : doit retourner ErrMaxRetriesExceeded.
	err := uc.Execute(context.Background(), 3)
	if err == nil {
		t.Fatal("attendu ErrMaxRetriesExceeded pour la derniere tentative")
	}
	if !errors.Is(err, domain.ErrMaxRetriesExceeded) {
		t.Errorf("attendu ErrMaxRetriesExceeded, obtenu %v", err)
	}
}
