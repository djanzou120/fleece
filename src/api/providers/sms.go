// Package providers regroupe les implementations concretes de l'interface api.Provider.
// Ces adapters migrent les logiques stub de src/provider/internal/adapters/providers/
// vers le service unifie src/api, sans aucune dependance vers l'ancien service.
package providers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"fleece/src/api"
	golog "fleece/src/go/log"
)

// Assertion de conformite au port : ne compile pas si SMSTwilio n'implemente pas api.Provider.
var _ api.Provider = (*SMSTwilio)(nil)

// SMSTwilio implemente api.Provider pour l'API SMS Twilio.
//
// TODO(production): remplacer les stubs par de vrais appels HTTP a l'API Twilio :
//   - POST https://api.twilio.com/2010-04-01/Accounts/{AccountSID}/Messages.json
//   - Authentification : HTTP Basic (AccountSID:AuthToken)
//   - Parsing de la reponse pour extraire le SID Twilio (externalId)
type SMSTwilio struct {
	// client est le client HTTP utilise pour les appels vers l'API Twilio.
	client *http.Client

	// baseURL est l'URL de base de l'API Twilio.
	// TODO(production): lire depuis la config (env TWILIO_BASE_URL).
	baseURL string

	// accountSID est l'identifiant de compte Twilio.
	// TODO(production): lire depuis la config (env TWILIO_ACCOUNT_SID).
	accountSID string

	// authToken est le token d'authentification Twilio.
	// TODO(production): lire depuis la config (env TWILIO_AUTH_TOKEN) et ne jamais logger.
	authToken string

	// from est le numero emetteur Twilio (+E.164).
	// TODO(production): lire depuis la config (env TWILIO_FROM_NUMBER).
	from string

	// Logger est le logger structure partage (D-M17, Phase 3), optionnel :
	// nil ⇒ aucun log emis (garde nil, meme pattern que s.AMQP == nil
	// ailleurs dans le depot). Cable par providers.BuildRegistry depuis le
	// composition root (src/api/cmd/api/main.go). Jamais log.Printf stdlib.
	Logger *golog.Logger
}

// NewSMSTwilio cree un adapter SMS Twilio avec les parametres fournis.
// En production, baseURL = "https://api.twilio.com".
func NewSMSTwilio(baseURL, accountSID, authToken, from string) *SMSTwilio {
	return &SMSTwilio{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL:    baseURL,
		accountSID: accountSID,
		authToken:  authToken,
		from:       from,
	}
}

// Send envoie un SMS via l'API Twilio.
//
// TODO(production): construire et executer la vraie requete HTTP :
//
//	POST {baseURL}/2010-04-01/Accounts/{AccountSID}/Messages.json
//	Authorization: Basic base64(AccountSID:AuthToken)
//	Content-Type: application/x-www-form-urlencoded
//	Body: To={to}&From={from}&Body={body}
//
// L'ExternalID retourne est le "sid" de la reponse Twilio (ex. "SM...").
func (p *SMSTwilio) Send(_ context.Context, to, body string) (api.ProviderResult, error) {
	// Validation de base.
	if to == "" {
		return api.ProviderResult{}, fmt.Errorf("sms_twilio: recipient vide: %w", errSendFailed)
	}
	if body == "" {
		return api.ProviderResult{}, fmt.Errorf("sms_twilio: contenu vide: %w", errSendFailed)
	}

	// TODO(production): executer la vraie requete HTTP vers l'API Twilio.
	// Stub : genere un externalId de format SID Twilio non vide.
	externalID := fmt.Sprintf("SM%d", time.Now().UnixNano())
	logInfo(p.Logger, "sms_twilio: stub send", "to", to, "external_id", externalID)

	return api.ProviderResult{
		ExternalID: externalID,
		Status:     "sent",
		Cost:       p.EstimateCost(to, body),
	}, nil
}

// EstimateCost retourne le cout estime d'un SMS en centimes.
//
// TODO(production): utiliser l'API Twilio Pricing ou une grille tarifaire locale.
// Tarifs indicatifs Twilio (en centimes) :
//   - CM (Cameroun) : ~25 XAF par SMS
//   - SN (Senegal)  : ~20 XAF par SMS
//   - FR (France)   : ~5 EUR cents par SMS
//
// Le parametre to est le numero E.164 du destinataire (indicatif pays utile en production).
// body est le contenu du message (longueur utile pour facturer les SMS multiparts).
func (p *SMSTwilio) EstimateCost(_, _ string) int64 {
	// TODO(production): remplacer par une grille tarifaire reelle.
	// Stub : cout fixe de 25 centimes (XAF) par message.
	return 25
}

// GetDeliveryStatus interroge Twilio pour obtenir le statut d'un SMS.
//
// TODO(production): appeler :
//
//	GET {baseURL}/2010-04-01/Accounts/{AccountSID}/Messages/{externalID}.json
//	Authorization: Basic base64(AccountSID:AuthToken)
//
// Mapper le champ "status" Twilio ("queued", "sent", "delivered", "failed", "undelivered")
// vers les valeurs normalisees "sent", "delivered", "failed", "rejected".
func (p *SMSTwilio) GetDeliveryStatus(_ context.Context, externalID string) (string, error) {
	if externalID == "" {
		return "", fmt.Errorf("sms_twilio: externalId vide: %w", errProviderUnavailable)
	}
	// TODO(production): requete GET vers l'API Twilio pour obtenir le vrai statut.
	logInfo(p.Logger, "sms_twilio: stub get_delivery_status", "external_id", externalID, "status", "sent")
	return "sent", nil
}
