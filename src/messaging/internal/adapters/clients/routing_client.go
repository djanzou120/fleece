package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"fleece/src/messaging/internal/application/ports/output"
	"fleece/src/messaging/internal/domain"
)

// RoutingClient implémente output.RoutingGateway via HTTP (POST JSON vers le Routing Service).
type RoutingClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewRoutingClient crée un RoutingClient ciblant l'URL de base fournie.
func NewRoutingClient(baseURL string, httpClient *http.Client) *RoutingClient {
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}
	return &RoutingClient{baseURL: baseURL, httpClient: httpClient}
}

// routingRequest est le corps de la requête POST vers le Routing Service.
type routingRequest struct {
	MessageID   string `json:"message_id"`
	WorkspaceID string `json:"workspace_id"`
	Recipient   string `json:"recipient"`
}

// routingAttempt est un élément de la réponse du Routing Service.
type routingAttempt struct {
	Channel  string `json:"channel"`
	Provider string `json:"provider"`
}

// Decide envoie une requête au Routing Service et retourne la liste ordonnée
// des RouteAttempt (canal + fournisseur) à essayer dans l'ordre de priorité.
func (c *RoutingClient) Decide(ctx context.Context, m *domain.Message) ([]output.RouteAttempt, error) {
	reqBody := routingRequest{
		MessageID:   m.ID,
		WorkspaceID: m.WorkspaceID,
		Recipient:   m.Recipient,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("routing_client: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/routing/decide", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("routing_client: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("routing_client: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("routing_client: unexpected status %d", resp.StatusCode)
	}

	var attempts []routingAttempt
	if err := json.NewDecoder(resp.Body).Decode(&attempts); err != nil {
		return nil, fmt.Errorf("routing_client: decode response: %w", err)
	}

	result := make([]output.RouteAttempt, 0, len(attempts))
	for _, a := range attempts {
		result = append(result, output.RouteAttempt{
			Channel:  domain.Channel(a.Channel),
			Provider: a.Provider,
		})
	}
	return result, nil
}
