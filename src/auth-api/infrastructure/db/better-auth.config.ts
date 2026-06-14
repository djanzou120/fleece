// Couche 4 — Configuration du framework Better Auth + Drizzle (schéma "identity").
//
// L'initialisation réelle (betterAuth({...}) avec l'adapter Drizzle) est branchée
// ici, au plus loin du domaine. Atlas reste propriétaire des migrations :
// Drizzle n'exécute pas les siennes (voir .ia/ARCHITECTURE.md §6.4).
//
// OFFLINE STUB : better-auth et drizzle-orm ne sont pas installés.
// TODO: import { betterAuth } from 'better-auth'
// TODO: import { drizzleAdapter } from 'better-auth/adapters/drizzle'

export const identitySchema = "identity";

/**
 * Configuration Better Auth (stub offline).
 *
 * À activer une fois better-auth + drizzle-orm installés :
 *
 * ```ts
 * import { betterAuth } from 'better-auth';
 * import { drizzleAdapter } from 'better-auth/adapters/drizzle';
 * import { db } from './connection.js';
 *
 * export const auth = betterAuth({
 *   database: drizzleAdapter(db, {
 *     provider: "pg",
 *     schema: identitySchema,
 *   }),
 *   emailAndPassword: { enabled: true },
 *   session: {
 *     expiresIn: 60 * 60 * 24 * 7, // 7 jours
 *   },
 * });
 * ```
 */
export const betterAuthConfig = {
  // database: drizzleAdapter(db, { provider: "pg", schema: identitySchema }),
  // emailAndPassword: { enabled: true },
  // session: { expiresIn: 604800 },
} as const;

/**
 * Instance Better Auth (stub offline).
 * TODO: remplacer par `export const auth = betterAuth(betterAuthConfig)` une fois installé.
 */
export const auth = {
  createSession: async (_userId: string) => {
    throw new Error("better-auth non installé — stub offline");
  },
  getSession: async (_token: string) => {
    throw new Error("better-auth non installé — stub offline");
  },
  revokeSession: async (_sessionId: string) => {
    throw new Error("better-auth non installé — stub offline");
  },
} as const;
