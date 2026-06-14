// Couche 1 — Domain (pur, aucun import externe). Voir .ia/ARCHITECTURE.md §4.1.
// Erreurs métier du domaine Identity.

/** Le workspace demandé n'existe pas. */
export class WorkspaceNotFoundError extends Error {
  constructor(id: string) {
    super(`Workspace non trouvé : ${id}`);
    this.name = "WorkspaceNotFoundError";
  }
}

/** La clé API demandée n'existe pas. */
export class ApiKeyNotFoundError extends Error {
  constructor(id: string) {
    super(`Clé API non trouvée : ${id}`);
    this.name = "ApiKeyNotFoundError";
  }
}

/** La clé API a été révoquée. */
export class ApiKeyRevokedError extends Error {
  constructor(id: string) {
    super(`Clé API révoquée : ${id}`);
    this.name = "ApiKeyRevokedError";
  }
}

/** La clé API est invalide (hash non reconnu, expirée, etc.). */
export class InvalidApiKeyError extends Error {
  constructor(reason?: string) {
    super(reason ?? "Clé API invalide");
    this.name = "InvalidApiKeyError";
  }
}

/** L'opération demandée n'est pas autorisée pour cet appelant. */
export class UnauthorizedError extends Error {
  constructor(reason?: string) {
    super(reason ?? "Non autorisé");
    this.name = "UnauthorizedError";
  }
}
