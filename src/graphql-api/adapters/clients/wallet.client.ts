// Couche 3 — Client REST driven : implémentation de WalletClient vers le Wallet Service (Go).
// Utilise fetch global (Node 18+) — pas de package npm.
// Convention stubs : retour valeurs vides / défaut — voir identity.client.ts pour détails.

import type { ApiContext, Page, PageArgs } from "@fleece/api-common";
import type {
  WalletClient,
  WalletBalanceDTO,
  TransactionDTO,
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

export class WalletRestClient implements WalletClient {
  constructor(private readonly baseUrl: string) {}

  async getBalance(ctx: ApiContext, workspaceId: string): Promise<WalletBalanceDTO> {
    // TODO(production): GET {baseUrl}/wallets/{workspaceId}/balance
    const result = await request<WalletBalanceDTO>(
      this.baseUrl,
      "GET",
      `/wallets/${workspaceId}/balance`,
      ctx,
    );
    // Valeur par défaut si le service est indisponible (stub offline)
    return result ?? { workspaceId, balance: 0, currency: "XOF" };
  }

  async listTransactions(
    ctx: ApiContext,
    workspaceId: string,
    page: PageArgs,
  ): Promise<Page<TransactionDTO>> {
    // TODO(production): GET {baseUrl}/wallets/{workspaceId}/transactions?cursor=...&limit=...
    const params = new URLSearchParams();
    if (page.cursor !== undefined) params.set("cursor", page.cursor);
    if (page.limit !== undefined) params.set("limit", String(page.limit));

    const result = await request<Page<TransactionDTO>>(
      this.baseUrl,
      "GET",
      `/wallets/${workspaceId}/transactions?${params.toString()}`,
      ctx,
    );
    // Page vide si le service est indisponible (stub offline)
    return result ?? { items: [], pageInfo: { nextCursor: null, hasNextPage: false } };
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: WalletClient = new WalletRestClient("http://localhost:3002");
void _conformance;
