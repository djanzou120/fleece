// Couche 1 — Domain (pur, aucun import externe). Voir .ia/ARCHITECTURE.md §4.1.

/**
 * Entité ApiKey : clé d'API appartenant à un workspace.
 * La clé en clair n'est JAMAIS stockée, uniquement son hash SHA-256 (keyHash).
 *
 * Champs riches conformes à la spec T-006.
 * NOTE écarts avec 0001_identity.sql :
 *  - name          : TODO(schema): absent de 0001 — aligner migration avant production.
 *  - prefix        : TODO(schema): absent de 0001 — aligner migration avant production.
 *  - permissions   : TODO(schema): absent de 0001 — aligner migration avant production.
 *  - lastUsedAt    : TODO(schema): absent de 0001 — aligner migration avant production.
 *  - expiresAt     : TODO(schema): absent de 0001 — aligner migration avant production.
 *  - active (bool) : dérivé de status ('active'/'revoked') — persisté via status en 0001.
 *  - revokedAt     : persisté via revoked_at en 0001.
 */
export class ApiKey {
  constructor(
    public readonly id: string,
    public readonly workspaceId: string,
    /** Hash SHA-256 de la clé brute. Seule valeur persistée (hashed_key). */
    public readonly keyHash: string,
    /**
     * Préfixe lisible de la clé (ex. "fk_abc123").
     * TODO(schema): colonne prefix absente de 0001 — aligner migration avant production.
     */
    public readonly prefix: string,
    /**
     * Nom descriptif de la clé (ex. "Production key").
     * TODO(schema): colonne name absente de 0001 — aligner migration avant production.
     */
    public readonly name: string,
    /**
     * Liste de permissions accordées à cette clé (ex. ["messages:send"]).
     * TODO(schema): colonne permissions absente de 0001 — aligner migration avant production.
     */
    public readonly permissions: string[],
    /** Si false, la clé est révoquée. Mappé sur status='active'|'revoked' en base. */
    public active: boolean,
    /**
     * Dernière utilisation de la clé.
     * TODO(schema): colonne last_used_at absente de 0001 — aligner migration avant production.
     */
    public lastUsedAt: Date | null,
    /**
     * Date d'expiration optionnelle.
     * TODO(schema): colonne expires_at absente de 0001 — aligner migration avant production.
     */
    public readonly expiresAt: Date | null,
    public readonly createdAt: Date,
    /** Date de révocation. Persistée via revoked_at en 0001. */
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
