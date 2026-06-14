// Couche 2 — Use case. Ne dépend que des ports (couche 2) et du domaine (couche 1).
// Importe ApiContext depuis @fleece/api-common (couche 0 transverse — autorisé).

import { createHash } from "node:crypto";
import { ApiContext } from "@fleece/api-common";
import { ApiKeyRepository } from "../ports/output/repositories.js";
import { InvalidApiKeyError, ApiKeyRevokedError } from "../../domain/errors.js";

/**
 * Cas d'usage : valider une clé API brute reçue d'un client.
 *
 * Flux :
 *  1. Hash SHA-256 de la clé brute
 *  2. Recherche par hash dans le repository
 *  3. Vérification isActive() (inactive → ApiKeyRevokedError, introuvable → InvalidApiKeyError)
 *  4. Mise à jour de lastUsedAt
 *  5. Retourne ApiContext (workspaceId, apiKeyId, authMethod: "api_key")
 */
export class ValidateApiKey {
  constructor(private readonly repo: ApiKeyRepository) {}

  async execute(rawKey: string): Promise<ApiContext> {
    const hash = createHash("sha256").update(rawKey).digest("hex");

    const apiKey = await this.repo.findByHash(hash);

    if (apiKey === null) {
      throw new InvalidApiKeyError("Clé API non reconnue");
    }

    const now = new Date();

    if (!apiKey.active) {
      throw new ApiKeyRevokedError(apiKey.id);
    }

    if (apiKey.isExpired(now)) {
      throw new InvalidApiKeyError(`Clé API expirée : ${apiKey.id}`);
    }

    // Mise à jour de lastUsedAt (best-effort)
    await this.repo.updateLastUsed(apiKey.id, now);

    const context: ApiContext = {
      workspaceId: apiKey.workspaceId,
      apiKeyId: apiKey.id,
      authMethod: "api_key",
    };

    return context;
  }
}
