// Couche 3 — Adapter : confine Better Auth derrière le port AuthProvider.
// Better Auth est un DÉTAIL d'implémentation ; le domaine et les use cases
// ne l'importent jamais. Voir .ia/ARCHITECTURE.md §4.1.
//
// OFFLINE STUB : better-auth n'est pas installé dans node_modules.
// TODO: import { betterAuth, Session } from 'better-auth'
// Les appels réels sont commentés ci-dessous ; les stubs lèvent des erreurs explicites.

import { AuthProvider, SessionData } from "../../application/ports/output/repositories.js";

// ---------------------------------------------------------------------------
// Interface locale typant les méthodes Better Auth utilisées par cet adapter.
// Remplace l'import du package (non installé en offline).
// TODO: remplacer par les types réels de 'better-auth' une fois le package installé.
// ---------------------------------------------------------------------------

interface BetterAuthSession {
  id: string;
  userId: string;
  expiresAt: Date;
}

/** Forme minimale de l'API Better Auth dont cet adapter a besoin. */
interface BetterAuthLike {
  /** Crée une nouvelle session pour un utilisateur. */
  createSession(userId: string): Promise<BetterAuthSession>;
  /** Valide un token de session et retourne les données ou null si invalide. */
  getSession(token: string): Promise<{ session: BetterAuthSession } | null>;
  /** Supprime une session (déconnexion). */
  revokeSession(sessionId: string): Promise<void>;
}

// ---------------------------------------------------------------------------
// Implémentation du port AuthProvider wrappant Better Auth
// ---------------------------------------------------------------------------

export class BetterAuthProvider implements AuthProvider {
  /**
   * L'instance Better Auth est construite en couche 4 (infrastructure/db/better-auth.config.ts)
   * et injectée ici, conformément à la règle de dépendance.
   */
  constructor(private readonly auth: BetterAuthLike) {}

  async createSession(userId: string): Promise<SessionData> {
    // TODO: appel réel Better Auth une fois le package installé :
    // const session = await this.auth.createSession(userId);
    // return { sessionId: session.id, userId: session.userId, expiresAt: session.expiresAt };

    // STUB offline — simule une session valide pour le développement
    throw new Error(
      "BetterAuthProvider.createSession: stub offline — installer better-auth pour activer",
    );
  }

  async validateSession(token: string): Promise<SessionData | null> {
    // TODO: appel réel Better Auth une fois le package installé :
    // const result = await this.auth.getSession(token);
    // if (!result) return null;
    // const { session } = result;
    // return { sessionId: session.id, userId: session.userId, expiresAt: session.expiresAt };

    // STUB offline
    void token;
    throw new Error(
      "BetterAuthProvider.validateSession: stub offline — installer better-auth pour activer",
    );
  }

  async deleteSession(sessionId: string): Promise<void> {
    // TODO: appel réel Better Auth une fois le package installé :
    // await this.auth.revokeSession(sessionId);

    void sessionId;
    throw new Error(
      "BetterAuthProvider.deleteSession: stub offline — installer better-auth pour activer",
    );
  }

  /** Rétrocompatibilité — délègue à validateSession. */
  async verifySession(token: string): Promise<{ userId: string } | null> {
    const session = await this.validateSession(token);
    return session ? { userId: session.userId } : null;
  }
}
