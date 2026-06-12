// Couche 1 — Domain (pur, aucun framework). Voir .ia/ARCHITECTURE.md.

export type ApiKeyStatus = "active" | "revoked";

/** Entité ApiKey. La clé en clair n'est jamais stockée, seulement son hash. */
export class ApiKey {
  constructor(
    public readonly id: string,
    public readonly workspaceId: string,
    public readonly hashedKey: string,
    public status: ApiKeyStatus = "active",
  ) {}

  revoke(): void {
    this.status = "revoked";
  }

  isUsable(): boolean {
    return this.status === "active";
  }
}
