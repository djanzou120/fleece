package providers

import (
	"context"
	"testing"

	"fleece/src/provider/internal/domain"
)

// domainOutboundMsg cree un OutboundMessage minimal pour les tests.
func domainOutboundMsg(id, recipient, content string) domain.OutboundMessage {
	return domain.OutboundMessage{
		Id:         id,
		ProviderId: "whatsapp-meta",
		Channel:    "whatsapp",
		Recipient:  recipient,
		Content:    content,
	}
}

func TestWhatsAppMeta_Send_ReturnsNonEmptyExternalID(t *testing.T) {
	adapter := NewWhatsAppMeta("http://stub.test", "token-test", "phone-123")

	externalId, err := adapter.Send(context.Background(), domainOutboundMsg("msg-1", "+237600000001", "Hello"))
	if err != nil {
		t.Fatalf("Send inattendu: %v", err)
	}
	if externalId == "" {
		t.Error("externalId doit etre non vide")
	}
}

func TestWhatsAppMeta_Send_EmptyRecipient_ReturnsError(t *testing.T) {
	adapter := NewWhatsAppMeta("http://stub.test", "token-test", "phone-123")

	_, err := adapter.Send(context.Background(), domainOutboundMsg("msg-2", "", "Hello"))
	if err == nil {
		t.Fatal("attendu une erreur pour recipient vide")
	}
}

func TestWhatsAppMeta_EstimateCost_ReturnsValidMoney(t *testing.T) {
	adapter := NewWhatsAppMeta("http://stub.test", "token-test", "phone-123")

	money, err := adapter.EstimateCost(context.Background(), "whatsapp", "CM")
	if err != nil {
		t.Fatalf("EstimateCost inattendu: %v", err)
	}
	if money.Amount <= 0 {
		t.Errorf("Amount doit etre positif, obtenu %d", money.Amount)
	}
	if money.Currency == "" {
		t.Error("Currency doit etre non vide")
	}
}

func TestWhatsAppMeta_EstimateCost_EuropeReturnsEUR(t *testing.T) {
	adapter := NewWhatsAppMeta("http://stub.test", "token-test", "phone-123")

	money, err := adapter.EstimateCost(context.Background(), "whatsapp", "FR")
	if err != nil {
		t.Fatalf("EstimateCost inattendu: %v", err)
	}
	if money.Currency != "EUR" {
		t.Errorf("attendu EUR pour FR, obtenu %s", money.Currency)
	}
}

func TestWhatsAppMeta_GetDeliveryStatus_ReturnsStatus(t *testing.T) {
	adapter := NewWhatsAppMeta("http://stub.test", "token-test", "phone-123")

	status, err := adapter.GetDeliveryStatus(context.Background(), "wamid.HBgL12345")
	if err != nil {
		t.Fatalf("GetDeliveryStatus inattendu: %v", err)
	}
	if status == "" {
		t.Error("status doit etre non vide")
	}
}
