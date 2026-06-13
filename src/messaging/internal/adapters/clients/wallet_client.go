package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// WalletClient implémente output.WalletGateway via HTTP vers le Wallet Service.
type WalletClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewWalletClient crée un WalletClient ciblant l'URL de base fournie.
func NewWalletClient(baseURL string, httpClient *http.Client) *WalletClient {
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}
	return &WalletClient{baseURL: baseURL, httpClient: httpClient}
}

// balanceResponse est la réponse de l'endpoint HasBalance.
type balanceResponse struct {
	HasBalance bool `json:"has_balance"`
}

// HasBalance interroge le Wallet Service pour vérifier que le workspace a un solde suffisant.
func (c *WalletClient) HasBalance(ctx context.Context, workspaceID string) (bool, error) {
	url := fmt.Sprintf("%s/internal/wallet/%s/balance", c.baseURL, workspaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("wallet_client: HasBalance new request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("wallet_client: HasBalance do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("wallet_client: HasBalance unexpected status %d", resp.StatusCode)
	}

	var result balanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("wallet_client: HasBalance decode: %w", err)
	}
	return result.HasBalance, nil
}

// debitRequest est le corps de la requête Debit.
type debitRequest struct {
	MessageID string `json:"message_id"`
}

// Debit débite le wallet du workspace pour l'envoi du message identifié.
func (c *WalletClient) Debit(ctx context.Context, workspaceID, messageID string) error {
	body, err := json.Marshal(debitRequest{MessageID: messageID})
	if err != nil {
		return fmt.Errorf("wallet_client: Debit marshal: %w", err)
	}
	url := fmt.Sprintf("%s/internal/wallet/%s/debit", c.baseURL, workspaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("wallet_client: Debit new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wallet_client: Debit do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("wallet_client: Debit unexpected status %d", resp.StatusCode)
	}
	return nil
}

// refundRequest est le corps de la requête Refund.
type refundRequest struct {
	MessageID string `json:"message_id"`
}

// Refund rembourse le workspace pour l'envoi qui a échoué.
func (c *WalletClient) Refund(ctx context.Context, workspaceID, messageID string) error {
	body, err := json.Marshal(refundRequest{MessageID: messageID})
	if err != nil {
		return fmt.Errorf("wallet_client: Refund marshal: %w", err)
	}
	url := fmt.Sprintf("%s/internal/wallet/%s/refund", c.baseURL, workspaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("wallet_client: Refund new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wallet_client: Refund do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("wallet_client: Refund unexpected status %d", resp.StatusCode)
	}
	return nil
}
