// Couche 4 — Infrastructure : définitions de tables Drizzle pour le schéma "identity".
// ALIGNÉES sur migrations/0001_identity.sql (Atlas = source de vérité).
// Drizzle = query builder + typage UNIQUEMENT. Ne pas activer les migrations Drizzle.
//
// OFFLINE STUB : drizzle-orm peut ne pas être installé.
// TODO: import { pgSchema, pgTable, uuid, text, timestamp, bigserial } from 'drizzle-orm/pg-core'

// ---------------------------------------------------------------------------
// Stubs locaux Drizzle (types purs, sans implémentation)
// À remplacer par les imports réels une fois drizzle-orm disponible.
// ---------------------------------------------------------------------------

// TODO: import réel — décommente et supprime les stubs ci-dessous :
// import {
//   pgSchema,
//   uuid,
//   text,
//   timestamp,
//   bigserial,
// } from "drizzle-orm/pg-core";

/** Stub local simulant le type d'une colonne Drizzle. */
type DrizzleColumn = { _type: string };

/** Stub local simulant la définition d'une table Drizzle. */
type DrizzleTable<T extends Record<string, DrizzleColumn>> = {
  [K in keyof T]: T[K];
};

/** Stub local simulant pgSchema. */
function pgSchemaStub(name: string) {
  return {
    table<T extends Record<string, DrizzleColumn>>(
      _tableName: string,
      _columns: T,
    ): DrizzleTable<T> {
      void name;
      return _columns as DrizzleTable<T>;
    },
  };
}

/** Stubs de types de colonnes Drizzle. */
const uuid = () => ({ _type: "uuid" }) as DrizzleColumn;
const text = () => ({ _type: "text" }) as DrizzleColumn;
const timestamp = (_name?: string, _opts?: Record<string, unknown>) =>
  ({ _type: "timestamp" }) as DrizzleColumn;
const bigserial = (_name?: string, _opts?: Record<string, unknown>) =>
  ({ _type: "bigserial" }) as DrizzleColumn;

// ---------------------------------------------------------------------------
// Schéma Drizzle : "identity" (reflète 0001_identity.sql exactement)
// ---------------------------------------------------------------------------

const identity = pgSchemaStub("identity");

/**
 * identity.workspaces
 * Colonnes 0001 : id, name, country, created_at
 * NON persistés (schema debt) : slug, owner_id, plan
 */
export const workspacesTable = identity.table("workspaces", {
  id: uuid(),
  name: text(),
  country: text(),
  // slug        : TODO(schema): absent de 0001 — aligner migration avant production
  // owner_id    : TODO(schema): absent de 0001 — aligner migration avant production
  // plan        : TODO(schema): absent de 0001 — aligner migration avant production
  created_at: timestamp("created_at", { withTimezone: true }),
});

/**
 * identity.users
 * Colonnes 0001 : id, workspace_id, email, created_at
 * NON persistés (schema debt) : name, role
 */
export const usersTable = identity.table("users", {
  id: uuid(),
  workspace_id: uuid(),
  email: text(),
  // name : TODO(schema): absent de 0001 — aligner migration avant production
  // role : TODO(schema): absent de 0001 — aligner migration avant production
  created_at: timestamp("created_at", { withTimezone: true }),
});

/**
 * identity.api_keys
 * Colonnes 0001 : id, workspace_id, hashed_key, status, created_at, revoked_at
 * NON persistés (schema debt) : name, prefix, permissions, last_used_at, expires_at
 */
export const apiKeysTable = identity.table("api_keys", {
  id: uuid(),
  workspace_id: uuid(),
  hashed_key: text(),
  status: text(), // 'active' | 'revoked'
  // name         : TODO(schema): absent de 0001 — aligner migration avant production
  // prefix       : TODO(schema): absent de 0001 — aligner migration avant production
  // permissions  : TODO(schema): absent de 0001 — aligner migration avant production
  // last_used_at : TODO(schema): absent de 0001 — aligner migration avant production
  // expires_at   : TODO(schema): absent de 0001 — aligner migration avant production
  created_at: timestamp("created_at", { withTimezone: true }),
  revoked_at: timestamp("revoked_at", { withTimezone: true }),
});

/**
 * identity.audit_logs
 * Colonnes 0001 : id, workspace_id, action, created_at
 */
export const auditLogsTable = identity.table("audit_logs", {
  id: bigserial("id"),
  workspace_id: uuid(),
  action: text(),
  created_at: timestamp("created_at", { withTimezone: true }),
});
