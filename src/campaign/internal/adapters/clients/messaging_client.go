package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"fleece/src/campaign/internal/application/ports/output"
)

// Assertion de conformite au port output.MessagingClient (couche 2).
// IMPORTANT : ce client ne doit JAMAIS importer fleece/src/messaging/... (D11, D19).
// Il parle HTTP — l'isolation inter-services passe par le reseau, pas par les imports Go.
var _ output.MessagingClient = (*MessagingClient)(nil)

// sendMessageRequest est le DTO JSON envoye au service messaging.
// Adapte au contrat du Messaging Service (MVP).
type sendMessageRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Recipient   string `json:"recipient"`
	Body        string `json:"body"`
	Channel     string `json:"channel"`
}

// sendMessageResponse est le DTO JSON retourne par le service messaging.
type sendMessageResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

// MessagingClient est le client REST vers le service messaging.
// Couche 3 (Interface Adapters, driven) — implementee avec net/http stdlib uniquement.
//
// Timeout : 10 secondes pour tolerer les latences du provider.
type MessagingClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMessagingClient cree un MessagingClient avec l'URL de base du service messaging.
// Exemple : "http://messaging:8081".
func NewMessagingClient(baseURL string) *MessagingClient {
	return &MessagingClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendMessage envoie un message individuel et retourne l'ID du message cree.
// workspaceID, recipient, body, channel sont les parametres du message.
func (c *MessagingClient) SendMessage(ctx context.Context, workspaceID, recipient, body, channel string) (string, error) {
	if c.baseURL == "" {
		return "", fmt.Errorf("messaging_client: baseURL non configure")
	}

	reqBody := sendMessageRequest{
		WorkspaceID: workspaceID,
		Recipient:   recipient,
		Body:        body,
		Channel:     channel,
	}

	encoded, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("messaging_client: marshal request: %w", err)
	}

	url := c.baseURL + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return "", fmt.Errorf("messaging_client: construire requete: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("messaging_client: appel HTTP POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("messaging_client: statut HTTP inattendu %d pour %s", resp.StatusCode, url)
	}

	var response sendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("messaging_client: decoder JSON: %w", err)
	}

	if response.MessageID == "" {
		return "", fmt.Errorf("messaging_client: message_id vide dans la reponse")
	}

	return response.MessageID, nil
}
