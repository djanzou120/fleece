// Couche 4 — Infrastructure : chargement de la configuration depuis les variables d'environnement.
// Valeurs par défaut réalistes pour le développement local (docker-compose).
// Appelé une seule fois au démarrage dans le composition root (index.ts).

// ---------------------------------------------------------------------------
// Interface de configuration
// ---------------------------------------------------------------------------

export interface Config {
  /** Port d'écoute du serveur GraphQL (défaut : 4000). */
  port: number;

  /** URL du service auth-api (Identity Service — TypeScript). */
  authApiUrl: string;

  /**
   * URL du service HTTP Go unifié `src/api` (ex-messaging/routing/provider/wallet/
   * webhook/contact-intelligence/campaign/analytics — un seul binaire, un seul port
   * depuis la migration `src/api`, cf. `.ia/MIGRATION_PLAN.md` M-025). Toutes les
   * requêtes REST internes (wallet, messages, webhooks, …) partent de cette base.
   */
  apiUrl: string;
}

// ---------------------------------------------------------------------------
// Chargement
// ---------------------------------------------------------------------------

/**
 * Charge et valide la configuration depuis les variables d'environnement.
 * Utilisé les valeurs par défaut si une variable est absente (développement local).
 */
export function loadConfig(): Config {
  return {
    port: parseInt(process.env["PORT"] ?? "4000", 10),

    // auth-api écoute sur 3001 (voir auth-api/index.ts) — service TS distinct, inchangé.
    authApiUrl: process.env["AUTH_API_URL"] ?? "http://localhost:3001",

    // src/api (Go, service HTTP unifié) écoute sur 8080 (cf. src/api/cmd/api/main.go).
    apiUrl: process.env["API_URL"] ?? "http://localhost:8080",
  };
}
