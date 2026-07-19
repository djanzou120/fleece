package api

import (
	"errors"
	"fmt"
)

// MessageStatus represente l'etat d'un message dans sa machine a etats (M-009).
type MessageStatus string

const (
	// MessageDraft : message cree, non encore soumis a l'envoi.
	MessageDraft MessageStatus = "draft"
	// MessagePending : message en file d'attente d'envoi.
	MessagePending MessageStatus = "pending"
	// MessageSent : message remis au fournisseur avec succes.
	MessageSent MessageStatus = "sent"
	// MessageDelivered : message delivre au destinataire final (DLR).
	MessageDelivered MessageStatus = "delivered"
	// MessageFailed : envoi echoue (erreur technique ou provider).
	MessageFailed MessageStatus = "failed"
	// MessageRejected : message rejete par le fournisseur (contenu invalide, etc.).
	MessageRejected MessageStatus = "rejected"
)

// ErrInvalidTransition est retournee quand une transition d'etat n'est pas
// autorisee depuis l'etat courant de l'entite.
var ErrInvalidTransition = errors.New("transition d'etat invalide")

// messageTransitions definit les transitions autorisees pour la machine a etats Message.
//
// Machine a etats :
//
//	draft     → pending, failed
//	pending   → sent, failed, rejected
//	sent      → delivered, failed
//	delivered → (terminal)
//	failed    → (terminal)
//	rejected  → (terminal)
var messageTransitions = map[MessageStatus][]MessageStatus{
	MessageDraft:     {MessagePending, MessageFailed},
	MessagePending:   {MessageSent, MessageFailed, MessageRejected},
	MessageSent:      {MessageDelivered, MessageFailed},
	MessageDelivered: {},
	MessageFailed:    {},
	MessageRejected:  {},
}

// Message est l'entite representant un message envoye via la plateforme Fleece.
type Message struct {
	ID          string
	WorkspaceID string
	Recipient   string
	Content     string
	Status      MessageStatus
	Channel     string // "sms", "whatsapp", "telegram", …
}

// NewMessage cree un message a l'etat initial draft.
func NewMessage(id, workspaceID, recipient, content, channel string) *Message {
	return &Message{
		ID:          id,
		WorkspaceID: workspaceID,
		Recipient:   recipient,
		Content:     content,
		Status:      MessageDraft,
		Channel:     channel,
	}
}

// Transition applique une transition d'etat. Retourne ErrInvalidTransition si
// la transition est interdite depuis l'etat courant.
func (m *Message) Transition(next MessageStatus) error {
	for _, allowed := range messageTransitions[m.Status] {
		if allowed == next {
			m.Status = next
			return nil
		}
	}
	return fmt.Errorf("message %s (status=%s → %s): %w", m.ID, m.Status, next, ErrInvalidTransition)
}
