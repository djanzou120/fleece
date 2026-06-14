package providers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"fleece/src/provider/internal/domain"
)

// SMSTwilio implemente le port output.Provider pour l'API SMS Twilio.
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
//	Body: To={recipient}&From={from}&Body={content}
//
// L'externalId retourne est le "sid" de la reponse Twilio (ex. "SM...").
func (p *SMSTwilio) Send(_ context.Context, msg domain.OutboundMessage) (string, error) {
	// Validation de base.
	if msg.Recipient == "" {
		return "", fmt.Errorf("sms_twilio: recipient vide: %w", domain.ErrSendFailed)
	}
	if msg.Content == "" {
		return "", fmt.Errorf("sms_twilio: contenu vide: %w", domain.ErrSendFailed)
	}

	// TODO(production): executer la vraie requete HTTP vers l'API Twilio.
	// Stub : genere un externalId de format SID Twilio non vide.
	externalId := fmt.Sprintf("SM%d", time.Now().UnixNano())
	log.Printf("[sms_twilio:stub] Send internalId=%s recipient=%s externalId=%s",
		msg.Id, msg.Recipient, externalId)

	return externalId, nil
}

// EstimateCost retourne le cout estime d'un SMS pour le pays donne.
//
// TODO(production): utiliser l'API Twilio Pricing ou une grille tarifaire locale.
// Tarifs indicatifs Twilio (en centimes) :
//   - CM (Cameroun) : ~25 XAF par SMS
//   - SN (Senegal)  : ~20 XAF par SMS
//   - FR (France)   : ~5 EUR cents par SMS
func (p *SMSTwilio) EstimateCost(_ context.Context, channel, country string) (domain.Money, error) {
	// TODO(production): remplacer par une grille tarifaire reelle.
	_ = channel
	switch country {
	case "FR", "BE", "CH":
		return domain.Money{Amount: 5, Currency: "EUR"}, nil
	default:
		// Afrique : tarif par defaut en XAF.
		return domain.Money{Amount: 25, Currency: "XAF"}, nil
	}
}

// GetDeliveryStatus interroge Twilio pour obtenir le statut d'un SMS.
//
// TODO(production): appeler :
//
//	GET {baseURL}/2010-04-01/Accounts/{AccountSID}/Messages/{externalId}.json
//	Authorization: Basic base64(AccountSID:AuthToken)
//
// Mapper le champ "status" Twilio ("queued", "sent", "delivered", "failed", "undelivered")
// vers domain.DeliveryStatus.
func (p *SMSTwilio) GetDeliveryStatus(_ context.Context, externalId string) (domain.DeliveryStatus, error) {
	if externalId == "" {
		return "", fmt.Errorf("sms_twilio: externalId vide: %w", domain.ErrProviderUnavailable)
	}
	// TODO(production): requete GET vers l'API Twilio pour obtenir le vrai statut.
	log.Printf("[sms_twilio:stub] GetDeliveryStatus externalId=%s → sent", externalId)
	return domain.StatusSent, nil
}
