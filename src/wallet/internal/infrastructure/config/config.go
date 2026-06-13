// Package config charge la configuration du service depuis les variables d'environnement.
// Couche 4 (Infrastructure) — aucun import de couches internes (domain/application).
package config

import "os"

// Config regroupe toutes les valeurs de configuration du service wallet.
type Config struct {
	// HTTP
	Port string

	// PostgreSQL
	PostgresDSN        string
	PostgresDriver     string
	PostgresSearchPath string

	// RabbitMQ
	RabbitMQURL string
}

// Load lit les variables d'environnement et retourne une Config avec des
// valeurs par defaut raisonnables.
func Load() Config {
	return Config{
		Port:               envOr("PORT", "8082"),
		PostgresDSN:        envOr("POSTGRES_DSN", "host=localhost port=5432 user=fleece password=fleece dbname=fleece sslmode=disable"),
		PostgresDriver:     envOr("POSTGRES_DRIVER", "postgres"),
		PostgresSearchPath: envOr("POSTGRES_SEARCH_PATH", "wallet"),
		RabbitMQURL:        envOr("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
	}
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
