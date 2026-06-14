// Couche 1 — Domain (pur, aucun import externe). Voir .ia/ARCHITECTURE.md §4.1.

/**
 * Entité ApiKey : clé d'API appartenant à un workspace.
 * La clé en clair n'est JAMAIS stockée, uniquement son hash SHA-256 (keyHash).
 *
 * Tous les champs sont désormais persistés :
 *   - 0001_identity.sql : id, workspace_id, hashed_key, status, created_at, revoked_at
 *   - 0011_identity_schema.sql : name, prefix, permissions, last_used_at, expires_at
 *   - active (bool) : dérivé de status ('active'/'revoked') en base.
 */
export class ApiKey {
  constructor(
    public readonly id: string,
    public readonly workspaceId: string,
    /** Hash SHA-256 de la clé brute. Persisté (0001 — colonne hashed_key). */
    public readonly keyHash: string,
    /** Préfixe lisible de la clé (ex. "fk_abc123"). Persisté (0011 — colonne prefix VARCHAR(20)). */
    public readonly prefix: string,
    /** Nom descriptif de la clé (ex. "Production key"). Persisté (0011 — colonne name VARCHAR(255)). */
    public readonly name: string,
    /** Permissions accordées à cette clé (ex. ["messages:send"]). Persisté (0011 — colonne permissions TEXT[]). */
    public readonly permissions: string[],
    /** Si false, la clé est révoquée. Mappé sur status='active'|'revoked' (0001). */
    public active: boolean,
    /** Dernière utilisation. Persisté (0011 — colonne last_used_at TIMESTAMPTZ nullable). */
    public lastUsedAt: Date | null,
    /** Date d'expiration optionnelle. Persisté (0011 — colonne expires_at TIMESTAMPTZ nullable). */
    public readonly expiresAt: Date | null,
    public readonly createdAt: Date,
    /** Date de révocation. Persistée (0001 — colonne revoked_at). */
    public revokedAt: Date | null = null,
  ) {}

  /**
   * Indique si la clé a dépassé sa date d'expiration.
   * @param now — point de comparaison (par défaut : maintenant). Injecté pour la testabilité.
   */
  isExpired(now: Date = new Date()): boolean {
    if (this.expiresAt === null) return false;
    return now > this.expiresAt;
  }

  /**
   * Indique si la clé est utilisable : active ET non expirée.
   * @param now — point de comparaison (par défaut : maintenant).
   */
  isActive(now: Date = new Date()): boolean {
    return this.active && !this.isExpired(now);
  }

  /** Révoque la clé (idempotent). */
  revoke(at: Date = new Date()): void {
    this.active = false;
    this.revokedAt = at;
  }

  /**
   * @deprecated Préfère isActive(). Conservé pour la compatibilité avec les anciens usages.
   */
  isUsable(): boolean {
    return this.isActive();
  }
}
