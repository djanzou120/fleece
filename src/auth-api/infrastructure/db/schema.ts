// Couche 4 — Infrastructure : schéma Drizzle du schéma PostgreSQL "identity".
//
// M-029 — CE FICHIER NE DÉFINIT PLUS RIEN. Il ne fait que ré-exporter les
// définitions partagées de `@fleece/model` (src/ts/model/schema/identity.ts),
// qui sont désormais l'unique endroit où le schéma `identity` est décrit côté
// TypeScript.
//
// Ce qu'il contenait avant : des STUBS MAISON (`pgSchemaStub`, faux `uuid()`,
// `text()`, `varchar()`…) écrits quand drizzle-orm était réputé indisponible.
// Ils ne portaient NI clé primaire, NI NOT NULL, NI valeur par défaut, NI clé
// étrangère, NI contrainte UNIQUE — c'était une esquisse de typage, pas un
// schéma. drizzle-orm (0.39.3) est en réalité installé, et `@fleece/model`
// utilise les vrais constructeurs `drizzle-orm/pg-core`, avec une fidélité
// aux migrations Atlas **prouvée** : M-028 a appliqué les migrations Atlas et
// le DDL généré depuis ces définitions sur deux bases PostgreSQL 16 distinctes,
// puis comparé leur introspection — 28 colonnes, 8 contraintes, 7 index et
// 1 séquence identiques, sans aucune différence.
//
// Ce fichier est conservé (plutôt que supprimé) parce qu'il est le point
// d'ancrage attendu par la couche 4 : les adapters de persistence importent
// « le schéma de leur infrastructure », pas directement un paquet partagé.
// C'est aussi ce qui permettra d'ajouter un jour des vues ou des tables
// propres à auth-api sans toucher au paquet commun.
//
// ATTENTION AU RENOMMAGE DES PROPRIÉTÉS : les définitions partagées suivent la
// convention Drizzle (propriété TS en camelCase → colonne SQL en snake_case).
// Les colonnes en base sont INCHANGÉES, mais le code applicatif s'écrit
// désormais `usersTable.workspaceId` et `apiKeysTable.hashedKey`, là où les
// exemples encore commentés dans adapters/persistence/*.ts utilisent
// `usersTable.workspace_id` et `apiKeysTable.hashed_key`. Ces exemples devront
// être ajustés au moment de leur activation (voir D-M48).

export {
  apiKeys as apiKeysTable,
  auditLogs as auditLogsTable,
  identitySchema,
  users as usersTable,
  workspaces as workspacesTable,
} from "@fleece/model";

export type {
  ApiKey,
  AuditLog,
  NewApiKey,
  NewAuditLog,
  NewUser,
  NewWorkspace,
  User,
  Workspace,
} from "@fleece/model";
