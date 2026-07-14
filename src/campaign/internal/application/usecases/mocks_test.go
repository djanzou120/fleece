package usecases_test

import (
	"context"
	"errors"

	"fleece/src/campaign/internal/domain"
)

// ─── Mock CampaignRepository ──────────────────────────────────────────────────

type mockCampaignRepo struct {
	campaigns           map[string]*domain.Campaign
	saveErr             error
	getErr              error
	scheduledDue        []*domain.Campaign
	atomicRows          int64
	atomicErr           error
	atomicFromStatus    domain.CampaignStatus
	atomicToStatus      domain.CampaignStatus
	savedCampaigns      []*domain.Campaign
}

func newMockCampaignRepo() *mockCampaignRepo {
	return &mockCampaignRepo{
		campaigns:  make(map[string]*domain.Campaign),
		atomicRows: 1, // defaut : 1 ligne affectee
	}
}

func (m *mockCampaignRepo) Save(_ context.Context, c *domain.Campaign) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	cp := *c
	m.campaigns[c.ID] = &cp
	m.savedCampaigns = append(m.savedCampaigns, &cp)
	return nil
}

func (m *mockCampaignRepo) Get(_ context.Context, id string) (*domain.Campaign, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	c, ok := m.campaigns[id]
	if !ok {
		return nil, domain.ErrCampaignNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *mockCampaignRepo) ListScheduledDue(_ context.Context) ([]*domain.Campaign, error) {
	return m.scheduledDue, nil
}

func (m *mockCampaignRepo) UpdateStatusAtomic(_ context.Context, id string, from, to domain.CampaignStatus) (int64, error) {
	m.atomicFromStatus = from
	m.atomicToStatus = to
	if m.atomicErr != nil {
		return 0, m.atomicErr
	}
	c, ok := m.campaigns[id]
	if !ok {
		return 0, nil
	}
	if c.Status != from {
		return 0, nil // anti-double-run : campagne deja changee de statut
	}
	c.Status = to
	m.campaigns[id] = c
	return m.atomicRows, nil
}

// ─── Mock RecipientRepository ─────────────────────────────────────────────────

type mockRecipientRepo struct {
	insertedCount    int64
	insertErr        error
	pending          []*domain.CampaignRecipient
	listPendingErr   error
	updateStatusRows int64
	updateStatusErr  error
	markedSent       map[int64]string // id → messageID
	markedFailed     []int64
	markSentErr      error
	markFailedErr    error
}

func newMockRecipientRepo() *mockRecipientRepo {
	return &mockRecipientRepo{
		markedSent:       make(map[int64]string),
		updateStatusRows: 1,
	}
}

func (m *mockRecipientRepo) BulkInsertDedup(_ context.Context, _ string, recipients []string) (int64, error) {
	if m.insertErr != nil {
		return 0, m.insertErr
	}
	if m.insertedCount > 0 {
		return m.insertedCount, nil
	}
	return int64(len(recipients)), nil
}

func (m *mockRecipientRepo) ListPendingByCampaign(_ context.Context, _ string) ([]*domain.CampaignRecipient, error) {
	if m.listPendingErr != nil {
		return nil, m.listPendingErr
	}
	return m.pending, nil
}

func (m *mockRecipientRepo) UpdateStatusByMessageID(_ context.Context, messageID string, _ domain.RecipientStatus) (int64, error) {
	if m.updateStatusErr != nil {
		return 0, m.updateStatusErr
	}
	if messageID == "" {
		return 0, nil
	}
	return m.updateStatusRows, nil
}

func (m *mockRecipientRepo) MarkSent(_ context.Context, id int64, messageID string) error {
	if m.markSentErr != nil {
		return m.markSentErr
	}
	m.markedSent[id] = messageID
	return nil
}

func (m *mockRecipientRepo) MarkFailed(_ context.Context, id int64) error {
	if m.markFailedErr != nil {
		return m.markFailedErr
	}
	m.markedFailed = append(m.markedFailed, id)
	return nil
}

// ─── Mock RunRepository ────────────────────────────────────────────────────────

type mockRunRepo struct {
	createID      int64
	createErr     error
	sentInc       int
	deliveredInc  int
	failedInc     int
	finishedRuns  []int64
}

func newMockRunRepo() *mockRunRepo {
	return &mockRunRepo{createID: 42}
}

func (m *mockRunRepo) Create(_ context.Context, _ *domain.CampaignRun) (int64, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	return m.createID, nil
}

func (m *mockRunRepo) IncrementSent(_ context.Context, _ int64) error {
	m.sentInc++
	return nil
}

func (m *mockRunRepo) IncrementDelivered(_ context.Context, _ int64) error {
	m.deliveredInc++
	return nil
}

func (m *mockRunRepo) IncrementFailed(_ context.Context, _ int64) error {
	m.failedInc++
	return nil
}

func (m *mockRunRepo) Finish(_ context.Context, runID int64) error {
	m.finishedRuns = append(m.finishedRuns, runID)
	return nil
}

// ─── Mock MessagingClient ─────────────────────────────────────────────────────

type mockMessagingClient struct {
	messageID string
	sendErr   error
	calls     []messagingCall
}

type messagingCall struct {
	workspaceID string
	recipient   string
	body        string
	channel     string
}

func newMockMessagingClient(messageID string) *mockMessagingClient {
	return &mockMessagingClient{messageID: messageID}
}

func (m *mockMessagingClient) SendMessage(_ context.Context, workspaceID, recipient, body, channel string) (string, error) {
	m.calls = append(m.calls, messagingCall{workspaceID, recipient, body, channel})
	if m.sendErr != nil {
		return "", m.sendErr
	}
	return m.messageID, nil
}

// ─── Mock RoutingClient ────────────────────────────────────────────────────────

type mockRoutingClient struct {
	providerID string
	channel    string
	cost       int64
	currency   string
	routingErr error
	calls      int
}

func (m *mockRoutingClient) GetRoutingDecision(_ context.Context, _, _, _, _ string) (string, string, int64, string, error) {
	m.calls++
	if m.routingErr != nil {
		return "", "", 0, "", m.routingErr
	}
	return m.providerID, m.channel, m.cost, m.currency, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

var errInternal = errors.New("erreur interne simulee")
