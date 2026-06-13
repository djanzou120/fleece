// Package config charge la configuration du service depuis les variables d'environnement.
// Couche 4 (Infrastructure) — aucun import de couches internes (domain/application).
package config

import "os"

// Config regroupe toutes les valeurs de configuration du service routing.
type Config struct {
	// HTTP
	Port string

	// PostgreSQL
	PostgresDSN        string
	PostgresDriver     string
	PostgresSearchPath string

	// DefaultCurrency est la devise utilisee par defaut pour les couts de tarification.
	// La table routing.provider_pricing ne stocke pas la devise ; l'adapter de persistence
	// utilise cette valeur pour construire le Money des entites domaine.
	DefaultCurrency string
}

// Load lit les variables d'environnement et retourne une Config avec des
// valeurs par defaut raisonnables pour le service routing.
func Load() Config {
	return Config{
		Port:               envOr("PORT", "8083"),
		PostgresDSN:        envOr("POSTGRES_DSN", "host=localhost port=5432 user=fleece password=fleece dbname=fleece sslmode=disable"),
		PostgresDriver:     envOr("POSTGRES_DRIVER", "postgres"),
		PostgresSearchPath: envOr("POSTGRES_SEARCH_PATH", "routing"),
		DefaultCurrency:    envOr("DEFAULT_CURRENCY", "XAF"),
	}
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
