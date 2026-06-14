// Tests unitaires — Couche 2 (Application). Use case CreateApiKey.
// Utilise des mocks manuels des ports (pas de framework mock requis).

import { CreateApiKey } from "../use-cases/create-api-key.js";
import { ApiKey } from "../../domain/api-key.js";
import { ApiKeyRepository } from "../ports/output/repositories.js";

// ---------------------------------------------------------------------------
// Mock du port ApiKeyRepository
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

  async updateLastUsed(_id: string, _at: Date): Promise<void> {
    // no-op pour les tests
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

describe("CreateApiKey", () => {
  let repo: InMemoryApiKeyRepository;
  let useCase: CreateApiKey;

  beforeEach(() => {
    repo = new InMemoryApiKeyRepository();
    useCase = new CreateApiKey(repo);
  });

  it("retourne une rawKey et une entité ApiKey", async () => {
    const result = await useCase.execute({
      workspaceId: "ws-1",
      name: "Ma clé de test",
    });

    expect(result.rawKey).toBeTruthy();
    expect(result.apiKey).toBeTruthy();
    expect(result.apiKey.workspaceId).toBe("ws-1");
    expect(result.apiKey.name).toBe("Ma clé de test");
  });

  it("génère une rawKey au format fk_<prefix>_<corps>", async () => {
    const result = await useCase.execute({
      workspaceId: "ws-1",
      name: "Test",
    });

    // Format attendu : fk_<8 hex chars>_<32 hex chars>
    expect(result.rawKey).toMatch(/^fk_[0-9a-f]{8}_[0-9a-f]{32}$/);
  });

  it("persiste la clé dans le repository", async () => {
    const result = await useCase.execute({
      workspaceId: "ws-1",
      name: "Test",
    });

    const saved = await repo.findById(result.apiKey.id);
    expect(saved).not.toBeNull();
    expect(saved?.id).toBe(result.apiKey.id);
  });

  it("ne persiste PAS la clé brute — seulement le hash", async () => {
    const result = await useCase.execute({
      workspaceId: "ws-1",
      name: "Test",
    });

    const saved = await repo.findById(result.apiKey.id);
    // Le keyHash stocké ne doit pas être égal à la clé brute
    expect(saved?.keyHash).not.toBe(result.rawKey);
    // Le keyHash doit être un hash SHA-256 (64 chars hex)
    expect(saved?.keyHash).toMatch(/^[0-9a-f]{64}$/);
  });

  it("génère des clés uniques à chaque exécution", async () => {
    const r1 = await useCase.execute({ workspaceId: "ws-1", name: "Clé 1" });
    const r2 = await useCase.execute({ workspaceId: "ws-1", name: "Clé 2" });

    expect(r1.rawKey).not.toBe(r2.rawKey);
    expect(r1.apiKey.id).not.toBe(r2.apiKey.id);
    expect(r1.apiKey.keyHash).not.toBe(r2.apiKey.keyHash);
  });

  it("crée la clé comme active par défaut", async () => {
    const result = await useCase.execute({ workspaceId: "ws-1", name: "Test" });
    expect(result.apiKey.active).toBe(true);
    expect(result.apiKey.isActive()).toBe(true);
  });

  it("respecte les permissions passées en entrée", async () => {
    const result = await useCase.execute({
      workspaceId: "ws-1",
      name: "Test",
      permissions: ["messages:send", "wallet:read"],
    });
    expect(result.apiKey.permissions).toEqual(["messages:send", "wallet:read"]);
  });

  it("utilise un tableau vide si aucune permission n'est passée", async () => {
    const result = await useCase.execute({ workspaceId: "ws-1", name: "Test" });
    expect(result.apiKey.permissions).toEqual([]);
  });
});
