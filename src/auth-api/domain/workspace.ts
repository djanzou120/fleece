// Couche 1 — Domain (pur, aucun import externe). Voir .ia/ARCHITECTURE.md §4.1.

/**
 * Entité Workspace : espace isolé d'une entreprise cliente.
 *
 * Champs riches tels que définis par la spec T-006.
 * NOTE : le schéma SQL 0001_identity.sql ne persiste que (id, name, country, created_at).
 * Les champs slug, ownerId, plan ne sont PAS dans la base — voir les TODO(schema) dans les repos.
 * country est conservé ici car persisté.
 */
export class Workspace {
  constructor(
    public readonly id: string,
    public readonly name: string,
    /** Pays du workspace — détermine le moyen de paiement (Mobile Money / Stripe). Persisté. */
    public readonly country: string,
    /**
     * Identifiant de l'utilisateur propriétaire du workspace.
     * TODO(schema): colonne owner_id absente de 0001 — aligner migration avant production.
     */
    public readonly ownerId: string,
    /**
     * Slug URL-friendly dérivé du nom.
     * TODO(schema): colonne slug absente de 0001 — aligner migration avant production.
     */
    public readonly slug: string,
    /**
     * Plan tarifaire du workspace (ex. "free", "pro", "enterprise").
     * TODO(schema): colonne plan absente de 0001 — aligner migration avant production.
     */
    public readonly plan: string,
    public readonly createdAt: Date,
  ) {}
}
