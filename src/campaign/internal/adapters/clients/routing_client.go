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

// Assertion de conformite au port output.RoutingClient (couche 2).
// IMPORTANT : ce client ne doit JAMAIS importer fleece/src/routing/... (D11, D19).
// Il parle HTTP — l'isolation inter-services passe par le reseau, pas par les imports Go.
var _ output.RoutingClient = (*RoutingClient)(nil)

// routeRequest est le DTO JSON envoye a POST /route du service routing.
// Correspond au contrat etabli en T-016 (champ recipient optionnel).
type routeRequest struct {
	WorkspaceID    string `json:"workspace_id"`
	Channel        string `json:"channel"`
	Country        string `json:"country"`
	RecipientCount int    `json:"recipient_count"`
	// Recipient est passe pour beneficier de l'enrichissement contact-intelligence (D28).
	Recipient string `json:"recipient,omitempty"`
}

// routeResponse est le DTO JSON retourne par POST /route du service routing.
type routeResponse struct {
	ProviderID    string `json:"provider_id"`
	Channel       string `json:"channel"`
	EstimatedCost int64  `json:"estimated_cost"`
	Currency      string `json:"currency"`
	Strategy      string `json:"strategy"`
}

// RoutingClient est le client REST vers le service routing.
// Couche 3 (Interface Adapters, driven) — implementee avec net/http stdlib uniquement.
//
// Timeout : 5 secondes par defaut (plus long que contact-intelligence car critique pour
// l'estimation de cout et l'envoi de campagne).
type RoutingClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewRoutingClient cree un RoutingClient avec l'URL de base du service routing.
// Exemple : "http://routing:8083".
func NewRoutingClient(baseURL string) *RoutingClient {
	return &RoutingClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetRoutingDecision appelle POST /route et retourne le provider selectionne et le cout unitaire.
// Le champ recipient est passe pour l'enrichissement contact-intelligence (T-016/D28).
func (c *RoutingClient) GetRoutingDecision(ctx context.Context, workspaceID, channel, country, recipient string) (providerID, selectedChannel string, costAmount int64, currency string, err error) {
	if c.baseURL == "" {
		return "", "", 0, "", fmt.Errorf("routing_client: baseURL non configure")
	}

	reqBody := routeRequest{
		WorkspaceID:    workspaceID,
		Channel:        channel,
		Country:        country,
		RecipientCount: 1,
		Recipient:      recipient,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("routing_client: marshal request: %w", err)
	}

	url := c.baseURL + "/route"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", 0, "", fmt.Errorf("routing_client: construire requete: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("routing_client: appel HTTP POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", 0, "", fmt.Errorf("routing_client: statut HTTP inattendu %d pour %s", resp.StatusCode, url)
	}

	var response routeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", "", 0, "", fmt.Errorf("routing_client: decoder JSON: %w", err)
	}

	return response.ProviderID, response.Channel, response.EstimatedCost, response.Currency, nil
}
