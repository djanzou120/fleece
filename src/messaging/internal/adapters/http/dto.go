// Package http expose le use case SendMessage via une API REST (net/http).
// Couche 3 (Interface Adapters, driving).
package http

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"fleece/src/messaging/internal/domain"
)

// SendMessageRequest est le DTO d'entrée de l'endpoint POST /messages.
type SendMessageRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Recipient   string `json:"recipient"`
	Content     string `json:"content"`
}

// SendMessageResponse est le DTO de sortie de l'endpoint POST /messages.
type SendMessageResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// validate vérifie que les champs obligatoires sont présents.
func (r *SendMessageRequest) validate() error {
	if r.WorkspaceID == "" {
		return fmt.Errorf("workspace_id est requis")
	}
	if r.Recipient == "" {
		return fmt.Errorf("recipient est requis")
	}
	if r.Content == "" {
		return fmt.Errorf("content est requis")
	}
	return nil
}

// toMessage construit une entité domaine à partir du DTO.
// L'ID UUID v4 est généré ici (crypto/rand, pas de lib externe).
func (r *SendMessageRequest) toMessage() (*domain.Message, error) {
	id, err := newUUID()
	if err != nil {
		return nil, fmt.Errorf("dto: generate uuid: %w", err)
	}
	return domain.NewMessage(id, r.WorkspaceID, r.Recipient, r.Content), nil
}

// newUUID génère un UUID v4 (RFC 4122) en utilisant uniquement crypto/rand.
func newUUID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	// Version 4 : bits 12-15 du 7e octet = 0100
	buf[6] = (buf[6] & 0x0f) | 0x40
	// Variante RFC 4122 : bits 6-7 du 9e octet = 10
	buf[8] = (buf[8] & 0x3f) | 0x80

	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(buf[0:4]),
		hex.EncodeToString(buf[4:6]),
		hex.EncodeToString(buf[6:8]),
		hex.EncodeToString(buf[8:10]),
		hex.EncodeToString(buf[10:16]),
	), nil
}
