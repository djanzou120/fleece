package usecases

import (
	"context"
	"errors"

	"fleece/src/contact-intelligence/internal/domain"
)

// ─── Mock ContactRepository ───────────────────────────────────────────────────

type mockContactRepo struct {
	contacts  map[string]*domain.Contact
	getErr    error
	upsertErr error
	upserted  []*domain.Contact
}

func newMockContactRepo() *mockContactRepo {
	return &mockContactRepo{contacts: make(map[string]*domain.Contact)}
}

func (m *mockContactRepo) Get(_ context.Context, phone string) (*domain.Contact, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	c, ok := m.contacts[phone]
	if !ok {
		return nil, domain.ErrContactNotFound
	}
	// copie defensive
	cp := *c
	return &cp, nil
}

func (m *mockContactRepo) Upsert(_ context.Context, c *domain.Contact) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	cp := *c
	m.contacts[c.Phone] = &cp
	m.upserted = append(m.upserted, &cp)
	return nil
}

// ─── Mock ChannelScoreRepository ─────────────────────────────────────────────

type mockChannelScoreRepo struct {
	scores    map[string]*domain.ChannelScore
	listErr   error
	upsertErr error
	upserted  []*domain.ChannelScore
}

func newMockChannelScoreRepo() *mockChannelScoreRepo {
	return &mockChannelScoreRepo{scores: make(map[string]*domain.ChannelScore)}
}

func (m *mockChannelScoreRepo) key(phone string, ch domain.Channel) string {
	return phone + ":" + string(ch)
}

func (m *mockChannelScoreRepo) Get(_ context.Context, phone string, ch domain.Channel) (*domain.ChannelScore, error) {
	cs, ok := m.scores[m.key(phone, ch)]
	if !ok {
		return nil, nil // neutre
	}
	cp := *cs
	return &cp, nil
}

func (m *mockChannelScoreRepo) ListByPhone(_ context.Context, phone string) ([]*domain.ChannelScore, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*domain.ChannelScore
	for _, cs := range m.scores {
		if cs.Phone == phone {
			cp := *cs
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *mockChannelScoreRepo) Upsert(_ context.Context, cs *domain.ChannelScore) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	cp := *cs
	m.scores[m.key(cs.Phone, cs.Channel)] = &cp
	m.upserted = append(m.upserted, &cp)
	return nil
}

// ─── Mock OutcomeStore ────────────────────────────────────────────────────────

type mockOutcomeStore struct {
	recorded []*recordedAtomically
	err      error
}

type recordedAtomically struct {
	outcome      *domain.DeliveryOutcome
	channelScore *domain.ChannelScore
	contact      *domain.Contact
}

func (m *mockOutcomeStore) RecordAtomically(_ context.Context, o *domain.DeliveryOutcome, cs *domain.ChannelScore, c *domain.Contact) error {
	if m.err != nil {
		return m.err
	}
	m.recorded = append(m.recorded, &recordedAtomically{
		outcome:      o,
		channelScore: cs,
		contact:      c,
	})
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

var errInternal = errors.New("erreur interne simulee")
