package domain

import "errors"

// Status représente l'état d'un message dans sa machine à états (TDD §6.1).
type Status string

const (
	StatusCreated   Status = "created"
	StatusQueued    Status = "queued"
	StatusSent      Status = "sent"
	StatusDelivered Status = "delivered"
	StatusFailed    Status = "failed"
)

// Channel est un canal de messagerie supporté.
type Channel string

const (
	ChannelSMS      Channel = "sms"
	ChannelWhatsApp Channel = "whatsapp"
	ChannelTelegram Channel = "telegram"
)

// Erreurs métier du domaine.
var (
	ErrInvalidTransition = errors.New("transition d'état invalide")
	ErrNoChannel         = errors.New("aucun canal valide")
)

// transitions autorisées de la machine à états.
var transitions = map[Status][]Status{
	StatusCreated:   {StatusQueued, StatusFailed},
	StatusQueued:    {StatusSent, StatusFailed},
	StatusSent:      {StatusDelivered, StatusFailed},
	StatusDelivered: {},
	StatusFailed:    {},
}

// Message est l'entité racine du domaine Messaging.
type Message struct {
	ID          string
	WorkspaceID string
	Recipient   string
	Content     string
	Status      Status
	Channel     Channel
}

// NewMessage crée un message à l'état initial `created`.
func NewMessage(id, workspaceID, recipient, content string) *Message {
	return &Message{
		ID:          id,
		WorkspaceID: workspaceID,
		Recipient:   recipient,
		Content:     content,
		Status:      StatusCreated,
	}
}

// TransitionTo applique une transition en respectant les invariants de la
// machine à états. Retourne ErrInvalidTransition si la transition est interdite.
func (m *Message) TransitionTo(next Status) error {
	for _, allowed := range transitions[m.Status] {
		if allowed == next {
			m.Status = next
			return nil
		}
	}
	return ErrInvalidTransition
}
