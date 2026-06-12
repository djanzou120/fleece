// Composition root du service graphql-api (GraphQL Gateway / BFF).
// Point d'entrée compilé par esbuild (voir mk/node.mk).

import { startServer } from "./infrastructure/server";

startServer();
