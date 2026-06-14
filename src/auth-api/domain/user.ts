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
 * Champs persistés dans 0001 : id, workspace_id, email, created_at.
 * Champs riches non persistés :
 *  - name : TODO(schema): absent de 0001 — aligner migration avant production.
 *  - role : TODO(schema): absent de 0001 — aligner migration avant production.
 */
export interface User {
  readonly id: string;
  readonly email: string;
  /**
   * Nom d'affichage.
   * TODO(schema): colonne name absente de 0001 — aligner migration avant production.
   */
  readonly name: string;
  readonly workspaceId: string;
  /**
   * Rôle dans le workspace.
   * TODO(schema): colonne role absente de 0001 — aligner migration avant production.
   * Valeur par défaut à la lecture : "owner" pour le premier utilisateur créé avec le workspace.
   */
  readonly role: UserRole;
  readonly createdAt: Date;
}
