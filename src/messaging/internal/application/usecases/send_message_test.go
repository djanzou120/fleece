package usecases

import (
	"context"
	"errors"
	"testing"

	"fleece/src/messaging/internal/application/ports/output"
	"fleece/src/messaging/internal/domain"
)

// --- Mocks des ports ---

type mockRepo struct {
	saved []*domain.Message
	getErr error
}

func (m *mockRepo) Save(_ context.Context, msg *domain.Message) error {
	m.saved = append(m.saved, msg)
	return nil
}

func (m *mockRepo) Get(_ context.Context, id string) (*domain.Message, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, msg := range m.saved {
		if msg.ID == id {
			return msg, nil
		}
	}
	return nil, errors.New("not found")
}

type mockWallet struct {
	hasBalance bool
	hasErr     error
	debitErr   error
	refundErr  error
	debited    bool
	refunded   bool
}

func (m *mockWallet) HasBalance(_ context.Context, _ string) (bool, error) {
	return m.hasBalance, m.hasErr
}

func (m *mockWallet) Debit(_ context.Context, _, _ string) error {
	m.debited = true
	return m.debitErr
}

func (m *mockWallet) Refund(_ context.Context, _, _ string) error {
	m.refunded = true
	return m.refundErr
}

type mockRouting struct {
	attempts []output.RouteAttempt
	err      error
}

func (m *mockRouting) Decide(_ context.Context, _ *domain.Message) ([]output.RouteAttempt, error) {
	return m.attempts, m.err
}

type mockProvider struct {
	failUntil int // les N premiers appels echouent
	calls     int
}

func (m *mockProvider) Send(_ context.Context, _ *domain.Message, _ output.RouteAttempt) error {
	m.calls++
	if m.calls <= m.failUntil {
		return errors.New("provider error")
	}
	return nil
}

type mockPublisher struct {
	events []string
}

func (m *mockPublisher) Publish(_ context.Context, event string, _ *domain.Message) error {
	m.events = append(m.events, event)
	return nil
}

// --- Tests ---

func newTestMessage() *domain.Message {
	return domain.NewMessage("msg-1", "ws-1", "+33600000000", "Hello")
}

func TestSendMessage_Success(t *testing.T) {
	repo := &mockRepo{}
	wallet := &mockWallet{hasBalance: true}
	routing := &mockRouting{attempts: []output.RouteAttempt{
		{Channel: domain.ChannelSMS, Provider: "twilio"},
	}}
	provider := &mockProvider{}
	pub := &mockPublisher{}

	uc := SendMessage{
		Repo:      repo,
		Routing:   routing,
		Wallet:    wallet,
		Provider:  provider,
		Publisher: pub,
	}

	msg := newTestMessage()
	if err := uc.Execute(context.Background(), msg); err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	if msg.Status != domain.StatusSent {
		t.Errorf("statut attendu %s, obtenu %s", domain.StatusSent, msg.Status)
	}
	if !wallet.debited {
		t.Error("wallet aurait du etre debite")
	}
	if len(repo.saved) == 0 {
		t.Error("message aurait du etre sauvegarde")
	}
}

func TestSendMessage_InsufficientFunds(t *testing.T) {
	uc := SendMessage{
		Repo:      &mockRepo{},
		Routing:   &mockRouting{},
		Wallet:    &mockWallet{hasBalance: false},
		Provider:  &mockProvider{},
		Publisher: &mockPublisher{},
	}

	err := uc.Execute(context.Background(), newTestMessage())
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Errorf("attendu ErrInsufficientFunds, obtenu %v", err)
	}
}

func TestSendMessage_NoChannel(t *testing.T) {
	uc := SendMessage{
		Repo:      &mockRepo{},
		Routing:   &mockRouting{attempts: nil},
		Wallet:    &mockWallet{hasBalance: true},
		Provider:  &mockProvider{},
		Publisher: &mockPublisher{},
	}

	err := uc.Execute(context.Background(), newTestMessage())
	if !errors.Is(err, domain.ErrNoChannel) {
		t.Errorf("attendu domain.ErrNoChannel, obtenu %v", err)
	}
}

func TestSendMessage_FallbackSuccess(t *testing.T) {
	// Le premier canal echoue, le second reussit.
	routing := &mockRouting{attempts: []output.RouteAttempt{
		{Channel: domain.ChannelWhatsApp, Provider: "meta"},
		{Channel: domain.ChannelSMS, Provider: "twilio"},
	}}
	provider := &mockProvider{failUntil: 1} // premier appel echoue
	wallet := &mockWallet{hasBalance: true}
	pub := &mockPublisher{}

	uc := SendMessage{
		Repo:      &mockRepo{},
		Routing:   routing,
		Wallet:    wallet,
		Provider:  provider,
		Publisher: pub,
	}

	msg := newTestMessage()
	if err := uc.Execute(context.Background(), msg); err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	if msg.Status != domain.StatusSent {
		t.Errorf("statut attendu %s, obtenu %s", domain.StatusSent, msg.Status)
	}
	if provider.calls != 2 {
		t.Errorf("attendu 2 appels provider (fallback), obtenu %d", provider.calls)
	}
}

func TestSendMessage_AllProvidersFail_Refund(t *testing.T) {
	routing := &mockRouting{attempts: []output.RouteAttempt{
		{Channel: domain.ChannelSMS, Provider: "twilio"},
	}}
	provider := &mockProvider{failUntil: 10} // tous les appels echouent
	wallet := &mockWallet{hasBalance: true}
	pub := &mockPublisher{}

	uc := SendMessage{
		Repo:      &mockRepo{},
		Routing:   routing,
		Wallet:    wallet,
		Provider:  provider,
		Publisher: pub,
	}

	msg := newTestMessage()
	// Execute ne retourne pas d'erreur mais publie message.failed
	_ = uc.Execute(context.Background(), msg)

	if msg.Status != domain.StatusFailed {
		t.Errorf("statut attendu %s, obtenu %s", domain.StatusFailed, msg.Status)
	}
	if !wallet.refunded {
		t.Error("remboursement aurait du etre declenche")
	}
}
