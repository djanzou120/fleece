// Couche 1 — Domain (pur, aucun import externe). Voir .ia/ARCHITECTURE.md §4.1.

/**
 * Rôle d'un utilisateur dans un workspace.
 * "owner" = créateur du workspace, droits complets.
 * "member" = collaborateur invité.
 */
export type UserRole = "owner" | "member";

/**
 * Entité User : utilisateur appartenant à un workspace.
 *
 * Tous les champs sont désormais persistés :
 *   - 0001_identity.sql : id, workspace_id, email, created_at
 *   - 0011_identity_schema.sql : name, role
 */
export interface User {
  readonly id: string;
  readonly email: string;
  /** Nom d'affichage. Persisté (0011 — colonne name VARCHAR(255)). */
  readonly name: string;
  readonly workspaceId: string;
  /** Rôle dans le workspace. Persisté (0011 — colonne role VARCHAR(50)). */
  readonly role: UserRole;
  readonly createdAt: Date;
}
