// Couche 2 — Use case. Ne dépend que des ports (couche 2) et du domaine (couche 1).

import { ApiKey } from "../../domain/api-key";
import { ApiKeyRepository } from "../ports/output/repositories";

export class CreateApiKey {
  constructor(private readonly repo: ApiKeyRepository) {}

  async execute(input: { id: string; workspaceId: string; hashedKey: string }): Promise<ApiKey> {
    const key = new ApiKey(input.id, input.workspaceId, input.hashedKey);
    await this.repo.save(key);
    return key;
  }
}
