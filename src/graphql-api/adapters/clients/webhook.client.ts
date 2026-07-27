// Couche 3 — Client REST driven : implémentation de WebhookClient vers le service HTTP
// unifié `src/api` (Go — routes /webhook-endpoints/*, cf. src/api/service.go registerRoutes).
// Utilise fetch global (Node 18+) — pas de package npm.
// Convention stubs : retour valeurs vides / [] — voir identity.client.ts pour détails.
//
// Dette D-M40 SOLDÉE (routes portées côté src/api) : ce client gère les endpoints webhook
// SORTANTS configurés par le dashboard (CRUD "webhook-endpoints" : url + events abonnés côté
// workspace, cf. WebhookClient / ManageWebhook / manage-webhook.ts). C'est un concept
// DIFFÉRENT des webhooks ENTRANTS (callbacks providers OM/MTN/Telegram) exposés par src/api
// sous /webhooks/om|mtn|telegram (src/api/webhooks_om.go, webhooks_mtn.go,
// webhooks_telegram.go).
//
// Contrat backend (src/api/webhook_endpoints_create.go, _list.go, _delete.go — vérifié) :
//   - POST /webhook-endpoints
//     Corps JSON : { workspace_id: uuid, url: string, secret?: string, events: string[] }.
//     `secret` est optionnel — généré côté Go (crypto/rand, 32 octets hex) si absent.
//     400 si workspace_id manquant/invalide, url vide, ou events vide.
//     201 + { id, workspace_id, url, secret, events, active, created_at } — le SECRET EN
//     CLAIR n'est présent QUE dans cette réponse (jamais renvoyé ensuite, même pattern que
//     les clés API : voir createApiKey / rotateApiKey côté IdentityClient).
//   - GET /webhook-endpoints?workspace_id=<uuid>
//     workspace_id est un paramètre requis (400 si absent/invalide). Filtre les endpoints
//     actifs uniquement (active = true). 200 + tableau JSON brut (jamais null côté Go).
//     Cette réponse NE CONTIENT JAMAIS le champ `secret` — absent de la ligne SQL sélectionnée
//     (webhookEndpointListRow côté Go n'a pas de colonne Secret).
//   - DELETE /webhook-endpoints/{id}?workspace_id=<uuid>
//     Suppression LOGIQUE (UPDATE active = false), pas physique (FK depuis
//     webhook.webhook_deliveries). Scopée par workspace_id. 204 si désactivé, 404 si
//     id inconnu / déjà inactif / n'appartenant pas à ce workspace (même réponse dans les
//     trois cas, pour ne pas fuiter l'existence d'un endpoint d'un autre workspace).
//
// Nommage des paramètres (snake_case, cohérent avec messages_list.go / campaigns_list.go) :
// le paramètre de scoping est `workspace_id`, aussi bien en query string (list/delete) qu'en
// corps JSON (create) — jamais `workspaceId`. Le champ `secret` de WebhookEndpointDTO est
// donc optionnel/absent en pratique pour tout ce qui provient de listEndpoints (voir
// mapRawListItem ci-dessous, qui ne le peuple jamais) et n'est renseigné que par
// createEndpoint (mapRawCreated).

import type { ApiContext } from "@fleece/api-common";
import type {
  WebhookClient,
  WebhookEndpointDTO,
} from "../../application/ports/output/clients.js";

// ---------------------------------------------------------------------------
// Types internes : réponses brutes du service Go unifié (snake_case)
// ---------------------------------------------------------------------------

/**
 * Ligne brute retournée par GET /webhook-endpoints (webhookEndpointListRow côté Go) —
 * volontairement SANS champ `secret` (jamais renvoyé en dehors de la création).
 */
interface RawWebhookEndpointListItem {
  id: string;
  workspace_id: string;
  url: string;
  events: string[];
  active: boolean;
  created_at: string; // RFC3339
}

/**
 * Réponse brute de POST /webhook-endpoints (createWebhookEndpointResponse côté Go) —
 * contient le secret en clair, uniquement à cet instant.
 */
interface RawWebhookEndpointCreated extends RawWebhookEndpointListItem {
  secret: string;
}

/** Mappe une ligne de liste brute (snake_case Go, sans secret) vers le DTO TypeScript. */
function mapRawListItem(raw: RawWebhookEndpointListItem): WebhookEndpointDTO {
  return {
    id: raw.id,
    workspaceId: raw.workspace_id,
    url: raw.url,
    events: raw.events,
    active: raw.active,
    createdAt: raw.created_at,
    // Pas de `secret` ici : la réponse de liste ne le contient jamais côté Go.
  };
}

/** Mappe la réponse de création brute (snake_case Go, avec secret) vers le DTO TypeScript. */
function mapRawCreated(raw: RawWebhookEndpointCreated): WebhookEndpointDTO {
  return {
    ...mapRawListItem(raw),
    secret: raw.secret,
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

export class WebhookRestClient implements WebhookClient {
  constructor(private readonly baseUrl: string) {}

  async listEndpoints(ctx: ApiContext, workspaceId: string): Promise<WebhookEndpointDTO[]> {
    // GET {baseUrl}/webhook-endpoints?workspace_id=...
    const result = await request<RawWebhookEndpointListItem[]>(
      this.baseUrl,
      "GET",
      `/webhook-endpoints?workspace_id=${encodeURIComponent(workspaceId)}`,
      ctx,
    );
    return result !== null ? result.map(mapRawListItem) : [];
  }

  async createEndpoint(
    ctx: ApiContext,
    workspaceId: string,
    payload: { url: string; events: string[] },
  ): Promise<WebhookEndpointDTO> {
    // POST {baseUrl}/webhook-endpoints { workspace_id, url, events }
    // `secret` n'est jamais envoyé par le BFF : le backend en génère un si absent.
    const result = await request<RawWebhookEndpointCreated>(
      this.baseUrl,
      "POST",
      "/webhook-endpoints",
      ctx,
      { workspace_id: workspaceId, ...payload },
    );
    if (result !== null) {
      return mapRawCreated(result);
    }
    // Stub offline : retourne un endpoint fictif pour permettre le développement du frontend.
    // Pas de secret fictif ici (aucun secret n'a réellement été généré).
    return {
      id: "stub-endpoint-id",
      workspaceId,
      url: payload.url,
      events: payload.events,
      active: true,
      createdAt: new Date().toISOString(),
    };
  }

  async deleteEndpoint(ctx: ApiContext, workspaceId: string, id: string): Promise<void> {
    // DELETE {baseUrl}/webhook-endpoints/{id}?workspace_id=...
    await request<void>(
      this.baseUrl,
      "DELETE",
      `/webhook-endpoints/${id}?workspace_id=${encodeURIComponent(workspaceId)}`,
      ctx,
    );
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: WebhookClient = new WebhookRestClient("http://localhost:8080");
void _conformance;
