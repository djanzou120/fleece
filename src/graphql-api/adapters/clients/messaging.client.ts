// Couche 3 — Client REST driven : implémentation de MessagingClient vers le Messaging Service (Go).
// Utilise fetch global (Node 18+) — pas de package npm.
// Convention stubs : retour valeurs vides — voir identity.client.ts pour détails.

import type { ApiContext, Page, PageArgs } from "@fleece/api-common";
import type {
  MessagingClient,
  MessageDTO,
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

export class MessagingRestClient implements MessagingClient {
  constructor(private readonly baseUrl: string) {}

  async listMessages(
    ctx: ApiContext,
    workspaceId: string,
    page: PageArgs,
  ): Promise<Page<MessageDTO>> {
    // TODO(production): GET {baseUrl}/messages?workspaceId=...&cursor=...&limit=...
    const params = new URLSearchParams({ workspaceId });
    if (page.cursor !== undefined) params.set("cursor", page.cursor);
    if (page.limit !== undefined) params.set("limit", String(page.limit));

    const result = await request<Page<MessageDTO>>(
      this.baseUrl,
      "GET",
      `/messages?${params.toString()}`,
      ctx,
    );
    // Page vide si le service est indisponible (stub offline)
    return result ?? { items: [], pageInfo: { nextCursor: null, hasNextPage: false } };
  }

  async getMessage(
    ctx: ApiContext,
    workspaceId: string,
    id: string,
  ): Promise<MessageDTO | null> {
    // TODO(production): GET {baseUrl}/messages/{id}?workspaceId=...
    return request<MessageDTO>(
      this.baseUrl,
      "GET",
      `/messages/${id}?workspaceId=${encodeURIComponent(workspaceId)}`,
      ctx,
    );
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: MessagingClient = new MessagingRestClient("http://localhost:3003");
void _conformance;
