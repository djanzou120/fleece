// Couche 2 — Use case : gestion des endpoints webhook d'un workspace.
// Dépend uniquement des ports de sortie et de @fleece/api-common.
// Regroupe list/create/delete dans une seule classe par cohésion fonctionnelle.

import type { ApiContext } from "@fleece/api-common";
import type { WebhookClient, WebhookEndpointDTO } from "../ports/output/clients.js";

/**
 * Gère le cycle de vie des endpoints webhook d'un workspace.
 * Délègue à WebhookClient — pas de logique métier propre au BFF.
 */
export class ManageWebhook {
  constructor(private readonly webhookClient: WebhookClient) {}

  /** Liste les endpoints webhook d'un workspace. */
  async list(ctx: ApiContext, workspaceId: string): Promise<WebhookEndpointDTO[]> {
    return this.webhookClient.listEndpoints(ctx, workspaceId);
  }

  /**
   * Crée un nouvel endpoint webhook.
   * @param url    - URL HTTPS de destination
   * @param events - Événements auxquels s'abonner (ex. "message.delivered")
   */
  async create(
    ctx: ApiContext,
    workspaceId: string,
    payload: { url: string; events: string[] },
  ): Promise<WebhookEndpointDTO> {
    return this.webhookClient.createEndpoint(ctx, workspaceId, payload);
  }

  /** Supprime un endpoint webhook. Retourne void — la confirmation est faite côté client. */
  async delete(ctx: ApiContext, workspaceId: string, id: string): Promise<void> {
    return this.webhookClient.deleteEndpoint(ctx, workspaceId, id);
  }
}
