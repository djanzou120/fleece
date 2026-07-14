// Package config charge la configuration du service campaign depuis les variables d'environnement.
// Couche 4 (Infrastructure) — aucun import de couches internes (domain/application).
package config

import "os"

// Config regroupe toutes les valeurs de configuration du service campaign.
type Config struct {
	// HTTP : port d'ecoute.
	// Convention : messaging=8081, wallet=8082, routing=8083, provider=8084,
	//              webhook=8085, contact-intelligence=8086, campaign=8087.
	Port string

	// PostgreSQL
	PostgresDSN        string
	PostgresDriver     string
	PostgresSearchPath string

	// RabbitMQ
	RabbitMQURL string

	// QueueDLR est la queue de consommation des DLR (message.delivered/message.failed).
	QueueDLR string

	// RoutingAPIURL est l'URL de base du service routing.
	// Exemple : "http://routing:8083".
	// Si vide, le client routing n'est pas instancie.
	RoutingAPIURL string

	// MessagingAPIURL est l'URL de base du service messaging.
	// Exemple : "http://messaging:8081".
	// Si vide, le client messaging n'est pas instancie.
	MessagingAPIURL string

	// SchedulerInterval est l'intervalle du scheduler en secondes (defaut 30s).
	SchedulerInterval int
}

// Load lit les variables d'environnement et retourne une Config avec des
// valeurs par defaut raisonnables.
func Load() Config {
	return Config{
		Port:               envOr("PORT", "8087"),
		PostgresDSN:        envOr("POSTGRES_DSN", "host=localhost port=5432 user=fleece password=fleece dbname=fleece sslmode=disable"),
		PostgresDriver:     envOr("POSTGRES_DRIVER", "postgres"),
		PostgresSearchPath: envOr("POSTGRES_SEARCH_PATH", "campaign"),
		RabbitMQURL:        envOr("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		QueueDLR:           envOr("QUEUE_DLR", "campaign.dlr"),
		RoutingAPIURL:      envOr("ROUTING_API_URL", ""),
		MessagingAPIURL:    envOr("MESSAGING_API_URL", ""),
		SchedulerInterval:  envOrInt("SCHEDULER_INTERVAL", 30),
	}
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envOrInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	var i int
	if _, err := parseSimpleInt(v, &i); err != nil {
		return defaultVal
	}
	return i
}

// parseSimpleInt parse un entier depuis une string (evite l'import de strconv dans les tests).
func parseSimpleInt(s string, out *int) (int, error) {
	n := 0
	neg := false
	for i, c := range s {
		if i == 0 && c == '-' {
			neg = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, errInvalidInt
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		n = -n
	}
	*out = n
	return n, nil
}

// errInvalidInt est l'erreur retournee pour un entier invalide.
type invalidIntError struct{}

func (e invalidIntError) Error() string { return "config: entier invalide" }

var errInvalidInt = invalidIntError{}
