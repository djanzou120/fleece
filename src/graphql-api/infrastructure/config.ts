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

  /** URL du Wallet Service (Go). */
  walletApiUrl: string;

  /** URL du Messaging Service (Go). */
  messagingApiUrl: string;

  /** URL du Webhook Service (Go). */
  webhookApiUrl: string;
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

    // auth-api écoute sur 3001 (voir auth-api/index.ts)
    authApiUrl: process.env["AUTH_API_URL"] ?? "http://localhost:3001",

    // Services Go — ports conventionnels du docker-compose local
    walletApiUrl: process.env["WALLET_API_URL"] ?? "http://localhost:3002",
    messagingApiUrl: process.env["MESSAGING_API_URL"] ?? "http://localhost:3003",
    webhookApiUrl: process.env["WEBHOOK_API_URL"] ?? "http://localhost:3004",
  };
}
