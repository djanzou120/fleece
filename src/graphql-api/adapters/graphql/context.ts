// Couche 3 — Adapter GraphQL : construction du contexte GraphQL à partir de la requête HTTP.
// Extrait le token (header Authorization Bearer ou cookie), valide via SessionValidator,
// lève une ApiError("UNAUTHORIZED") si absent ou invalide.
// Reste framework-agnostic : la requête est typée via une interface locale minimale.

import type { ApiContext, SessionValidator } from "@fleece/api-common";
import { ApiError } from "@fleece/api-common";

// ---------------------------------------------------------------------------
// Interface minimale de requête — framework-agnostic
// Permet de consommer ce fichier depuis Apollo, GraphQL Yoga ou node:http.
// ---------------------------------------------------------------------------

/** Représentation minimale d'une requête HTTP entrant dans le BFF GraphQL. */
export interface IncomingRequest {
  headers: {
    get(name: string): string | null;
    // Compatibilité Map-like ou objet — le consommateur adapte au framework réel
  };
  /** Valeur brute du header "cookie" si le framework ne l'expose pas via headers.get. */
  cookieHeader?: string;
}

// ---------------------------------------------------------------------------
// Extraction du token
// ---------------------------------------------------------------------------

/**
 * Extrait le token de session depuis la requête.
 *
 * Priorité :
 * 1. Header `Authorization: Bearer <token>`
 * 2. Cookie `session` (valeur brute)
 *
 * Retourne null si aucun token trouvé.
 */
function extractToken(req: IncomingRequest): string | null {
  // 1. Header Authorization Bearer
  const authHeader = req.headers.get("authorization");
  if (authHeader !== null && authHeader.startsWith("Bearer ")) {
    return authHeader.slice("Bearer ".length).trim() || null;
  }

  // 2. Cookie "session"
  const cookieRaw = req.cookieHeader ?? req.headers.get("cookie");
  if (cookieRaw !== null) {
    // Parsing simple : "key=value; key2=value2"
    for (const part of cookieRaw.split(";")) {
      const [rawKey, ...rest] = part.trim().split("=");
      if (rawKey !== undefined && rawKey.trim() === "session") {
        return rest.join("=").trim() || null;
      }
    }
  }

  return null;
}

// ---------------------------------------------------------------------------
// buildContext — appelé par le serveur GraphQL pour chaque requête
// ---------------------------------------------------------------------------

/**
 * Construit le contexte GraphQL pour la requête entrante.
 *
 * @throws {ApiError} code UNAUTHORIZED si le token est absent ou invalide.
 */
export async function buildContext(
  req: IncomingRequest,
  sessionValidator: SessionValidator,
): Promise<ApiContext> {
  const token = extractToken(req);

  if (token === null) {
    throw new ApiError("UNAUTHORIZED", "Token de session manquant", 401);
  }

  const ctx = await sessionValidator.validate(token);

  if (ctx === null) {
    throw new ApiError("UNAUTHORIZED", "Session invalide ou expirée", 401);
  }

  return ctx;
}
