// Couche 2 — Use case. Ne dépend que des ports (couche 2) et du domaine (couche 1).

import { ApiKeyRepository } from "../ports/output/repositories.js";
import { ApiKeyNotFoundError } from "../../domain/errors.js";

export interface RevokeApiKeyInput {
  apiKeyId: string;
}

/**
 * Cas d'usage : révoquer une clé API par son identifiant.
 *
 * Flux :
 *  1. findById → ApiKeyNotFoundError si absente
 *  2. apiKey.revoke() — met active=false, revokedAt=now
 *  3. Persistance via markRevoked
 */
export class RevokeApiKey {
  constructor(private readonly repo: ApiKeyRepository) {}

  async execute(input: RevokeApiKeyInput): Promise<void> {
    const apiKey = await this.repo.findById(input.apiKeyId);

    if (apiKey === null) {
      throw new ApiKeyNotFoundError(input.apiKeyId);
    }

    const now = new Date();
    apiKey.revoke(now);

    await this.repo.markRevoked(apiKey.id, now);
  }
}
