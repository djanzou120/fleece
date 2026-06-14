// Couche 4 — Infrastructure : serveur GraphQL.
// Monte le schéma, les résolveurs et injecte le contexte via buildContext.
//
// Apollo Server / GraphQL Yoga NON installés (contrainte offline).
// L'interface GraphQLServerLike est stubée ; la structure montre le câblage réel.
// TODO(production): décommenter les imports Apollo et retirer le stub.
//
// TODO: import { ApolloServer } from '@apollo/server'
// TODO: import { startStandaloneServer } from '@apollo/server/standalone'

import { createServer, IncomingMessage, ServerResponse } from "node:http";
import type { SessionValidator } from "@fleece/api-common";
import { buildContext } from "../adapters/graphql/context.js";
import type { Config } from "./config.js";

// ---------------------------------------------------------------------------
// Import du schéma GraphQL (chargé en texte par esbuild --loader:.graphql=text)
// ---------------------------------------------------------------------------

// TODO(production): l'import suivant fonctionne à l'exécution via esbuild.
// tsc ne lève pas d'erreur grâce à graphql.d.ts (declare module "*.graphql").
import typeDefs from "../adapters/graphql/schema.graphql";

// ---------------------------------------------------------------------------
// Interface locale stubant le serveur GraphQL (framework non installé)
// ---------------------------------------------------------------------------

/**
 * Interface minimale représentant un serveur GraphQL.
 * TODO(production): remplacée par ApolloServer ou YogaServer lors de l'installation des packages.
 */
interface GraphQLServerLike {
  /** Démarre le serveur et écoute sur le port spécifié. */
  listen(port: number, callback?: () => void): void;
  /** Arrête le serveur (graceful shutdown). */
  stop(): Promise<void>;
}

// ---------------------------------------------------------------------------
// Options d'entrée du serveur
// ---------------------------------------------------------------------------

export interface ServerOptions {
  /** Schéma SDL (SDL string) — déjà importé via typeDefs. */
  typeDefs?: string;

  /**
   * Map des résolveurs fusionnés { Query: {...}, Mutation: {...} }.
   * Construit par buildResolvers(deps) dans index.ts.
   */
  resolvers: Record<string, Record<string, unknown>>;

  /** Validateur de session injecté (couche 4, HttpSessionValidator). */
  sessionValidator: SessionValidator;

  /** Configuration chargée depuis l'environnement. */
  config: Config;
}

// ---------------------------------------------------------------------------
// Stub d'exécuteur GraphQL (remplace ApolloServer hors ligne)
// ---------------------------------------------------------------------------

/**
 * Gestionnaire HTTP stub : répond 501 en attendant l'installation d'Apollo/Yoga.
 * Montre la structure de câblage : contexte extrait via buildContext, puis transmis aux résolveurs.
 *
 * TODO(production): remplacer cette fonction par ApolloServer + startStandaloneServer :
 *   const server = new ApolloServer({ typeDefs, resolvers });
 *   const { url } = await startStandaloneServer(server, {
 *     listen: { port: opts.config.port },
 *     context: async ({ req }) => buildContext(req, opts.sessionValidator),
 *   });
 */
async function handleRequest(
  req: IncomingMessage,
  res: ServerResponse,
  opts: ServerOptions,
): Promise<void> {
  // Adaptation node:http → IncomingRequest (interface locale de context.ts)
  const incomingRequest = {
    headers: {
      get: (name: string): string | null => {
        const val = req.headers[name.toLowerCase()];
        if (Array.isArray(val)) return val.join(", ");
        return val ?? null;
      },
    },
  };

  try {
    // Validation de la session — lève ApiError("UNAUTHORIZED") si invalide
    const _ctx = await buildContext(incomingRequest, opts.sessionValidator);
    void _ctx; // ctx sera passé aux résolveurs dans la version Apollo

    // Stub : informe que le serveur GraphQL réel n'est pas encore câblé
    const json = JSON.stringify({
      errors: [
        {
          message: "Serveur GraphQL non câblé (stub offline) — voir infrastructure/server.ts",
          extensions: { code: "NOT_IMPLEMENTED" },
        },
      ],
    });
    res.writeHead(501, { "Content-Type": "application/json" });
    res.end(json);
  } catch (err) {
    // Erreur de session ou d'exécution
    const message = err instanceof Error ? err.message : "Erreur interne";
    const code = (err as { code?: string }).code ?? "INTERNAL_ERROR";
    res.writeHead(401, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ errors: [{ message, extensions: { code } }] }));
  }
}

// ---------------------------------------------------------------------------
// startServer — point d'entrée appelé depuis index.ts
// ---------------------------------------------------------------------------

/**
 * Démarre le serveur GraphQL BFF.
 * Monte le schéma SDL + résolveurs + middleware de contexte (session JWT).
 * Gère le SIGTERM pour le graceful shutdown.
 */
export function startServer(opts: ServerOptions): void {
  // Le schéma SDL est prêt (importé via esbuild)
  const schema = typeDefs;
  void schema; // TODO(production): passer à ApolloServer({ typeDefs: schema, resolvers })

  // TODO(production): remplacer node:http par ApolloServer :
  // const apolloServer = new ApolloServer({ typeDefs: schema, resolvers: opts.resolvers });
  // await startStandaloneServer(apolloServer, {
  //   listen: { port: opts.config.port },
  //   context: async ({ req }) => buildContext(req, opts.sessionValidator),
  // });

  const server = createServer((req, res) => {
    handleRequest(req, res, opts).catch((err: unknown) => {
      console.error("Erreur non gérée dans handleRequest:", err);
      res.writeHead(500, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ errors: [{ message: "Erreur interne" }] }));
    });
  });

  server.listen(opts.config.port, () => {
    console.log(
      `graphql-api (BFF dashboard) démarré sur le port ${opts.config.port}`,
    );
    console.log(
      "Schéma GraphQL chargé — résolveurs montés (stub offline : Apollo non installé)",
    );
  });

  // Graceful shutdown
  process.on("SIGTERM", () => {
    console.log("SIGTERM reçu — arrêt gracieux du BFF GraphQL...");
    server.close(() => {
      console.log("Serveur GraphQL arrêté.");
      process.exit(0);
    });
  });
}
