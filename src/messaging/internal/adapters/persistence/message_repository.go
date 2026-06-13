package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"fleece/src/messaging/internal/domain"
)

// MessageRepository implémente output.MessageRepository via *sql.DB.
// Il n'accède qu'au schéma "messaging" (tables préfixées messaging.).
type MessageRepository struct {
	db *sql.DB
}

// NewMessageRepository crée un MessageRepository avec la connexion fournie.
func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Save persiste un message dans messaging.messages.
// ON CONFLICT (id) DO UPDATE gère les mises à jour de statut et de canal
// sans avoir à distinguer INSERT / UPDATE côté use case.
func (r *MessageRepository) Save(ctx context.Context, m *domain.Message) error {
	rec := fromEntity(m)
	const query = `
		INSERT INTO messaging.messages
		       (id, workspace_id, recipient, content, status, channel, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE
		  SET status    = EXCLUDED.status,
		      channel   = EXCLUDED.channel
	`
	_, err := r.db.ExecContext(ctx, query,
		rec.ID,
		rec.WorkspaceID,
		rec.Recipient,
		rec.Content,
		rec.Status,
		rec.Channel,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("persistence: Save message %s: %w", m.ID, err)
	}
	return nil
}

// Get récupère un message par son identifiant.
// Retourne sql.ErrNoRows wrappé si le message est introuvable.
func (r *MessageRepository) Get(ctx context.Context, id string) (*domain.Message, error) {
	const query = `
		SELECT id, workspace_id, recipient, content, status, channel, created_at
		  FROM messaging.messages
		 WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	var rec messageRecord
	err := row.Scan(
		&rec.ID,
		&rec.WorkspaceID,
		&rec.Recipient,
		&rec.Content,
		&rec.Status,
		&rec.Channel,
		&rec.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("persistence: Get message %s: %w", id, err)
	}
	return rec.toEntity(), nil
}
