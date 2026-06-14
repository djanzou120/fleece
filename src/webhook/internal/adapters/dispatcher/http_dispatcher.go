// Package dispatcher implemente le port output.HTTPDispatcher via net/http.
// Couche 3 (Interface Adapters, driven).
package dispatcher

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"fleece/src/webhook/internal/application/ports/output"
)

// Assertion de conformite a la compilation : HTTPDispatcher doit implementer output.HTTPDispatcher.
// Couche 3 → couche 2 : sens autorise (vers l'interieur).
var _ output.HTTPDispatcher = (*HTTPDispatcher)(nil)

// HTTPDispatcher implemente output.HTTPDispatcher en effectuant un HTTP POST
// sur l'URL cible avec le header de signature Fleece.
type HTTPDispatcher struct {
	client *http.Client
}

// New cree un HTTPDispatcher avec un client HTTP a timeout de 10 secondes.
func New(client *http.Client) *HTTPDispatcher {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &HTTPDispatcher{client: client}
}

// Dispatch effectue un HTTP POST sur url avec le payload et la signature fournis.
//
// Headers positionnes :
//   - Content-Type: application/json
//   - X-Fleece-Signature: <signature>
//
// Retourne le code HTTP de la reponse (ou 0 en cas d'erreur reseau) et l'erreur eventuelle.
func (d *HTTPDispatcher) Dispatch(ctx context.Context, url string, payload []byte, signature string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("dispatcher: creer requete vers %s: %w", url, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Fleece-Signature", signature)

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("dispatcher: POST vers %s: %w", url, err)
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}
