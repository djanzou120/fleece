// Couche 1 — Domain. Voir .ia/ARCHITECTURE.md.

/** Entité Workspace : espace isolé d'une entreprise cliente. */
export class Workspace {
  constructor(
    public readonly id: string,
    public readonly name: string,
    /** Pays du workspace — détermine le moyen de paiement (Mobile Money / Stripe). */
    public readonly country: string,
  ) {}
}
