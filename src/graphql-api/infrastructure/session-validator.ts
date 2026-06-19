// Couche 4 — Infrastructure : implémentation concrète de SessionValidator.
// Appelle auth-api /validate-session via fetch global (Node 18+).
// Better Auth est confiné dans auth-api ; ici on consomme uniquement l'endpoint HTTP.

import type { ApiContext, SessionValidator } from "@fleece/api-common";

// ---------------------------------------------------------------------------
// Réponse attendue de auth-api /validate-session
// ---------------------------------------------------------------------------

interface ValidateSessionResponse {
  workspaceId: string;
  userId: string;
  authMethod: "session";
}

// ---------------------------------------------------------------------------
// Implémentation
// ---------------------------------------------------------------------------

/**
 * Valide un token de session JWT en appelant auth-api POST /validate-session.
 * Retourne un ApiContext complet { workspaceId, userId, authMethod } si le token est valide,
 * null si la session est invalide/expirée ou si auth-api est indisponible.
 */
export class HttpSessionValidator implements SessionValidator {
  constructor(private readonly authApiUrl: string) {}

  async validate(token: string): Promise<ApiContext | null> {
    // Appel HTTP vers auth-api /validate-session.
    // Utilise le fetch global (Node 18+ — pas d'import npm nécessaire).
    // Fallback propre : retourne null si auth-api est indisponible ou retourne non-OK.
    try {
      const resp = await fetch(`${this.authApiUrl}/validate-session`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ token }),
      });
      if (!resp.ok) return null;
      const data = await resp.json() as ValidateSessionResponse;
      const context: ApiContext = {
        workspaceId: data.workspaceId,
        userId: data.userId,
        authMethod: data.authMethod,
      };
      return context;
    } catch {
      // Indisponibilité temporaire de auth-api → rejet de la session
      return null;
    }
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: SessionValidator = new HttpSessionValidator("http://localhost:3001");
void _conformance;
