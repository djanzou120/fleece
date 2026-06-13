// Composition root du Public REST API Gateway.
// Câble l'infrastructure (serveur HTTP, middleware auth) et les adapters
// (routes, clients vers les services internes) — DI manuelle, pas de framework DI.
// Voir .ia/ARCHITECTURE.md §4.3.

import { startServer } from "./infrastructure/server";

startServer();
