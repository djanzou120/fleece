package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"fleece/src/messaging/internal/application/ports/output"
	"fleece/src/messaging/internal/domain"
)

// ProviderClient implémente output.ProviderGateway via HTTP vers le Provider Service.
type ProviderClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewProviderClient crée un ProviderClient ciblant l'URL de base fournie.
func NewProviderClient(baseURL string, httpClient *http.Client) *ProviderClient {
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}
	return &ProviderClient{baseURL: baseURL, httpClient: httpClient}
}

// sendRequest est le corps de la requête POST vers le Provider Service.
type sendRequest struct {
	MessageID   string `json:"message_id"`
	WorkspaceID string `json:"workspace_id"`
	Recipient   string `json:"recipient"`
	Content     string `json:"content"`
	Channel     string `json:"channel"`
	Provider    string `json:"provider"`
}

// Send délègue l'envoi effectif au Provider Service.
func (c *ProviderClient) Send(ctx context.Context, m *domain.Message, attempt output.RouteAttempt) error {
	reqBody := sendRequest{
		MessageID:   m.ID,
		WorkspaceID: m.WorkspaceID,
		Recipient:   m.Recipient,
		Content:     m.Content,
		Channel:     string(attempt.Channel),
		Provider:    attempt.Provider,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("provider_client: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/provider/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("provider_client: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("provider_client: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("provider_client: unexpected status %d for provider=%s channel=%s",
			resp.StatusCode, attempt.Provider, attempt.Channel)
	}
	return nil
}

// defaultHTTPClient retourne un *http.Client avec des timeouts raisonnables.
// Partagé par tous les clients du package.
func defaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}
