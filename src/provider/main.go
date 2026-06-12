// Command provider est le point d'entree (composition root) du provider Service.
//
// Il cable l'infrastructure et les adapters dans les use cases en respectant
// la regle de dependance de la Clean Architecture (voir .ia/ARCHITECTURE.md).
package main

import "fleece/src/go/app"

func main() {
	app.Bootstrap("provider")
	// TODO: charger la config, ouvrir Postgres (schema "provider"), connecter
	// RabbitMQ/Redis, construire les adapters, les injecter dans les use cases,
	// demarrer le serveur HTTP et/ou les consumers.
}
