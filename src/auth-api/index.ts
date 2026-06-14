// Composition root du service auth-api (Identity Service).
// Câble l'infrastructure et les adapters dans les use cases (Clean Architecture).
// Point d'entrée compilé par esbuild (voir mk/node.mk).
// DI manuelle — pas de framework DI.

import { createServer } from "node:http";
import { DrizzleWorkspaceRepository } from "./adapters/persistence/workspace-repository.js";
import { DrizzleApiKeyRepository } from "./adapters/persistence/api-key-repository.js";
import { DrizzleUserRepository } from "./adapters/persistence/user-repository.js";
import { BetterAuthProvider } from "./adapters/auth/better-auth.adapter.js";
import { CreateWorkspace } from "./application/use-cases/create-workspace.js";
import { CreateApiKey } from "./application/use-cases/create-api-key.js";
import { ValidateApiKey } from "./application/use-cases/validate-api-key.js";
import { RevokeApiKey } from "./application/use-cases/revoke-api-key.js";
import { RotateApiKey } from "./application/use-cases/rotate-api-key.js";
import { AuthHandler } from "./adapters/http/auth.handler.js";
import { auth } from "./infrastructure/db/better-auth.config.js";

const PORT = parseInt(process.env["PORT"] ?? "3001", 10);

async function main(): Promise<void> {
  // -------------------------------------------------------------------------
  // Couche 4 — Infrastructure : connexion base de données (stub offline)
  // -------------------------------------------------------------------------

  // TODO: remplacer par la connexion Drizzle réelle une fois drizzle-orm/pg installés :
  // import { drizzle } from 'drizzle-orm/node-postgres';
  // import { Pool } from 'pg';
  // const pool = new Pool({ connectionString: process.env.DATABASE_URL });
  // const db = drizzle(pool, { schema });

  // Stub offline — les repositories lèveront une erreur explicite à l'appel réel.
  // TODO: remplacer par l'instance Drizzle réelle (voir commentaire ci-dessus).
  const db = {} as import("./adapters/persistence/workspace-repository.js").DrizzleDb;

  // -------------------------------------------------------------------------
  // Couche 3 — Adapters : repositories + auth provider
  // -------------------------------------------------------------------------

  const workspaceRepo = new DrizzleWorkspaceRepository(db);
  const apiKeyRepo = new DrizzleApiKeyRepository(db);
  const userRepo = new DrizzleUserRepository(db);

  // BetterAuthProvider wrappant l'instance Better Auth (stub offline)
  const authProvider = new BetterAuthProvider(auth);

  // -------------------------------------------------------------------------
  // Couche 2 — Use cases
  // -------------------------------------------------------------------------

  const createWorkspace = new CreateWorkspace(workspaceRepo, userRepo);
  const createApiKey = new CreateApiKey(apiKeyRepo);
  const validateApiKey = new ValidateApiKey(apiKeyRepo);
  const revokeApiKey = new RevokeApiKey(apiKeyRepo);
  const rotateApiKey = new RotateApiKey(apiKeyRepo);

  // -------------------------------------------------------------------------
  // Couche 3 — Handler HTTP (authProvider injecté pour POST /validate-session)
  // -------------------------------------------------------------------------

  const handler = new AuthHandler(
    createWorkspace,
    createApiKey,
    revokeApiKey,
    validateApiKey,
    rotateApiKey,
    authProvider,
  );

  // -------------------------------------------------------------------------
  // Couche 4 — Serveur HTTP (node:http stdlib)
  // -------------------------------------------------------------------------

  const server = createServer((req, res) => {
    handler.handle(req, res).catch((err: unknown) => {
      console.error("Unhandled error in AuthHandler:", err);
      res.writeHead(500, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ code: "INTERNAL_ERROR", message: "Erreur interne" }));
    });
  });

  server.listen(PORT, () => {
    console.log(`auth-api (identity) démarré sur le port ${PORT}`);
  });

  // Graceful shutdown
  process.on("SIGTERM", () => {
    console.log("SIGTERM reçu — arrêt gracieux...");
    server.close(() => {
      console.log("Serveur arrêté.");
      process.exit(0);
    });
  });
}

void main();
