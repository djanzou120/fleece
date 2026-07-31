// Définitions Drizzle du schéma PostgreSQL `identity` (M-027).
//
// ┌───────────────────────────────────────────────────────────────────────────┐
// │ ATLAS EST LA SOURCE DE VÉRITÉ. Ce fichier REFLÈTE les migrations, il ne   │
// │ les produit pas. Ne JAMAIS lancer `drizzle-kit migrate`/`push` contre une │
// │ base Fleece (règle CLAUDE.md). `drizzle-kit generate` n'est utilisé que   │
// │ pour PROUVER l'équivalence avec les migrations Atlas (M-028), et sa       │
// │ sortie va dans un répertoire de travail jetable, jamais dans migrations/. │
// └───────────────────────────────────────────────────────────────────────────┘
//
// Source de ces définitions — les DEUX migrations doivent être lues ensemble,
// la seconde étant strictement additive sur la première :
//   • migrations/0001_identity.sql        — tables et colonnes d'origine
//   • migrations/0011_identity_schema.sql — colonnes ajoutées (dette T-006)
//
// PÉRIMÈTRE : `identity` UNIQUEMENT. M-027 mentionnait aussi 0002 (wallet),
// 0003 (messaging) et 0006 (webhook), mais sa propre parenthèse disait
// « (schémas identity) » — et modéliser ici les schémas de services Go
// violerait la règle « un schéma par service » : `auth-api` est le seul
// service TS propriétaire d'un schéma. Vérifié pendant M-027 : `graphql-api`
// n'a AUCUN accès SQL (0 occurrence de drizzle/pg/POSTGRES_*), il lit ces
// domaines via les clients REST vers `src/api`. Exposer des tables `wallet`/
// `messaging`/`webhook` dans un paquet TS aurait invité précisément l'accès
// cross-schéma que l'architecture interdit.
//
// FIDÉLITÉ : nullabilité, valeurs par défaut, clés primaires, clés étrangères
// et contraintes UNIQUE sont reproduites à l'identique — c'est ce qui rend la
// comparaison de M-028 significative. Les contraintes UNIQUE et PRIMARY KEY
// sont nommées EXPLICITEMENT avec le nom que PostgreSQL aurait généré
// (`<table>_pkey`, `<table>_<colonne>_key`) pour que les deux schémas soient
// comparables jusqu'au nom, et pas seulement « sémantiquement proches ».

import { sql } from "drizzle-orm";
import {
  bigserial,
  pgSchema,
  text,
  timestamp,
  uniqueIndex,
  uuid,
  varchar,
} from "drizzle-orm/pg-core";

/** Le schéma PostgreSQL `identity`, propriété du service auth-api. */
export const identitySchema = pgSchema("identity");

/**
 * identity.workspaces
 *
 * 0001 : id (PK), name, country, created_at
 * 0011 : slug (varchar 100, NOT NULL DEFAULT ''), owner_id (uuid nullable),
 *        plan (varchar 50, NOT NULL DEFAULT 'free')
 *
 * `uq_workspaces_slug` est un INDEX UNIQUE nommé, pas une contrainte UNIQUE
 * anonyme : 0011 le crée ainsi délibérément (idempotence + lisibilité en
 * introspection). Le reproduire en `uniqueIndex()` — et non en `.unique()` —
 * est ce qui fait correspondre les deux schémas.
 */
export const workspaces = identitySchema.table(
  "workspaces",
  {
    id: uuid("id").primaryKey(),
    name: text("name").notNull(),
    country: text("country").notNull(),
    slug: varchar("slug", { length: 100 }).notNull().default(""),
    ownerId: uuid("owner_id"),
    plan: varchar("plan", { length: 50 }).notNull().default("free"),
    createdAt: timestamp("created_at", { withTimezone: true })
      .notNull()
      .defaultNow(),
  },
  (table) => [uniqueIndex("uq_workspaces_slug").on(table.slug)],
);

/**
 * identity.users
 *
 * 0001 : id (PK), workspace_id (FK → workspaces.id), email (UNIQUE), created_at
 * 0011 : name (varchar 255, NOT NULL DEFAULT ''), role (varchar 50, NOT NULL
 *        DEFAULT 'member')
 */
export const users = identitySchema.table("users", {
  id: uuid("id").primaryKey(),
  workspaceId: uuid("workspace_id")
    .notNull()
    .references(() => workspaces.id),
  email: text("email").notNull().unique("users_email_key"),
  name: varchar("name", { length: 255 }).notNull().default(""),
  role: varchar("role", { length: 50 }).notNull().default("member"),
  createdAt: timestamp("created_at", { withTimezone: true })
    .notNull()
    .defaultNow(),
});

/**
 * identity.api_keys
 *
 * 0001 : id (PK), workspace_id (FK → workspaces.id), hashed_key (UNIQUE),
 *        status (NOT NULL DEFAULT 'active'), created_at, revoked_at (nullable)
 * 0011 : name, prefix, permissions (TEXT[] NOT NULL DEFAULT '{}'),
 *        last_used_at (nullable), expires_at (nullable)
 *
 * `permissions` : le défaut est écrit en SQL brut (`'{}'`) plutôt qu'en
 * littéral TS `[]`, pour que le DDL généré porte exactement le même texte que
 * la migration 0011 et non une forme équivalente mais différemment sérialisée.
 */
export const apiKeys = identitySchema.table("api_keys", {
  id: uuid("id").primaryKey(),
  workspaceId: uuid("workspace_id")
    .notNull()
    .references(() => workspaces.id),
  hashedKey: text("hashed_key").notNull().unique("api_keys_hashed_key_key"),
  status: text("status").notNull().default("active"),
  name: varchar("name", { length: 255 }).notNull().default(""),
  prefix: varchar("prefix", { length: 20 }).notNull().default(""),
  permissions: text("permissions")
    .array()
    .notNull()
    .default(sql`'{}'`),
  lastUsedAt: timestamp("last_used_at", { withTimezone: true }),
  expiresAt: timestamp("expires_at", { withTimezone: true }),
  createdAt: timestamp("created_at", { withTimezone: true })
    .notNull()
    .defaultNow(),
  revokedAt: timestamp("revoked_at", { withTimezone: true }),
});

/**
 * identity.audit_logs
 *
 * 0001 : id (bigserial PK), workspace_id, action, created_at
 *
 * ATTENTION — `workspace_id` n'a **volontairement PAS** de clé étrangère ici,
 * contrairement à `users` et `api_keys` : 0001 la déclare `uuid NOT NULL` tout
 * court. Un journal d'audit doit survivre à la suppression de ce qu'il
 * journalise ; ajouter la FK « par cohérence » changerait le schéma réel et
 * ferait échouer la comparaison de M-028. Ne pas « corriger » cette asymétrie
 * ici : elle appartient aux migrations.
 */
export const auditLogs = identitySchema.table("audit_logs", {
  id: bigserial("id", { mode: "bigint" }).primaryKey(),
  workspaceId: uuid("workspace_id").notNull(),
  action: text("action").notNull(),
  createdAt: timestamp("created_at", { withTimezone: true })
    .notNull()
    .defaultNow(),
});

/** Types d'entités inférés depuis les définitions Drizzle. */
export type Workspace = typeof workspaces.$inferSelect;
export type NewWorkspace = typeof workspaces.$inferInsert;
export type User = typeof users.$inferSelect;
export type NewUser = typeof users.$inferInsert;
export type ApiKey = typeof apiKeys.$inferSelect;
export type NewApiKey = typeof apiKeys.$inferInsert;
export type AuditLog = typeof auditLogs.$inferSelect;
export type NewAuditLog = typeof auditLogs.$inferInsert;
