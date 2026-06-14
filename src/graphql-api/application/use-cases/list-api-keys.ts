// Couche 2 — Use case : liste les clés API d'un workspace.
// Dépend uniquement des ports de sortie et de @fleece/api-common.
// Délègue entièrement à IdentityClient — pas de logique métier propre.

import type { ApiContext } from "@fleece/api-common";
import type { IdentityClient, ApiKeyDTO } from "../ports/output/clients.js";

/**
 * Liste les clés API actives et révoquées d'un workspace.
 * Le filtrage éventuel (active seulement) est laissé à la couche présentation.
 */
export class ListApiKeys {
  constructor(private readonly identityClient: IdentityClient) {}

  async execute(ctx: ApiContext, workspaceId: string): Promise<ApiKeyDTO[]> {
    return this.identityClient.listApiKeys(ctx, workspaceId);
  }
}
