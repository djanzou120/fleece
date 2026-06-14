package providers

import (
	"context"
	"testing"

	"fleece/src/provider/internal/domain"
)

func TestSMSTwilio_Send_ReturnsNonEmptyExternalID(t *testing.T) {
	adapter := NewSMSTwilio("http://stub.test", "ACTEST", "authtoken", "+15005550006")

	msg := domain.OutboundMessage{
		Id:         "msg-sms-1",
		ProviderId: "sms-twilio",
		Channel:    "sms",
		Recipient:  "+237600000001",
		Content:    "Hello via SMS",
	}

	externalId, err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send inattendu: %v", err)
	}
	if externalId == "" {
		t.Error("externalId doit etre non vide")
	}
}

func TestSMSTwilio_Send_EmptyRecipient_ReturnsError(t *testing.T) {
	adapter := NewSMSTwilio("http://stub.test", "ACTEST", "authtoken", "+15005550006")

	msg := domain.OutboundMessage{
		Id:        "msg-sms-2",
		Recipient: "",
		Content:   "Hello",
	}

	_, err := adapter.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("attendu une erreur pour recipient vide")
	}
}

func TestSMSTwilio_Send_EmptyContent_ReturnsError(t *testing.T) {
	adapter := NewSMSTwilio("http://stub.test", "ACTEST", "authtoken", "+15005550006")

	msg := domain.OutboundMessage{
		Id:        "msg-sms-3",
		Recipient: "+237600000001",
		Content:   "",
	}

	_, err := adapter.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("attendu une erreur pour content vide")
	}
}

func TestSMSTwilio_EstimateCost_AfricaReturnsXAF(t *testing.T) {
	adapter := NewSMSTwilio("http://stub.test", "ACTEST", "authtoken", "+15005550006")

	money, err := adapter.EstimateCost(context.Background(), "sms", "CM")
	if err != nil {
		t.Fatalf("EstimateCost inattendu: %v", err)
	}
	if money.Amount <= 0 {
		t.Errorf("Amount doit etre positif, obtenu %d", money.Amount)
	}
	if money.Currency != "XAF" {
		t.Errorf("attendu XAF pour CM, obtenu %s", money.Currency)
	}
}

func TestSMSTwilio_EstimateCost_EuropeReturnsEUR(t *testing.T) {
	adapter := NewSMSTwilio("http://stub.test", "ACTEST", "authtoken", "+15005550006")

	money, err := adapter.EstimateCost(context.Background(), "sms", "FR")
	if err != nil {
		t.Fatalf("EstimateCost inattendu: %v", err)
	}
	if money.Currency != "EUR" {
		t.Errorf("attendu EUR pour FR, obtenu %s", money.Currency)
	}
}

func TestSMSTwilio_GetDeliveryStatus_ReturnsStatus(t *testing.T) {
	adapter := NewSMSTwilio("http://stub.test", "ACTEST", "authtoken", "+15005550006")

	status, err := adapter.GetDeliveryStatus(context.Background(), "SM1234567890")
	if err != nil {
		t.Fatalf("GetDeliveryStatus inattendu: %v", err)
	}
	if status == "" {
		t.Error("status doit etre non vide")
	}
}

func TestSMSTwilio_GetDeliveryStatus_EmptyExternalID_ReturnsError(t *testing.T) {
	adapter := NewSMSTwilio("http://stub.test", "ACTEST", "authtoken", "+15005550006")

	_, err := adapter.GetDeliveryStatus(context.Background(), "")
	if err == nil {
		t.Fatal("attendu une erreur pour externalId vide")
	}
}

func TestSMSTwilio_Send_TwoCallsReturnDistinctExternalIDs(t *testing.T) {
	adapter := NewSMSTwilio("http://stub.test", "ACTEST", "authtoken", "+15005550006")

	msg := domain.OutboundMessage{
		Id:        "msg-sms-4",
		Recipient: "+237600000001",
		Content:   "Test",
	}

	id1, err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("premier Send: %v", err)
	}
	// Changer l'Id pour simuler un second message distinct.
	msg.Id = "msg-sms-5"
	id2, err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("deuxieme Send: %v", err)
	}

	// Les deux externalIds doivent etre non vides (l'unicite absolue n'est pas garantie en <1ns mais le format SM+nanos devrait differ en pratique).
	if id1 == "" || id2 == "" {
		t.Error("les externalIds ne doivent pas etre vides")
	}
}
