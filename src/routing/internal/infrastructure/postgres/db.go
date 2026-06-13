// Package postgres gere le pool de connexions PostgreSQL du service routing.
//
// IMPORTANT : ce package n'importe AUCUN driver PostgreSQL concret.
// Le driver doit etre enregistre dans le composition root (main.go) via un
// import anonyme "_ <driver>" AVANT d'appeler Open().
// Exemple : import _ "github.com/lib/pq"  (a ajouter quand la dependance sera disponible)
//
// TODO(driver): ajouter l'import du driver concret dans src/routing/main.go
// une fois que go.sum sera initialise avec la dependance choisie.
package postgres

import (
	"database/sql"
	"fmt"
	"time"
)

// DB regroupe la connexion ouverte et la configuration.
type DB struct {
	*sql.DB
}

// Open ouvre un pool de connexions via sql.Open(driver, dsn), configure les
// parametres du pool et verifie la connectivite avec Ping.
// Le parametre searchPath est injecte via l'option DSN si non vide.
func Open(driver, dsn, searchPath string) (*DB, error) {
	if searchPath != "" {
		// On configure le search_path au niveau de la connexion via options DSN.
		// Avec lib/pq on peut ajouter "search_path=<schema>" a la DSN ;
		// sinon on executera SET search_path apres ouverture.
		dsn = fmt.Sprintf("%s search_path=%s", dsn, searchPath)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: sql.Open: %w", err)
	}

	// Configuration du pool — valeurs prudentes pour un microservice.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}

	return &DB{db}, nil
}
