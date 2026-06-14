// Couche 4 — Infrastructure : implémentation concrète de SessionValidator.
// Appelle auth-api /validate-session via fetch global (Node 18+).
// Better Auth est confiné dans auth-api ; ici on consomme uniquement l'endpoint HTTP.
// Stub offline : retourne null (session invalide) par défaut.

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
 * Valide un token de session JWT en appelant auth-api.
 * Retourne un ApiContext si le token est valide, null sinon.
 *
 * TODO(production): remplacer le stub par l'appel fetch réel (voir corps de validate).
 */
export class HttpSessionValidator implements SessionValidator {
  constructor(private readonly authApiUrl: string) {}

  async validate(token: string): Promise<ApiContext | null> {
    // TODO(production): remplacer par l'appel fetch réel :
    // try {
    //   const resp = await fetch(`${this.authApiUrl}/validate-session`, {
    //     method: "POST",
    //     headers: {
    //       "Content-Type": "application/json",
    //       // TODO(production): ajouter le token interne service-to-service si nécessaire
    //     },
    //     body: JSON.stringify({ token }),
    //   });
    //   if (!resp.ok) return null;
    //   const data = await resp.json() as ValidateSessionResponse;
    //   return {
    //     workspaceId: data.workspaceId,
    //     userId: data.userId,
    //     authMethod: data.authMethod,
    //   };
    // } catch {
    //   // Indisponibilité temporaire de auth-api → rejet de la session
    //   return null;
    // }

    // Stub offline : auth-api non disponible, toute session est rejetée
    void token;
    void this.authApiUrl;
    return null;
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: SessionValidator = new HttpSessionValidator("http://localhost:3001");
void _conformance;

// Référence de type pour éviter l'avertissement "déclaré mais jamais lu" sur l'interface.
// L'interface est utilisée dans le code commenté TODO(production).
type _ValidateSessionResponseRef = ValidateSessionResponse;
void (undefined as unknown as _ValidateSessionResponseRef);
