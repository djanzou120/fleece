// Package config charge la configuration du service contact-intelligence
// depuis les variables d'environnement.
// Couche 4 (Infrastructure) — aucun import de couches internes.
package config

import "os"

// Config regroupe toutes les valeurs de configuration du service contact-intelligence.
type Config struct {
	// HTTP : port d'ecoute.
	// Convention : messaging=8081, wallet=8082, routing=8083, provider=8084,
	//              webhook=8085, contact-intelligence=8086.
	Port string

	// PostgreSQL
	PostgresDSN        string
	PostgresDriver     string
	PostgresSearchPath string

	// RabbitMQ
	RabbitMQURL string

	// QueueDLR est la queue de consommation des DLR (message.delivered/message.failed).
	QueueDLR string
}

// Load lit les variables d'environnement et retourne une Config avec des
// valeurs par defaut raisonnables.
func Load() Config {
	return Config{
		Port:               envOr("PORT", "8086"),
		PostgresDSN:        envOr("POSTGRES_DSN", "host=localhost port=5432 user=fleece password=fleece dbname=fleece sslmode=disable"),
		PostgresDriver:     envOr("POSTGRES_DRIVER", "postgres"),
		PostgresSearchPath: envOr("POSTGRES_SEARCH_PATH", "contact_intel"),
		RabbitMQURL:        envOr("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		QueueDLR:           envOr("QUEUE_DLR", "contact_intel.dlr"),
	}
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
