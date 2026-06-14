// Couche 2 — Ports pilotés : interfaces requises par les use cases.
// Implémentées en couche 3 (persistence Drizzle, adapter Better Auth).
// Aucune dépendance framework ici — types purs uniquement.

import { ApiKey } from "../../../domain/api-key.js";
import { Workspace } from "../../../domain/workspace.js";
import { User } from "../../../domain/user.js";

// ---------------------------------------------------------------------------
// Repository ports
// ---------------------------------------------------------------------------

export interface WorkspaceRepository {
  save(workspace: Workspace): Promise<void>;
  findById(id: string): Promise<Workspace | null>;
}

export interface ApiKeyRepository {
  save(key: ApiKey): Promise<void>;
  findById(id: string): Promise<ApiKey | null>;
  findByHash(hash: string): Promise<ApiKey | null>;
  /** Met à jour la date de dernière utilisation d'une clé. */
  updateLastUsed(id: string, at: Date): Promise<void>;
  /** Marque la clé comme révoquée (active=false, revokedAt=at). */
  markRevoked(id: string, at: Date): Promise<void>;
}

export interface UserRepository {
  save(user: User): Promise<void>;
  findById(id: string): Promise<User | null>;
  findByWorkspace(workspaceId: string): Promise<User[]>;
}

// ---------------------------------------------------------------------------
// Auth provider port
// ---------------------------------------------------------------------------

/** Données de session créée (types purs — aucun type Better Auth ici). */
export interface SessionData {
  sessionId: string;
  userId: string;
  expiresAt: Date;
}

/**
 * Port d'authentification (couche 2).
 * Better Auth en sera l'implémentation (couche 3 — adapters/auth/).
 * Le domaine et les use cases ne connaissent jamais le framework.
 */
export interface AuthProvider {
  createSession(userId: string): Promise<SessionData>;
  validateSession(token: string): Promise<SessionData | null>;
  deleteSession(sessionId: string): Promise<void>;
  /** Rétrocompatibilité — délègue à validateSession. */
  verifySession(token: string): Promise<{ userId: string } | null>;
}
