// Couche 3 — Adapter : confine Better Auth derrière le port AuthProvider.
// Better Auth est un DÉTAIL d'implémentation ; le domaine et les use cases
// ne l'importent jamais. Voir .ia/ARCHITECTURE.md §4.1.

import { AuthProvider } from "../../application/ports/output/repositories";

/** Forme minimale de l'instance Better Auth dont l'adapter a besoin. */
interface BetterAuthLike {
  getSession(token: string): Promise<{ user: { id: string } } | null>;
}

export class BetterAuthProvider implements AuthProvider {
  // L'instance Better Auth est construite en couche 4 (infrastructure) et
  // injectée ici, conformément à la règle de dépendance.
  constructor(private readonly auth: BetterAuthLike) {}

  async verifySession(token: string): Promise<{ userId: string } | null> {
    const session = await this.auth.getSession(token);
    return session ? { userId: session.user.id } : null;
  }
}
