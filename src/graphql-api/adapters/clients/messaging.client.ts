// Couche 3 — Client REST driven : implémentation de MessagingClient vers le service HTTP
// unifié `src/api` (Go — routes /messages, cf. src/api/service.go registerRoutes / M-025).
// Utilise fetch global (Node 18+) — pas de package npm.
// Convention stubs : retour valeurs vides — voir identity.client.ts pour détails.
//
// Contrat backend (src/api/messages_list.go, messages_get.go — vérifié M-025) :
//   - GET /messages?workspace_id=&status=&limit=&offset=  → tableau JSON brut RawMessage[],
//     PAS un objet Page<MessageDTO> ; le backend ne connaît QUE la pagination limit/offset
//     (bornée à 200 par requête, src/api/messages_list.go `maxLimit`), aucune notion de
//     curseur opaque côté Go. Le nom du paramètre est `workspace_id` (snake_case), pas
//     `workspaceId` — écart corrigé M-025 (l'ancien appel envoyait le mauvais nom, silencieusement
//     ignoré par un service Go qui n'existe plus de toute façon).
//     `workspace_id` est désormais REQUIS (400 sans lui) : l'appeler sans ce paramètre
//     retournait auparavant les messages de TOUS les workspaces confondus (D-M41).
//   - GET /messages/{id}?workspace_id=  → objet RawMessage unique, 404 si absent.
//     `workspace_id` est REQUIS et le handler FILTRE dessus (correctif D-M41).
//     Auparavant la requête était `WHERE id = $1` seul : connaître un UUID de message
//     suffisait à lire celui d'un autre workspace, et ce client recevait donc un
//     `workspaceId` qu'il ignorait délibérément. Un message d'un autre workspace renvoie
//     404, indistinguable d'un message inexistant (404 uniforme, anti-énumération).
//   - messageRow (Go) n'a PAS de colonne `updated_at` (migration 0003) : MessageDTO.updatedAt
//     est donc approximé par `created_at` ci-dessous (divergence de contrat signalée, pas
//     de solution "sûre" possible sans migration côté Go — D-M25-2).
//
// La pagination cursor-based exposée par le port MessagingClient est donc appliquée
// CÔTÉ BFF sur la liste complète retournée par une requête bornée à `maxLimit` (200) côté
// backend — même pattern que WalletRestClient.listTransactions. Au-delà de 200 messages
// pour un workspace, le curseur BFF ne peut pas voir les éléments suivants (limite connue,
// D-M25-3 — à traiter si nécessaire par une vraie pagination serveur côté src/api).

import type { ApiContext, Page, PageArgs } from "@fleece/api-common";
import type {
  MessagingClient,
  MessageDTO,
} from "../../application/ports/output/clients.js";

// ---------------------------------------------------------------------------
// Type interne : réponse brute du service Go unifié
// ---------------------------------------------------------------------------

/** Format brut d'un message retourné par src/api (snake_case Go, messages_list.go/get.go). */
interface RawMessage {
  id: string;
  workspace_id: string;
  recipient: string;
  content: string;
  status: string;
  channel: string;
  created_at: string; // RFC3339
  // NB : pas de "updated_at" côté backend (voir commentaire d'en-tête, D-M25-2).
}

/** Mappe un message brut (snake_case Go) vers le DTO TypeScript. */
function mapRawMessage(raw: RawMessage): MessageDTO {
  return {
    id: raw.id,
    workspaceId: raw.workspace_id,
    to: raw.recipient,
    channel: raw.channel,
    content: raw.content,
    status: raw.status,
    createdAt: raw.created_at,
    // Approximation documentée (D-M25-2) : messaging.messages n'a pas de colonne updated_at.
    updatedAt: raw.created_at,
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
 * Le curseur encode l'index de départ dans la liste complète (bornée à maxLimit
 * côté backend — voir commentaire d'en-tête, D-M25-3).
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
    // TODO(production): GET {baseUrl}/messages?workspace_id=...&limit=...
    //   src/api ne connaît que limit/offset (pas de curseur, borné à 200) : on demande le
    //   maximum autorisé côté backend et on pagine par curseur CÔTÉ BFF sur le résultat,
    //   même pattern que WalletRestClient.listTransactions (voir en-tête, D-M25-3).
    const params = new URLSearchParams({
      workspace_id: workspaceId,
      limit: "200",
    });

    const rawResult = await request<RawMessage[]>(
      this.baseUrl,
      "GET",
      `/messages?${params.toString()}`,
      ctx,
    );

    if (rawResult !== null) {
      const mapped = rawResult.map(mapRawMessage);
      return paginateSlice(mapped, page);
    }

    // Page vide si le service est indisponible (stub offline)
    return { items: [], pageInfo: { nextCursor: null, hasNextPage: false } };
  }

  async getMessage(
    ctx: ApiContext,
    workspaceId: string,
    id: string,
  ): Promise<MessageDTO | null> {
    // TODO(production): GET {baseUrl}/messages/{id}?workspace_id=...
    //
    // CORRECTIF D-M41 : `workspaceId` est désormais TRANSMIS, et le backend
    // l'EXIGE (400 sans lui). Il était auparavant reçu puis délibérément ignoré
    // (`_workspaceId`) parce que src/api/messages_get.go ne filtrait pas par
    // workspace — connaître un UUID de message suffisait alors à lire celui
    // d'un autre workspace. Le filtre existe maintenant côté serveur : ne pas
    // revenir à un appel sans ce paramètre, il échouerait en 400.
    //
    // Un message appartenant à un autre workspace renvoie 404, exactement comme
    // un message inexistant (404 uniforme, pour ne pas fuiter l'existence de
    // l'identifiant) — donc `null` ici dans les deux cas, sans distinction
    // possible côté BFF. C'est voulu.
    const params = new URLSearchParams({ workspace_id: workspaceId });
    const raw = await request<RawMessage>(
      this.baseUrl,
      "GET",
      `/messages/${id}?${params.toString()}`,
      ctx,
    );
    return raw !== null ? mapRawMessage(raw) : null;
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: MessagingClient = new MessagingRestClient(
  "http://localhost:8080",
);
void _conformance;
