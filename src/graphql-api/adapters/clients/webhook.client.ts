// Couche 3 — Client REST driven : implémentation de WebhookClient vers le Webhook Service (Go).
// Utilise fetch global (Node 18+) — pas de package npm.
// Convention stubs : retour valeurs vides / [] — voir identity.client.ts pour détails.

import type { ApiContext } from "@fleece/api-common";
import type {
  WebhookClient,
  WebhookEndpointDTO,
} from "../../application/ports/output/clients.js";

// ---------------------------------------------------------------------------
// Helper interne
// ---------------------------------------------------------------------------

function buildHeaders(ctx: ApiContext): Record<string, string> {
  return {
    "Content-Type": "application/json",
    "X-Workspace-Id": ctx.workspaceId,
    // TODO(production): ajouter le token interne service-to-service
  };
}

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
  void buildHeaders;
  return null;
}

// ---------------------------------------------------------------------------
// Implémentation
// ---------------------------------------------------------------------------

export class WebhookRestClient implements WebhookClient {
  constructor(private readonly baseUrl: string) {}

  async listEndpoints(ctx: ApiContext, workspaceId: string): Promise<WebhookEndpointDTO[]> {
    // TODO(production): GET {baseUrl}/webhook-endpoints?workspaceId=...
    const result = await request<WebhookEndpointDTO[]>(
      this.baseUrl,
      "GET",
      `/webhook-endpoints?workspaceId=${encodeURIComponent(workspaceId)}`,
      ctx,
    );
    return result ?? [];
  }

  async createEndpoint(
    ctx: ApiContext,
    workspaceId: string,
    payload: { url: string; events: string[] },
  ): Promise<WebhookEndpointDTO> {
    // TODO(production): POST {baseUrl}/webhook-endpoints { workspaceId, url, events }
    const result = await request<WebhookEndpointDTO>(
      this.baseUrl,
      "POST",
      "/webhook-endpoints",
      ctx,
      { workspaceId, ...payload },
    );
    // Stub offline : retourne un endpoint fictif pour permettre le développement du frontend
    return result ?? {
      id: "stub-endpoint-id",
      workspaceId,
      url: payload.url,
      events: payload.events,
      active: true,
      createdAt: new Date().toISOString(),
    };
  }

  async deleteEndpoint(ctx: ApiContext, workspaceId: string, id: string): Promise<void> {
    // TODO(production): DELETE {baseUrl}/webhook-endpoints/{id}?workspaceId=...
    await request<void>(
      this.baseUrl,
      "DELETE",
      `/webhook-endpoints/${id}?workspaceId=${encodeURIComponent(workspaceId)}`,
      ctx,
    );
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: WebhookClient = new WebhookRestClient("http://localhost:3004");
void _conformance;
