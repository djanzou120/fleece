// Tests unitaires — Couche 1 (Domain). Purs, sans mock, sans framework externe.
// Testent la logique métier de l'entité ApiKey.

import { ApiKey } from "../api-key.js";

function makeApiKey(overrides: Partial<{
  id: string;
  workspaceId: string;
  keyHash: string;
  prefix: string;
  name: string;
  permissions: string[];
  active: boolean;
  lastUsedAt: Date | null;
  expiresAt: Date | null;
  createdAt: Date;
  revokedAt: Date | null;
}> = {}): ApiKey {
  return new ApiKey(
    overrides.id ?? "key-1",
    overrides.workspaceId ?? "ws-1",
    overrides.keyHash ?? "abc123hash",
    overrides.prefix ?? "fk_aabbccdd",
    overrides.name ?? "Test Key",
    overrides.permissions ?? [],
    overrides.active ?? true,
    overrides.lastUsedAt ?? null,
    overrides.expiresAt ?? null,
    overrides.createdAt ?? new Date("2026-01-01"),
    overrides.revokedAt ?? null,
  );
}

describe("ApiKey", () => {
  describe("isExpired()", () => {
    it("retourne false si expiresAt est null", () => {
      const key = makeApiKey({ expiresAt: null });
      expect(key.isExpired()).toBe(false);
    });

    it("retourne false si la date d'expiration est dans le futur", () => {
      const future = new Date(Date.now() + 60_000);
      const key = makeApiKey({ expiresAt: future });
      expect(key.isExpired()).toBe(false);
    });

    it("retourne true si la date d'expiration est dans le passé", () => {
      const past = new Date(Date.now() - 60_000);
      const key = makeApiKey({ expiresAt: past });
      expect(key.isExpired()).toBe(true);
    });

    it("accepte un paramètre `now` pour rendre le test déterministe", () => {
      const expiresAt = new Date("2026-06-01T12:00:00Z");
      const key = makeApiKey({ expiresAt });
      // Avant expiration
      expect(key.isExpired(new Date("2026-05-31T00:00:00Z"))).toBe(false);
      // Après expiration
      expect(key.isExpired(new Date("2026-06-02T00:00:00Z"))).toBe(true);
    });
  });

  describe("isActive()", () => {
    it("retourne true si active=true et non expirée", () => {
      const key = makeApiKey({ active: true, expiresAt: null });
      expect(key.isActive()).toBe(true);
    });

    it("retourne false si active=false", () => {
      const key = makeApiKey({ active: false, expiresAt: null });
      expect(key.isActive()).toBe(false);
    });

    it("retourne false si active=true mais expirée", () => {
      const past = new Date(Date.now() - 60_000);
      const key = makeApiKey({ active: true, expiresAt: past });
      expect(key.isActive()).toBe(false);
    });
  });

  describe("revoke()", () => {
    it("passe active à false", () => {
      const key = makeApiKey({ active: true });
      key.revoke();
      expect(key.active).toBe(false);
    });

    it("enregistre la date de révocation", () => {
      const at = new Date("2026-06-13T10:00:00Z");
      const key = makeApiKey({ active: true });
      key.revoke(at);
      expect(key.revokedAt).toEqual(at);
    });

    it("est idempotent : appeler revoke() deux fois ne lève pas d'erreur", () => {
      const key = makeApiKey({ active: true });
      key.revoke();
      expect(() => key.revoke()).not.toThrow();
      expect(key.active).toBe(false);
    });
  });
});
