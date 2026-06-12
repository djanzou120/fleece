// Couche 2 — Ports pilotés : interfaces des clients de services (REST interne).
// Le BFF n'a pas de domaine propre ; il orchestre les services métier.

export interface IdentityClient {
  getWorkspace(id: string): Promise<{ id: string; name: string } | null>;
}

export interface WalletClient {
  getBalance(workspaceId: string): Promise<number>;
}
