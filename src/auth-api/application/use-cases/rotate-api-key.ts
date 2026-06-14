// Couche 2 — Use case. Ne dépend que des ports (couche 2) et du domaine (couche 1).

import { ApiKeyRepository } from "../ports/output/repositories.js";
import { ApiKeyNotFoundError } from "../../domain/errors.js";
import { CreateApiKey, CreateApiKeyOutput } from "./create-api-key.js";

export interface RotateApiKeyInput {
  /** Identifiant de la clé à remplacer. */
  oldApiKeyId: string;
  /** Nom de la nouvelle clé (optionnel — reprend le nom de l'ancienne si absent). */
  newName?: string;
}

export interface RotateApiKeyOutput {
  /** La nouvelle clé brute en clair — à transmettre une seule fois. */
  rawKey: string;
  /** L'entité de la nouvelle clé API. */
  apiKey: CreateApiKeyOutput["apiKey"];
}

/**
 * Cas d'usage : faire tourner une clé API.
 *
 * Flux :
 *  1. Trouve l'ancienne clé (ApiKeyNotFoundError si absente)
 *  2. Révoque l'ancienne clé via markRevoked
 *  3. Crée une nouvelle clé (réutilise CreateApiKey) avec les mêmes attributs
 *  4. Retourne la nouvelle clé brute en clair
 */
export class RotateApiKey {
  private readonly createApiKey: CreateApiKey;

  constructor(private readonly repo: ApiKeyRepository) {
    this.createApiKey = new CreateApiKey(repo);
  }

  async execute(input: RotateApiKeyInput): Promise<RotateApiKeyOutput> {
    const oldKey = await this.repo.findById(input.oldApiKeyId);

    if (oldKey === null) {
      throw new ApiKeyNotFoundError(input.oldApiKeyId);
    }

    // Révocation de l'ancienne clé
    const now = new Date();
    oldKey.revoke(now);
    await this.repo.markRevoked(oldKey.id, now);

    // Création de la nouvelle clé avec les mêmes métadonnées
    const result = await this.createApiKey.execute({
      workspaceId: oldKey.workspaceId,
      name: input.newName ?? oldKey.name,
      permissions: oldKey.permissions,
      expiresAt: oldKey.expiresAt,
    });

    return {
      rawKey: result.rawKey,
      apiKey: result.apiKey,
    };
  }
}
