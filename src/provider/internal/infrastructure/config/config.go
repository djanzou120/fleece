// Package config charge la configuration du service depuis les variables d'environnement.
// Couche 4 (Infrastructure) — aucun import de couches internes (domain/application).
package config

import "os"

// Config regroupe toutes les valeurs de configuration du service provider.
type Config struct {
	// HTTP
	Port string

	// PostgreSQL
	PostgresDSN        string
	PostgresDriver     string
	PostgresSearchPath string

	// RabbitMQ
	RabbitMQURL string

	// WhatsApp Meta
	WhatsAppMetaBaseURL     string
	WhatsAppMetaToken       string
	WhatsAppMetaPhoneNumber string

	// SMS Twilio
	TwilioBaseURL    string
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFromNumber string

	// Telegram Bot API
	// TelegramBaseURL est l'URL de base de l'API Telegram Bot.
	// Env : TELEGRAM_BASE_URL (defaut : "https://api.telegram.org").
	TelegramBaseURL string
	// TelegramBotToken est le token du bot Telegram (format "123456:ABC-DEF...").
	// Env : TELEGRAM_BOT_TOKEN. Ne jamais logger.
	// TODO(production) D27 : lire depuis provider.provider_credentials (AES-GCM)
	// quand le mecanisme de dechiffrement K8s sera implemente (T-024 DevOps).
	TelegramBotToken string
}

// Load lit les variables d'environnement et retourne une Config avec des
// valeurs par defaut raisonnables.
func Load() Config {
	return Config{
		Port:               envOr("PORT", "8084"),
		PostgresDSN:        envOr("POSTGRES_DSN", "host=localhost port=5432 user=fleece password=fleece dbname=fleece sslmode=disable"),
		PostgresDriver:     envOr("POSTGRES_DRIVER", "postgres"),
		PostgresSearchPath: envOr("POSTGRES_SEARCH_PATH", "provider"),
		RabbitMQURL:        envOr("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),

		WhatsAppMetaBaseURL:     envOr("WHATSAPP_META_BASE_URL", "https://graph.facebook.com/v18.0"),
		WhatsAppMetaToken:       envOr("WHATSAPP_META_TOKEN", ""),
		WhatsAppMetaPhoneNumber: envOr("WHATSAPP_META_PHONE_NUMBER_ID", ""),

		TwilioBaseURL:    envOr("TWILIO_BASE_URL", "https://api.twilio.com"),
		TwilioAccountSID: envOr("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:  envOr("TWILIO_AUTH_TOKEN", ""),
		TwilioFromNumber: envOr("TWILIO_FROM_NUMBER", ""),

		TelegramBaseURL:  envOr("TELEGRAM_BASE_URL", "https://api.telegram.org"),
		TelegramBotToken: envOr("TELEGRAM_BOT_TOKEN", ""),
	}
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
