// Couche 4 — Infrastructure : serveur GraphQL (graphql-yoga).
// Monte le schéma SDL + résolveurs + middleware de contexte (session JWT).
// graphql-yoga v5 : createYoga + createSchema + node:http.

import { createYoga, createSchema } from "graphql-yoga";
import { createServer } from "node:http";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import type { SessionValidator } from "@fleece/api-common";
import { buildContext } from "../adapters/graphql/context.js";
import type { Config } from "./config.js";

// ---------------------------------------------------------------------------
// Schéma SDL — lu directement depuis le fichier source
// (évite la dépendance au loader esbuild --loader:.graphql=text en dev)
// ---------------------------------------------------------------------------
const typeDefs = readFileSync(
  resolve(__dirname, "../adapters/graphql/schema.graphql"),
  "utf-8",
);

// ---------------------------------------------------------------------------
// Options d'entrée du serveur
// ---------------------------------------------------------------------------

export interface ServerOptions {
  resolvers: Record<string, Record<string, unknown>>;
  sessionValidator: SessionValidator;
  config: Config;
}

// ---------------------------------------------------------------------------
// startServer — point d'entrée appelé depuis index.ts
// ---------------------------------------------------------------------------

export function startServer(opts: ServerOptions): void {
  const yoga = createYoga({
    schema: createSchema({
      typeDefs,
      resolvers: opts.resolvers as Parameters<typeof createSchema>[0]["resolvers"],
    }),
    context: async ({ request }: { request: Request }) => {
      try {
        return await buildContext(
          {
            headers: {
              get: (name: string) => request.headers.get(name),
            },
          },
          opts.sessionValidator,
        );
      } catch {
        // Contexte non authentifié — graphql-yoga renvoie l'erreur via le résolveur
        return {};
      }
    },
    graphqlEndpoint: "/graphql",
    landingPage: true,
  });

  const server = createServer(yoga);

  server.listen(opts.config.port, () => {
    console.log(`\n🟢 graphql-api (BFF dashboard) sur http://localhost:${opts.config.port}/graphql\n`);
  });

  process.on("SIGTERM", () => {
    console.log("SIGTERM reçu — arrêt gracieux du BFF GraphQL...");
    server.close(() => {
      console.log("Serveur GraphQL arrêté.");
      process.exit(0);
    });
  });
}
