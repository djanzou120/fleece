package providers

import (
	"fleece/src/api"
	golog "fleece/src/go/log"
)

// ProviderConfig regroupe les credentials et URLs des trois providers.
// Les valeurs sont lues depuis les variables d'environnement dans main.go
// (ou via os.Getenv directement) avant d'appeler BuildRegistry.
type ProviderConfig struct {
	// SMS Twilio
	// Env : TWILIO_BASE_URL, TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, TWILIO_FROM_NUMBER.
	TwilioBaseURL    string
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFromNumber string

	// WhatsApp Meta Cloud
	// Env : WHATSAPP_META_BASE_URL, WHATSAPP_META_TOKEN, WHATSAPP_META_PHONE_NUMBER_ID.
	WhatsAppMetaBaseURL     string
	WhatsAppMetaToken       string
	WhatsAppMetaPhoneNumber string

	// Telegram Bot API
	// Env : TELEGRAM_BASE_URL, TELEGRAM_BOT_TOKEN.
	// Ne jamais logger TelegramBotToken.
	// TODO(production) D27: lire depuis provider.provider_credentials (AES-GCM, T-024 DevOps).
	TelegramBaseURL  string
	TelegramBotToken string

	// Logger est le logger structure partage (D-M17, Phase 3), optionnel :
	// nil ⇒ les trois adapters ne loggent rien (garde nil, voir logging.go).
	// Cable par le composition root (src/api/cmd/api/main.go) — remplace les
	// anciens log.Printf stdlib de sms.go/telegram.go/whatsapp.go.
	Logger *golog.Logger
}

// BuildRegistry instancie les trois adapters providers et les enregistre sous
// les cles canoniques du registry (correspondant aux valeurs de provider.providers.id
// en base, seedes dans les migrations 0005 et 0015).
//
// Cles retournees :
//   - "sms-twilio"    → SMSTwilio
//   - "whatsapp-meta" → WhatsAppMeta
//   - "telegram-bot"  → TelegramBot
func BuildRegistry(cfg ProviderConfig) map[string]api.Provider {
	sms := NewSMSTwilio(cfg.TwilioBaseURL, cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFromNumber)
	sms.Logger = cfg.Logger

	whatsapp := NewWhatsAppMeta(cfg.WhatsAppMetaBaseURL, cfg.WhatsAppMetaToken, cfg.WhatsAppMetaPhoneNumber)
	whatsapp.Logger = cfg.Logger

	telegram := NewTelegramBot(cfg.TelegramBaseURL, cfg.TelegramBotToken)
	telegram.Logger = cfg.Logger

	return map[string]api.Provider{
		"sms-twilio":    sms,
		"whatsapp-meta": whatsapp,
		"telegram-bot":  telegram,
	}
}
