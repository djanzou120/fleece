// Contexte d'authentification résolu par le middleware de chaque gateway.
// Partagé entre rest-api (API Key) et graphql-api (session JWT).
// Aucune dépendance framework — types purs.

export type AuthMethod = "api_key" | "session";

export interface ApiContext {
  workspaceId: string;
  userId?: string; // présent pour les sessions dashboard
  apiKeyId?: string; // présent pour les requêtes API Key
  authMethod: AuthMethod;
}

// Port de validation d'une API Key (implémenté par l'adapter appelant auth-api).
export interface ApiKeyValidator {
  validate(rawKey: string): Promise<ApiContext | null>;
}

// Port de validation d'une session JWT (implémenté par l'adapter appelant auth-api).
export interface SessionValidator {
  validate(token: string): Promise<ApiContext | null>;
}
