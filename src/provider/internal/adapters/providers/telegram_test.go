package providers

import (
	"context"
	"testing"

	"fleece/src/provider/internal/domain"
)

// --- Helpers ---

func telegramOutboundMsg(id, recipient, content string) domain.OutboundMessage {
	return domain.OutboundMessage{
		Id:         id,
		ProviderId: "telegram-bot",
		Channel:    "telegram",
		Recipient:  recipient,
		Content:    content,
	}
}

// --- Tests MapTelegramResponseStatus (mapping pur, sans I/O) ---

func TestMapTelegramResponseStatus_OkTrue_ReturnsSent(t *testing.T) {
	status := MapTelegramResponseStatus(true, 0)
	if status != domain.StatusSent {
		t.Errorf("ok=true: attendu %s, obtenu %s", domain.StatusSent, status)
	}
}

func TestMapTelegramResponseStatus_ErrorCode400_ReturnsRejected(t *testing.T) {
	status := MapTelegramResponseStatus(false, 400)
	if status != domain.StatusRejected {
		t.Errorf("errorCode=400: attendu %s, obtenu %s", domain.StatusRejected, status)
	}
}

func TestMapTelegramResponseStatus_ErrorCode403_ReturnsRejected(t *testing.T) {
	status := MapTelegramResponseStatus(false, 403)
	if status != domain.StatusRejected {
		t.Errorf("errorCode=403: attendu %s, obtenu %s", domain.StatusRejected, status)
	}
}

func TestMapTelegramResponseStatus_ErrorCode500_ReturnsFailed(t *testing.T) {
	status := MapTelegramResponseStatus(false, 500)
	if status != domain.StatusFailed {
		t.Errorf("errorCode=500: attendu %s, obtenu %s", domain.StatusFailed, status)
	}
}

func TestMapTelegramResponseStatus_ErrorCode503_ReturnsFailed(t *testing.T) {
	status := MapTelegramResponseStatus(false, 503)
	if status != domain.StatusFailed {
		t.Errorf("errorCode=503: attendu %s, obtenu %s", domain.StatusFailed, status)
	}
}

func TestMapTelegramResponseStatus_NetworkTimeout_ReturnsFailed(t *testing.T) {
	// Timeout reseau : par convention, errorCode = -1 (hors plage HTTP).
	status := MapTelegramResponseStatus(false, -1)
	if status != domain.StatusFailed {
		t.Errorf("errorCode=-1 (timeout): attendu %s, obtenu %s", domain.StatusFailed, status)
	}
}

func TestMapTelegramResponseStatus_UnknownErrorCode_ReturnsFailed(t *testing.T) {
	// Code inconnu (ex. 429 Too Many Requests) : fallback StatusFailed.
	status := MapTelegramResponseStatus(false, 429)
	if status != domain.StatusFailed {
		t.Errorf("errorCode=429: attendu %s, obtenu %s", domain.StatusFailed, status)
	}
}

// --- Tests TelegramBot.Send ---

func TestTelegramBot_Send_ReturnsNonEmptyExternalID(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	externalId, err := adapter.Send(context.Background(), telegramOutboundMsg("msg-tg-1", "123456789", "Bonjour"))
	if err != nil {
		t.Fatalf("Send inattendu: %v", err)
	}
	if externalId == "" {
		t.Error("externalId doit etre non vide")
	}
}

func TestTelegramBot_Send_EmptyRecipient_ReturnsError(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	_, err := adapter.Send(context.Background(), telegramOutboundMsg("msg-tg-2", "", "Bonjour"))
	if err == nil {
		t.Fatal("attendu une erreur pour recipient vide")
	}
}

func TestTelegramBot_Send_EmptyContent_ReturnsError(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	_, err := adapter.Send(context.Background(), telegramOutboundMsg("msg-tg-3", "123456789", ""))
	if err == nil {
		t.Fatal("attendu une erreur pour content vide")
	}
}

func TestTelegramBot_Send_EmptyRecipient_WrapsErrSendFailed(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	_, err := adapter.Send(context.Background(), telegramOutboundMsg("msg-tg-4", "", "Hello"))
	if err == nil {
		t.Fatal("attendu une erreur")
	}
	// Verifier que l'erreur enveloppe bien ErrSendFailed.
	if !isWrapping(err, domain.ErrSendFailed) {
		t.Errorf("attendu erreur wrappant ErrSendFailed, obtenu: %v", err)
	}
}

func TestTelegramBot_Send_TwoCallsReturnNonEmptyIDs(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	id1, err := adapter.Send(context.Background(), telegramOutboundMsg("msg-tg-5", "111", "Msg1"))
	if err != nil {
		t.Fatalf("premier Send: %v", err)
	}
	id2, err := adapter.Send(context.Background(), telegramOutboundMsg("msg-tg-6", "222", "Msg2"))
	if err != nil {
		t.Fatalf("deuxieme Send: %v", err)
	}
	if id1 == "" || id2 == "" {
		t.Error("les externalIds ne doivent pas etre vides")
	}
}

// --- Tests TelegramBot.EstimateCost ---

func TestTelegramBot_EstimateCost_AlwaysZeroAmount(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	// Telegram est gratuit : le montant doit etre 0 pour tous les pays.
	countries := []string{"CM", "SN", "FR", "BE", "CH", "US", "NG"}
	for _, country := range countries {
		money, err := adapter.EstimateCost(context.Background(), "telegram", country)
		if err != nil {
			t.Fatalf("EstimateCost(%s) inattendu: %v", country, err)
		}
		if money.Amount != 0 {
			t.Errorf("EstimateCost(%s): attendu Amount=0 (Telegram gratuit), obtenu %d", country, money.Amount)
		}
	}
}

func TestTelegramBot_EstimateCost_EuropeReturnsEUR(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	for _, country := range []string{"FR", "BE", "CH"} {
		money, err := adapter.EstimateCost(context.Background(), "telegram", country)
		if err != nil {
			t.Fatalf("EstimateCost(%s) inattendu: %v", country, err)
		}
		if money.Currency != "EUR" {
			t.Errorf("EstimateCost(%s): attendu EUR, obtenu %s", country, money.Currency)
		}
		if money.Amount != 0 {
			t.Errorf("EstimateCost(%s): attendu Amount=0, obtenu %d", country, money.Amount)
		}
	}
}

func TestTelegramBot_EstimateCost_AfricaDefaultReturnsXAF(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	money, err := adapter.EstimateCost(context.Background(), "telegram", "CM")
	if err != nil {
		t.Fatalf("EstimateCost(CM) inattendu: %v", err)
	}
	if money.Currency != "XAF" {
		t.Errorf("attendu XAF pour CM, obtenu %s", money.Currency)
	}
	if money.Amount != 0 {
		t.Errorf("attendu Amount=0 pour CM (Telegram gratuit), obtenu %d", money.Amount)
	}
}

func TestTelegramBot_EstimateCost_NonEuropeanCountryReturnsXAF(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	money, err := adapter.EstimateCost(context.Background(), "telegram", "NG")
	if err != nil {
		t.Fatalf("EstimateCost(NG) inattendu: %v", err)
	}
	if money.Currency != "XAF" {
		t.Errorf("attendu XAF pour NG, obtenu %s", money.Currency)
	}
}

// --- Tests TelegramBot.GetDeliveryStatus ---

func TestTelegramBot_GetDeliveryStatus_ReturnsStatusSent(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	status, err := adapter.GetDeliveryStatus(context.Background(), "tg1234567890")
	if err != nil {
		t.Fatalf("GetDeliveryStatus inattendu: %v", err)
	}
	if status != domain.StatusSent {
		t.Errorf("attendu %s, obtenu %s", domain.StatusSent, status)
	}
}

func TestTelegramBot_GetDeliveryStatus_EmptyExternalID_ReturnsError(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	_, err := adapter.GetDeliveryStatus(context.Background(), "")
	if err == nil {
		t.Fatal("attendu une erreur pour externalId vide")
	}
}

func TestTelegramBot_GetDeliveryStatus_EmptyExternalID_WrapsErrProviderUnavailable(t *testing.T) {
	adapter := NewTelegramBot("http://stub.test", "bot-token-test")

	_, err := adapter.GetDeliveryStatus(context.Background(), "")
	if err == nil {
		t.Fatal("attendu une erreur")
	}
	if !isWrapping(err, domain.ErrProviderUnavailable) {
		t.Errorf("attendu erreur wrappant ErrProviderUnavailable, obtenu: %v", err)
	}
}

// --- Constantes ---

func TestTelegramCapabilities_Constants(t *testing.T) {
	// Verifie les valeurs des constantes de capabilities (regression).
	if TelegramMaxMessageLength != 4096 {
		t.Errorf("TelegramMaxMessageLength attendu 4096, obtenu %d", TelegramMaxMessageLength)
	}
	if !TelegramSupportsRichMedia {
		t.Error("TelegramSupportsRichMedia doit etre true")
	}
	if TelegramRequiresTemplate {
		t.Error("TelegramRequiresTemplate doit etre false")
	}
	if TelegramChannel != "telegram" {
		t.Errorf("TelegramChannel attendu \"telegram\", obtenu %q", TelegramChannel)
	}
}

// --- Helpers internes ---

// isWrapping verifie si err enveloppe target (errors.Is).
func isWrapping(err error, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		// Unwrap manuel compatible stdlib (pas d'import errors pour rester minimal).
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
