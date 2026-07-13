package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"fleece/src/routing/internal/application/ports/output"
)

// Assertion de conformite au port output.ContactScoreClient (couche 2).
// Garantit que ContactIntelClient implemente bien l'interface au moment de la compilation.
// IMPORTANT : ce client ne doit JAMAIS importer fleece/src/contact-intelligence/... (D11, D19).
// Il parle HTTP — l'isolation inter-services passe par le reseau, pas par les imports Go.
var _ output.ContactScoreClient = (*ContactIntelClient)(nil)

// contactScoreResponse est le DTO JSON retourne par GET /contacts/{phone}/score.
// Contrat du service contact-intelligence (T-012) :
//
//	{ "phone": string, "delivery_score": int, "found": bool,
//	  "channel_scores": [ { "channel": string, "score": int,
//	                        "sample_size": int, "last_success_at": string|null } ] }
//
// Jamais de 404 : contact inconnu → found=false, delivery_score=50 (prior Laplace).
type contactScoreResponse struct {
	Phone         string               `json:"phone"`
	DeliveryScore int                  `json:"delivery_score"`
	Found         bool                 `json:"found"`
	ChannelScores []channelScoreRecord `json:"channel_scores"`
}

type channelScoreRecord struct {
	Channel       string `json:"channel"`
	Score         int    `json:"score"`
	SampleSize    int    `json:"sample_size"`
	LastSuccessAt any    `json:"last_success_at"` // string|null
}

// ContactIntelClient est le client REST vers le service contact-intelligence.
// Couche 3 (Interface Adapters, driven) — implementee avec net/http stdlib uniquement.
//
// Timeout court (2 s par defaut) pour garantir le fallback gracieux sans degrader
// la latence de routage en cas d'indisponibilite du service.
//
// Si l'URL de base est vide, chaque appel retourne une erreur (le use case
// degrade alors silencieusement — ne jamais injecter ce client avec une URL vide,
// preferer injecter nil directement dans ce cas).
type ContactIntelClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewContactIntelClient cree un ContactIntelClient avec l'URL de base du service
// contact-intelligence (ex. "http://contact-intelligence:8086") et un timeout
// HTTP par defaut de 2 secondes (degrade gracieux si le service est lent).
func NewContactIntelClient(baseURL string) *ContactIntelClient {
	return &ContactIntelClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// GetChannelScore retourne le score de joignabilite du destinataire phone sur channel.
//
// Logique de selection du score :
//  1. Si channel_scores contient une entree pour le canal demande → retourne ce score.
//  2. Si le canal n'est pas dans channel_scores (canal non observe) → retourne delivery_score
//     (score global de delivraison, repli documente). Exemple : contact connu sur SMS
//     mais jamais contacte sur WhatsApp → delivery_score (50 si inconnu).
//
// Fallback gracieux (documente dans le port output.ContactScoreClient) :
//   - HTTP timeout → err non nil → le use case degrade vers provider_scores seuls.
//   - Status HTTP != 200 → err non nil → meme degrade.
//   - JSON invalide → err non nil → meme degrade.
//   - contact inconnu (found=false) → score=50, found=false, err=nil (jamais 404).
func (c *ContactIntelClient) GetChannelScore(ctx context.Context, phone, channel string) (score int, found bool, err error) {
	if c.baseURL == "" {
		return 0, false, fmt.Errorf("contact_intel_client: baseURL non configure")
	}

	url := fmt.Sprintf("%s/contacts/%s/score", c.baseURL, phone)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false, fmt.Errorf("contact_intel_client: construire requete: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, false, fmt.Errorf("contact_intel_client: appel HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, false, fmt.Errorf("contact_intel_client: statut HTTP inattendu %d pour %s", resp.StatusCode, url)
	}

	var body contactScoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, false, fmt.Errorf("contact_intel_client: decoder JSON: %w", err)
	}

	// Chercher le score specifique au canal demande dans channel_scores.
	// channel_scores est trie par score DESC (contrat T-012) mais on cherche par canal.
	for _, cs := range body.ChannelScores {
		if cs.Channel == channel {
			return cs.Score, body.Found, nil
		}
	}

	// Canal non trouve dans channel_scores : repli sur delivery_score global.
	// Cas courant : contact connu sur un autre canal mais jamais contacte sur celui-ci.
	// delivery_score = 50 si contact inconnu (prior Laplace, found=false).
	return body.DeliveryScore, body.Found, nil
}
