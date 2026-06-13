// Package config charge la configuration du service depuis les variables d'environnement.
// Couche 4 (Infrastructure) — aucun import de couches internes (domain/application).
package config

import "os"

// Config regroupe toutes les valeurs de configuration du service messaging.
type Config struct {
	// HTTP
	Port string

	// PostgreSQL
	PostgresDSN        string
	PostgresDriver     string
	PostgresSearchPath string

	// RabbitMQ
	RabbitMQURL string

	// Services internes
	RoutingURL  string
	WalletURL   string
	ProviderURL string
}

// Load lit les variables d'environnement et retourne une Config avec des
// valeurs par défaut raisonnables.
func Load() Config {
	return Config{
		Port:               envOr("PORT", "8080"),
		PostgresDSN:        envOr("POSTGRES_DSN", "host=localhost port=5432 user=fleece password=fleece dbname=fleece sslmode=disable"),
		PostgresDriver:     envOr("POSTGRES_DRIVER", "postgres"),
		PostgresSearchPath: envOr("POSTGRES_SEARCH_PATH", "messaging"),
		RabbitMQURL:        envOr("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RoutingURL:         envOr("ROUTING_URL", "http://routing:8080"),
		WalletURL:          envOr("WALLET_URL", "http://wallet:8080"),
		ProviderURL:        envOr("PROVIDER_URL", "http://provider:8080"),
	}
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
