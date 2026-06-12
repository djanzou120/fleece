// Command routing est le point d'entree (composition root) du routing Service.
//
// Il cable l'infrastructure et les adapters dans les use cases en respectant
// la regle de dependance de la Clean Architecture (voir .ia/ARCHITECTURE.md).
package main

import "fleece/src/go/app"

func main() {
	app.Bootstrap("routing")
	// TODO: charger la config, ouvrir Postgres (schema "routing"), connecter
	// RabbitMQ/Redis, construire les adapters, les injecter dans les use cases,
	// demarrer le serveur HTTP et/ou les consumers.
}
