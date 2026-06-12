// Couche 2 — Ports pilotés : interfaces requises par les use cases.
// Implémentées en couche 3 (persistence Drizzle, adapter Better Auth).

import { ApiKey } from "../../../domain/api-key";
import { Workspace } from "../../../domain/workspace";

export interface WorkspaceRepository {
  save(workspace: Workspace): Promise<void>;
  findById(id: string): Promise<Workspace | null>;
}

export interface ApiKeyRepository {
  save(key: ApiKey): Promise<void>;
  findByHash(hash: string): Promise<ApiKey | null>;
}

/**
 * Port d'authentification. Better Auth en sera l'implémentation (couche 3) :
 * le domaine ne connaît jamais le framework.
 */
export interface AuthProvider {
  verifySession(token: string): Promise<{ userId: string } | null>;
}
