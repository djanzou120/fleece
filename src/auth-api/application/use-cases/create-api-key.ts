// Couche 2 — Use case. Ne dépend que des ports (couche 2) et du domaine (couche 1).
// Aucun import npm externe — uniquement node:crypto (stdlib).

import { createHash, randomBytes, randomUUID } from "node:crypto";
import { ApiKey } from "../../domain/api-key.js";
import { ApiKeyRepository } from "../ports/output/repositories.js";

export interface CreateApiKeyInput {
  workspaceId: string;
  name: string;
  permissions?: string[];
  expiresAt?: Date | null;
}

export interface CreateApiKeyOutput {
  /** La clé brute en clair — renvoyée UNE SEULE FOIS à l'appelant, jamais stockée. */
  rawKey: string;
  /** L'entité ApiKey persistée (sans la clé brute). */
  apiKey: ApiKey;
}

/**
 * Génère un hash SHA-256 d'une clé brute (hex).
 * Utilise uniquement node:crypto (stdlib).
 */
function hashKey(rawKey: string): string {
  return createHash("sha256").update(rawKey).digest("hex");
}

/**
 * Cas d'usage : créer une nouvelle clé API pour un workspace.
 *
 * Format de la clé : `fk_<prefix8>_<random32hex>`
 * Seul le hash SHA-256 de la clé est persisté (hashed_key en base).
 * La clé brute est retournée une unique fois dans la réponse — elle ne peut pas être récupérée.
 */
export class CreateApiKey {
  constructor(private readonly repo: ApiKeyRepository) {}

  async execute(input: CreateApiKeyInput): Promise<CreateApiKeyOutput> {
    // Préfixe : 4 octets aléatoires en hex (8 caractères)
    const prefixBytes = randomBytes(4).toString("hex");
    const prefix = `fk_${prefixBytes}`;

    // Corps de la clé : 16 octets aléatoires en hex (32 caractères)
    const body = randomBytes(16).toString("hex");

    // Clé brute complète — renvoyée à l'appelant une seule fois
    const rawKey = `${prefix}_${body}`;

    // Seul le hash est stocké
    const keyHash = hashKey(rawKey);

    const id = randomUUID();
    const now = new Date();

    const apiKey = new ApiKey(
      id,
      input.workspaceId,
      keyHash,
      prefix,
      input.name,
      input.permissions ?? [],
      true, // active
      null, // lastUsedAt
      input.expiresAt ?? null,
      now,
      null, // revokedAt
    );

    await this.repo.save(apiKey);

    return { rawKey, apiKey };
  }
}
