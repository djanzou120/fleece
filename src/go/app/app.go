// Package app regroupe les métadonnées et l'amorçage communs aux services Go.
//
// Version et Name sont injectés au build via -ldflags (voir mk/go.mk).
// Cette lib est transverse : elle ne contient AUCUNE règle métier.
package app

import "log"

var (
	// Version est injectée au build.
	Version = "dev"
	// Name est injecté au build (nom du service).
	Name = "unknown"
)

// Bootstrap initialise les préoccupations transverses communes
// (logs structurés, trace-id, etc.) au démarrage d'un service.
func Bootstrap(service string) {
	if Name == "unknown" {
		Name = service
	}
	log.Printf("starting service=%s version=%s", Name, Version)
}
