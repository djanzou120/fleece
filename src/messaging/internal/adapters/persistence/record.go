// Package persistence implémente le port output.MessageRepository via *sql.DB.
// Couche 3 (Interface Adapters, driven).
package persistence

import (
	"time"

	"fleece/src/messaging/internal/domain"
)

// messageRecord est le modèle de la table messaging.messages.
// Il assure la séparation entre la représentation base et l'entité domaine.
type messageRecord struct {
	ID          string
	WorkspaceID string
	Recipient   string
	Content     string
	Status      string
	Channel     string
	CreatedAt   time.Time
}

// toEntity convertit un messageRecord en entité domaine.
func (r messageRecord) toEntity() *domain.Message {
	return &domain.Message{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		Recipient:   r.Recipient,
		Content:     r.Content,
		Status:      domain.Status(r.Status),
		Channel:     domain.Channel(r.Channel),
	}
}

// fromEntity convertit une entité domaine en messageRecord.
func fromEntity(m *domain.Message) messageRecord {
	return messageRecord{
		ID:          m.ID,
		WorkspaceID: m.WorkspaceID,
		Recipient:   m.Recipient,
		Content:     m.Content,
		Status:      string(m.Status),
		Channel:     string(m.Channel),
	}
}
