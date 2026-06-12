// Composition root du service auth-api (Identity Service).
// Câble l'infrastructure et les adapters dans les use cases (Clean Architecture).
// Point d'entrée compilé par esbuild (voir mk/node.mk).

import { CreateApiKey } from "./application/use-cases/create-api-key";

async function main(): Promise<void> {
  // TODO: ouvrir Postgres (schéma "identity"), initialiser Better Auth (adapter
  // Drizzle), construire les repositories, les injecter dans les use cases,
  // puis démarrer le serveur HTTP (API REST interne).
  void CreateApiKey;
  console.log("auth-api (identity) bootstrap");
}

void main();
