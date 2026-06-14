// Tests unitaires — Couche 2 (Application). Use case ValidateApiKey.
// Utilise des mocks manuels des ports.

import { createHash } from "node:crypto";
import { ValidateApiKey } from "../use-cases/validate-api-key.js";
import { CreateApiKey } from "../use-cases/create-api-key.js";
import { ApiKey } from "../../domain/api-key.js";
import { ApiKeyRepository } from "../ports/output/repositories.js";
import { InvalidApiKeyError, ApiKeyRevokedError } from "../../domain/errors.js";

// ---------------------------------------------------------------------------
// Mock du port ApiKeyRepository (en mémoire)
// ---------------------------------------------------------------------------

class InMemoryApiKeyRepository implements ApiKeyRepository {
  private store = new Map<string, ApiKey>();

  async save(key: ApiKey): Promise<void> {
    this.store.set(key.id, key);
  }

  async findById(id: string): Promise<ApiKey | null> {
    return this.store.get(id) ?? null;
  }

  async findByHash(hash: string): Promise<ApiKey | null> {
    for (const key of this.store.values()) {
      if (key.keyHash === hash) return key;
    }
    return null;
  }

  async updateLastUsed(id: string, at: Date): Promise<void> {
    const key = this.store.get(id);
    if (key) {
      key.lastUsedAt = at;
    }
  }

  async markRevoked(id: string, at: Date): Promise<void> {
    const key = this.store.get(id);
    if (key) {
      key.revoke(at);
    }
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("ValidateApiKey", () => {
  let repo: InMemoryApiKeyRepository;
  let createUseCase: CreateApiKey;
  let validateUseCase: ValidateApiKey;

  beforeEach(() => {
    repo = new InMemoryApiKeyRepository();
    createUseCase = new CreateApiKey(repo);
    validateUseCase = new ValidateApiKey(repo);
  });

  it("retourne un ApiContext valide pour une clé active", async () => {
    const created = await createUseCase.execute({
      workspaceId: "ws-42",
      name: "Clé test",
    });

    const context = await validateUseCase.execute(created.rawKey);

    expect(context.workspaceId).toBe("ws-42");
    expect(context.apiKeyId).toBe(created.apiKey.id);
    expect(context.authMethod).toBe("api_key");
  });

  it("lève InvalidApiKeyError si la clé n'existe pas", async () => {
    await expect(validateUseCase.execute("fk_00000000_notavalidkey00000000000000"))
      .rejects
      .toThrow(InvalidApiKeyError);
  });

  it("lève ApiKeyRevokedError si la clé est révoquée", async () => {
    const created = await createUseCase.execute({ workspaceId: "ws-1", name: "Test" });

    // Révocation directe dans le repo
    await repo.markRevoked(created.apiKey.id, new Date());

    await expect(validateUseCase.execute(created.rawKey))
      .rejects
      .toThrow(ApiKeyRevokedError);
  });

  it("lève InvalidApiKeyError si la clé est expirée", async () => {
    const past = new Date(Date.now() - 60_000);
    const key = new ApiKey(
      "key-exp",
      "ws-1",
      createHash("sha256").update("fk_aabb1234_expiredkey000000000000000000").digest("hex"),
      "fk_aabb1234",
      "Expirée",
      [],
      true,
      null,
      past, // expiresAt dans le passé
      new Date("2026-01-01"),
      null,
    );
    await repo.save(key);

    await expect(
      validateUseCase.execute("fk_aabb1234_expiredkey000000000000000000"),
    ).rejects.toThrow(InvalidApiKeyError);
  });

  it("met à jour lastUsedAt après une validation réussie", async () => {
    const created = await createUseCase.execute({ workspaceId: "ws-1", name: "Test" });
    const before = new Date();

    await validateUseCase.execute(created.rawKey);

    const saved = await repo.findById(created.apiKey.id);
    expect(saved?.lastUsedAt).not.toBeNull();
    expect(saved!.lastUsedAt!.getTime()).toBeGreaterThanOrEqual(before.getTime());
  });
});
