package usecases

import (
	"context"
	"errors"

	"fleece/src/messaging/internal/application/ports/output"
	"fleece/src/messaging/internal/domain"
)

// ErrInsufficientFunds est retournée quand le wallet n'a pas un solde suffisant.
var ErrInsufficientFunds = errors.New("insufficient_funds")

// SendMessage orchestre le flux d'envoi (TDD §5.2). Il ne dépend que des ports
// (interfaces) : aucune dépendance vers un framework ou un détail technique.
type SendMessage struct {
	Repo      output.MessageRepository
	Routing   output.RoutingGateway
	Wallet    output.WalletGateway
	Provider  output.ProviderGateway
	Publisher output.EventPublisher
}

// Execute applique le pipeline : persistance → solde → routage → débit →
// envoi avec fallback → remboursement automatique en cas d'échec final.
func (uc SendMessage) Execute(ctx context.Context, m *domain.Message) error {
	if err := uc.Repo.Save(ctx, m); err != nil {
		return err
	}
	_ = uc.Publisher.Publish(ctx, "message.created", m)

	ok, err := uc.Wallet.HasBalance(ctx, m.WorkspaceID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInsufficientFunds
	}

	attempts, err := uc.Routing.Decide(ctx, m)
	if err != nil {
		return err
	}
	if len(attempts) == 0 {
		return domain.ErrNoChannel
	}

	if err := uc.Wallet.Debit(ctx, m.WorkspaceID, m.ID); err != nil {
		return err
	}
	_ = m.TransitionTo(domain.StatusQueued)
	_ = uc.Publisher.Publish(ctx, "message.queued", m)

	// Fallback : on parcourt la liste ordonnée jusqu'au premier succès.
	for _, a := range attempts {
		m.Channel = a.Channel
		if sendErr := uc.Provider.Send(ctx, m, a); sendErr == nil {
			_ = m.TransitionTo(domain.StatusSent)
			return uc.Publisher.Publish(ctx, "message.sent", m)
		}
	}

	// Toutes les tentatives ont échoué : échec + remboursement.
	_ = m.TransitionTo(domain.StatusFailed)
	_ = uc.Wallet.Refund(ctx, m.WorkspaceID, m.ID)
	return uc.Publisher.Publish(ctx, "message.failed", m)
}
