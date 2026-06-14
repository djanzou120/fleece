// Couche 1 — Domain (pur, aucun import externe). Voir .ia/ARCHITECTURE.md §4.1.

/**
 * Entité Workspace : espace isolé d'une entreprise cliente.
 *
 * Champs riches tels que définis par la spec T-006.
 * Tous les champs sont désormais persistés :
 *   - 0001_identity.sql : id, name, country, created_at
 *   - 0011_identity_schema.sql : slug, owner_id, plan
 */
export class Workspace {
  constructor(
    public readonly id: string,
    public readonly name: string,
    /** Pays du workspace — détermine le moyen de paiement (Mobile Money / Stripe). Persisté (0001). */
    public readonly country: string,
    /** Identifiant de l'utilisateur propriétaire du workspace. Persisté (0011 — colonne owner_id). */
    public readonly ownerId: string,
    /** Slug URL-friendly unique. Persisté (0011 — colonne slug, UNIQUE INDEX). */
    public readonly slug: string,
    /** Plan tarifaire (ex. "free", "pro", "enterprise"). Persisté (0011 — colonne plan). */
    public readonly plan: string,
    public readonly createdAt: Date,
  ) {}
}
