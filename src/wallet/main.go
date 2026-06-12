// Command wallet est le point d'entree (composition root) du wallet Service.
//
// Il cable l'infrastructure et les adapters dans les use cases en respectant
// la regle de dependance de la Clean Architecture (voir .ia/ARCHITECTURE.md).
package main

import "fleece/src/go/app"

func main() {
	app.Bootstrap("wallet")
	// TODO: charger la config, ouvrir Postgres (schema "wallet"), connecter
	// RabbitMQ/Redis, construire les adapters, les injecter dans les use cases,
	// demarrer le serveur HTTP et/ou les consumers.
}
