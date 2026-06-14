// Couche 3 — Client REST driven : implémentation de IdentityClient vers auth-api.
// Utilise fetch global (Node 18+) — pas de package npm.
// Stubs réalistes : les méthodes retournent des valeurs vides/null plutôt que de throw,
// avec TODO(production) explicite sur chaque appel REST réel.
//
// Convention stubs : retour valeurs vides (null / []) — cohérence pour le BFF dashboard
// qui préfère afficher une interface vide plutôt qu'une erreur sur indisponibilité partielle.

import type { ApiContext } from "@fleece/api-common";
import type {
  IdentityClient,
  WorkspaceDTO,
  ApiKeyDTO,
} from "../../application/ports/output/clients.js";

// ---------------------------------------------------------------------------
// Helper interne : construction des headers d'autorisation inter-services
// ---------------------------------------------------------------------------

/**
 * Construit les headers pour les appels REST inter-services.
 * TODO(production): ajouter le token interne service-to-service (HMAC ou JWT service).
 */
function buildHeaders(ctx: ApiContext): Record<string, string> {
  return {
    "Content-Type": "application/json",
    // En-tête interne permettant à auth-api d'identifier le workspace appelant
    "X-Workspace-Id": ctx.workspaceId,
    // TODO(production): ajouter "Authorization: Bearer <service-token>" signé par le BFF
  };
}

// ---------------------------------------------------------------------------
// Helper interne : appel HTTP sortant
// ---------------------------------------------------------------------------

/**
 * Effectue un appel HTTP vers auth-api.
 * Retourne null si la réponse n'est pas 2xx (au lieu de throw) pour éviter
 * les erreurs partielles côté dashboard.
 *
 * TODO(production): gérer les erreurs réseau, timeouts, retries.
 */
async function request<T>(
  baseUrl: string,
  method: string,
  path: string,
  ctx: ApiContext,
  body?: unknown,
): Promise<T | null> {
  // TODO(production): remplacer par l'appel fetch réel :
  // const resp = await fetch(`${baseUrl}${path}`, {
  //   method,
  //   headers: buildHeaders(ctx),
  //   body: body !== undefined ? JSON.stringify(body) : undefined,
  // });
  // if (!resp.ok) return null;
  // return resp.json() as Promise<T>;
  void baseUrl;
  void method;
  void path;
  void ctx;
  void body;
  void buildHeaders; // utilisé dans la version production
  return null;
}

// ---------------------------------------------------------------------------
// Implémentation
// ---------------------------------------------------------------------------

export class IdentityRestClient implements IdentityClient {
  constructor(private readonly baseUrl: string) {}

  async getWorkspace(ctx: ApiContext, workspaceId: string): Promise<WorkspaceDTO | null> {
    // TODO(production): GET {baseUrl}/workspaces/{workspaceId}
    return request<WorkspaceDTO>(this.baseUrl, "GET", `/workspaces/${workspaceId}`, ctx);
  }

  async listApiKeys(ctx: ApiContext, workspaceId: string): Promise<ApiKeyDTO[]> {
    // TODO(production): GET {baseUrl}/workspaces/{workspaceId}/api-keys
    const result = await request<ApiKeyDTO[]>(
      this.baseUrl,
      "GET",
      `/workspaces/${workspaceId}/api-keys`,
      ctx,
    );
    return result ?? [];
  }

  async createApiKey(
    ctx: ApiContext,
    workspaceId: string,
    name: string,
  ): Promise<{ rawKey: string; apiKey: ApiKeyDTO }> {
    // TODO(production): POST {baseUrl}/workspaces/{workspaceId}/api-keys { name }
    const result = await request<{ rawKey: string; apiKey: ApiKeyDTO }>(
      this.baseUrl,
      "POST",
      `/workspaces/${workspaceId}/api-keys`,
      ctx,
      { name },
    );
    if (result === null) {
      // Stub offline : retourne une clé fictive pour permettre le développement du frontend
      return {
        rawKey: "fleece_stub_raw_key",
        apiKey: {
          id: "stub-id",
          workspaceId,
          name,
          prefix: "fleece_stub",
          active: true,
          createdAt: new Date().toISOString(),
          expiresAt: null,
        },
      };
    }
    return result;
  }

  async revokeApiKey(ctx: ApiContext, workspaceId: string, keyId: string): Promise<void> {
    // TODO(production): DELETE {baseUrl}/workspaces/{workspaceId}/api-keys/{keyId}
    await request<void>(
      this.baseUrl,
      "DELETE",
      `/workspaces/${workspaceId}/api-keys/${keyId}`,
      ctx,
    );
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: IdentityClient = new IdentityRestClient("http://localhost:3001");
void _conformance;
