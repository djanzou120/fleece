// Couche 3 — Client REST driven : implémentation de WalletClient vers le service HTTP
// unifié `src/api` (Go — routes /wallet/*, cf. src/api/service.go registerRoutes / M-025).
// Utilise fetch global (Node 18+) — pas de package npm.
// Convention stubs : retour valeurs vides / défaut — voir identity.client.ts pour détails.
//
// Mapping balance (M-025) :
//   Backend Go → `GET /wallet/{workspaceId}` (src/api/wallet_get.go) → objet JSON unique.
//   Champs backend (snake_case) : { workspace_id, balance:number (bigint centimes),
//                      balance_cents:number (alias == balance), currency, updated_at:RFC3339 }
//   Le chemin est bien `/wallet/...` (singulier) et NON `/wallets/...` — écart corrigé M-025
//   (l'ancien Wallet Service Go exposait `/wallets/...` sur son propre port ; src/api agrège
//   sous `/wallet/...`, cf. service.go `GET /wallet/{workspaceId}`).
//
// Mapping transactions (inchangé dans sa forme, chemin corrigé M-025) :
//   Backend Go → `GET /wallet/{workspaceId}/transactions` (src/api/wallet_transactions.go)
//   → tableau JSON brut.
//   Champs backend : { id:number, workspace_id, kind:"debit"|"credit"|"refund",
//                      amount:number (centimes), message_id:string|null, created_at:RFC3339 }
//   La devise n'est PAS dans la réponse transaction — récupérée via getBalance.
//   La pagination cursor-based est appliquée côté BFF sur la liste complète.

import type { ApiContext, Page, PageArgs } from "@fleece/api-common";
import type {
  WalletClient,
  WalletBalanceDTO,
  TransactionDTO,
} from "../../application/ports/output/clients.js";

// ---------------------------------------------------------------------------
// Type interne : réponse brute du Wallet Service (Go)
// ---------------------------------------------------------------------------

/** Format brut retourné par GET /wallet/{workspaceId} (snake_case Go, src/api/wallet_get.go). */
interface RawWalletBalance {
  workspace_id: string;
  balance: number;        // bigint centimes, colonne réelle "balance"
  balance_cents: number;  // alias == balance (voir wallet_get.go)
  currency: string;
  updated_at: string;     // RFC3339 — non repris dans WalletBalanceDTO (pas dans le port)
}

/** Format brut retourné par GET /wallet/{workspaceId}/transactions (snake_case Go). */
interface RawTransaction {
  id: number;
  workspace_id: string;
  kind: "debit" | "credit" | "refund";
  amount: number;              // centimes
  message_id: string | null;   // nullable côté Go (uuid nullable) — jamais absent, peut être null
  created_at: string;          // RFC3339
}

// ---------------------------------------------------------------------------
// Helpers de mapping et de pagination (BFF-side)
// ---------------------------------------------------------------------------

/** Mappe une balance brute (snake_case Go) vers le DTO TypeScript. */
function mapRawBalance(raw: RawWalletBalance): WalletBalanceDTO {
  return {
    workspaceId: raw.workspace_id,
    balance: raw.balance,
    currency: raw.currency,
  };
}

/**
 * Mappe une transaction brute (snake_case Go) vers le DTO TypeScript.
 * @param raw      - Objet brut retourné par le Wallet Service.
 * @param currency - Devise du workspace (obtenue via getBalance).
 */
function mapRawTransaction(raw: RawTransaction, currency: string): TransactionDTO {
  const description = raw.message_id
    ? `${raw.kind} — message:${raw.message_id}`
    : raw.kind;

  return {
    id: String(raw.id),
    workspaceId: raw.workspace_id,
    type: raw.kind,
    amount: raw.amount,
    currency,
    description,
    createdAt: raw.created_at,
  };
}

/**
 * Décode un curseur opaque (base64 d'un index décimal) en entier.
 * Retourne 0 si le curseur est absent ou invalide.
 */
function decodeCursor(cursor?: string): number {
  if (!cursor) return 0;
  try {
    return parseInt(Buffer.from(cursor, "base64").toString("utf8"), 10);
  } catch {
    return 0;
  }
}

/** Encode un index en curseur opaque (base64 d'un entier décimal). */
function encodeCursor(index: number): string {
  return Buffer.from(String(index), "utf8").toString("base64");
}

/**
 * Applique une pagination cursor-based sur un tableau en mémoire.
 * Le curseur encode l'index de départ dans la liste complète.
 */
function paginateSlice<T>(items: T[], page: PageArgs): Page<T> {
  const limit = page.limit ?? 20;
  const start = decodeCursor(page.cursor);
  const slice = items.slice(start, start + limit);
  const hasNextPage = start + limit < items.length;
  return {
    items: slice,
    pageInfo: {
      nextCursor: hasNextPage ? encodeCursor(start + limit) : null,
      hasNextPage,
    },
  };
}

// ---------------------------------------------------------------------------
// Helper interne : appel HTTP
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
    // TODO(production): GET {baseUrl}/wallet/{workspaceId} (src/api/wallet_get.go, M-025 :
    // route agrégée sous /wallet/... singulier, pas /wallets/.../balance)
    const result = await request<RawWalletBalance>(
      this.baseUrl,
      "GET",
      `/wallet/${workspaceId}`,
      ctx,
    );
    // Valeur par défaut si le service est indisponible (stub offline)
    return result !== null ? mapRawBalance(result) : { workspaceId, balance: 0, currency: "XOF" };
  }

  async listTransactions(
    ctx: ApiContext,
    workspaceId: string,
    page: PageArgs,
  ): Promise<Page<TransactionDTO>> {
    // TODO(production): GET {baseUrl}/wallet/{workspaceId}/transactions (M-025 : route
    //   agrégée sous /wallet/... singulier)
    //   → tableau JSON brut RawTransaction[] (path param, pas de query params vers le backend)
    //
    // Production flow (à activer) :
    // const rawResult = await request<RawTransaction[]>(
    //   this.baseUrl, "GET", `/wallet/${workspaceId}/transactions`, ctx
    // );
    // if (rawResult !== null) {
    //   const balance = await this.getBalance(ctx, workspaceId);   // → currency
    //   const mapped = rawResult.map(r => mapRawTransaction(r, balance.currency));
    //   return paginateSlice(mapped, page);
    // }

    const rawResult = await request<RawTransaction[]>(
      this.baseUrl,
      "GET",
      `/wallet/${workspaceId}/transactions`,
      ctx,
    );

    if (rawResult !== null) {
      // Récupère la devise via le wallet (non incluse dans la réponse de transaction)
      const balance = await this.getBalance(ctx, workspaceId);
      const mapped = rawResult.map((r) => mapRawTransaction(r, balance.currency));
      return paginateSlice(mapped, page);
    }

    // Stub offline — page vide
    void page;
    return { items: [], pageInfo: { nextCursor: null, hasNextPage: false } };
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: WalletClient = new WalletRestClient("http://localhost:8080");
void _conformance;
