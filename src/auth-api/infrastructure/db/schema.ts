// Couche 4 — Infrastructure : définitions de tables Drizzle pour le schéma "identity".
// ALIGNÉES sur migrations/0001_identity.sql + 0011_identity_schema.sql (Atlas = source de vérité).
// Drizzle = query builder + typage UNIQUEMENT. Ne pas activer les migrations Drizzle.
//
// OFFLINE STUB : drizzle-orm peut ne pas être installé.
// TODO: import { pgSchema, pgTable, uuid, text, varchar, timestamp, bigserial } from 'drizzle-orm/pg-core'

// ---------------------------------------------------------------------------
// Stubs locaux Drizzle (types purs, sans implémentation)
// À remplacer par les imports réels une fois drizzle-orm disponible.
// ---------------------------------------------------------------------------

// TODO: import réel — décommente et supprime les stubs ci-dessous :
// import {
//   pgSchema,
//   uuid,
//   text,
//   varchar,
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
/**
 * Stub text() — supporte .array() pour modéliser TEXT[].
 * TODO: remplacer par `text()` de drizzle-orm/pg-core une fois installé.
 */
const text = () => {
  const col = { _type: "text" } as DrizzleColumn & { array: () => DrizzleColumn };
  col.array = () => ({ _type: "text[]" }) as DrizzleColumn;
  return col;
};
/** varchar(n) — stub acceptant le nom de colonne et les options (ex. { length: 100 }). */
const varchar = (_name?: string, _opts?: Record<string, unknown>) =>
  ({ _type: "varchar" }) as DrizzleColumn;
const timestamp = (_name?: string, _opts?: Record<string, unknown>) =>
  ({ _type: "timestamp" }) as DrizzleColumn;
const bigserial = (_name?: string, _opts?: Record<string, unknown>) =>
  ({ _type: "bigserial" }) as DrizzleColumn;

// ---------------------------------------------------------------------------
// Schéma Drizzle : "identity" (reflète 0001_identity.sql + 0011_identity_schema.sql)
// ---------------------------------------------------------------------------

const identity = pgSchemaStub("identity");

/**
 * identity.workspaces
 * Colonnes 0001 : id, name, country, created_at
 * Colonnes 0011 : slug (varchar 100, NOT NULL DEFAULT ''), owner_id (uuid, nullable), plan (varchar 50, NOT NULL DEFAULT 'free')
 */
export const workspacesTable = identity.table("workspaces", {
  id: uuid(),
  name: text(),
  country: text(),
  /** slug : VARCHAR(100) NOT NULL DEFAULT '' — ajouté par 0011_identity_schema.sql */
  slug: varchar("slug", { length: 100 }),
  /** owner_id : UUID nullable — ajouté par 0011_identity_schema.sql */
  owner_id: uuid(),
  /** plan : VARCHAR(50) NOT NULL DEFAULT 'free' — ajouté par 0011_identity_schema.sql */
  plan: varchar("plan", { length: 50 }),
  created_at: timestamp("created_at", { withTimezone: true }),
});

/**
 * identity.users
 * Colonnes 0001 : id, workspace_id, email, created_at
 * Colonnes 0011 : name (varchar 255, NOT NULL DEFAULT ''), role (varchar 50, NOT NULL DEFAULT 'member')
 */
export const usersTable = identity.table("users", {
  id: uuid(),
  workspace_id: uuid(),
  email: text(),
  /** name : VARCHAR(255) NOT NULL DEFAULT '' — ajouté par 0011_identity_schema.sql */
  name: varchar("name", { length: 255 }),
  /** role : VARCHAR(50) NOT NULL DEFAULT 'member' — ajouté par 0011_identity_schema.sql */
  role: varchar("role", { length: 50 }),
  created_at: timestamp("created_at", { withTimezone: true }),
});

/**
 * identity.api_keys
 * Colonnes 0001 : id, workspace_id, hashed_key, status, created_at, revoked_at
 * Colonnes 0011 :
 *   name         : VARCHAR(255) NOT NULL DEFAULT ''
 *   prefix       : VARCHAR(20)  NOT NULL DEFAULT ''
 *   permissions  : TEXT[]       NOT NULL DEFAULT '{}'
 *   last_used_at : TIMESTAMPTZ  nullable
 *   expires_at   : TIMESTAMPTZ  nullable
 */
export const apiKeysTable = identity.table("api_keys", {
  id: uuid(),
  workspace_id: uuid(),
  hashed_key: text(),
  status: text(), // 'active' | 'revoked'
  /** name : VARCHAR(255) NOT NULL DEFAULT '' — ajouté par 0011_identity_schema.sql */
  name: varchar("name", { length: 255 }),
  /** prefix : VARCHAR(20) NOT NULL DEFAULT '' — ajouté par 0011_identity_schema.sql */
  prefix: varchar("prefix", { length: 20 }),
  /** permissions : TEXT[] NOT NULL DEFAULT '{}' — ajouté par 0011_identity_schema.sql */
  permissions: text().array(), // TEXT[] — aligné sur la colonne réelle (migrations 0011)
  /** last_used_at : TIMESTAMPTZ nullable — ajouté par 0011_identity_schema.sql */
  last_used_at: timestamp("last_used_at", { withTimezone: true }),
  /** expires_at : TIMESTAMPTZ nullable — ajouté par 0011_identity_schema.sql */
  expires_at: timestamp("expires_at", { withTimezone: true }),
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
