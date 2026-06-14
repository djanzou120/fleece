// Couche 3 — Adapter persistence : implémentation de WorkspaceRepository avec Drizzle.
// Drizzle est confiné dans adapters/persistence/ et infrastructure/db/.
//
// OFFLINE STUB : drizzle-orm non installé.
// TODO: import { eq } from 'drizzle-orm'
// TODO: import { drizzle, NodePgDatabase } from 'drizzle-orm/node-postgres'
// Les requêtes réelles sont commentées ; les stubs lèvent une erreur explicite.

import { Workspace } from "../../domain/workspace.js";
import { WorkspaceRepository } from "../../application/ports/output/repositories.js";

// ---------------------------------------------------------------------------
// Type local simulant une instance Drizzle (db).
// TODO: remplacer par NodePgDatabase<typeof schema> de drizzle-orm/node-postgres
// ---------------------------------------------------------------------------
export interface DrizzleDb {
  // Interface minimaliste pour les besoins de cet adapter.
  // Les types Drizzle réels seront injectés depuis infrastructure/db/connection.ts
  query: unknown;
  insert: unknown;
  update: unknown;
  select: unknown;
}

// ---------------------------------------------------------------------------
// Mappage entité domaine ↔ lignes SQL (0001 + 0011_identity_schema.sql)
// ---------------------------------------------------------------------------

/**
 * Ligne telle que retournée par la table identity.workspaces.
 * Colonnes 0001 : id, name, country, created_at
 * Colonnes 0011 : slug, owner_id, plan
 */
interface WorkspaceRow {
  id: string;
  name: string;
  country: string;
  /** slug : VARCHAR(100) NOT NULL DEFAULT '' — disponible depuis 0011 */
  slug: string;
  /** owner_id : UUID nullable — disponible depuis 0011 */
  owner_id: string | null;
  /** plan : VARCHAR(50) NOT NULL DEFAULT 'free' — disponible depuis 0011 */
  plan: string;
  created_at: Date;
}

function rowToEntity(row: WorkspaceRow): Workspace {
  return new Workspace(
    row.id,
    row.name,
    row.country,
    // owner_id → ownerId (nullable en base, chaîne vide si absent)
    row.owner_id ?? "",
    // slug : persisté depuis 0011
    row.slug,
    // plan : persisté depuis 0011
    row.plan,
    row.created_at,
  );
}

// ---------------------------------------------------------------------------
// Implémentation
// ---------------------------------------------------------------------------

export class DrizzleWorkspaceRepository implements WorkspaceRepository {
  constructor(private readonly db: DrizzleDb) {}

  async save(workspace: Workspace): Promise<void> {
    // TODO(drizzle): remplacer par l'insert Drizzle réel :
    // await (this.db as NodePgDatabase).insert(workspacesTable).values({
    //   id: workspace.id,
    //   name: workspace.name,
    //   country: workspace.country,
    //   slug: workspace.slug,
    //   owner_id: workspace.ownerId || null,
    //   plan: workspace.plan,
    //   created_at: workspace.createdAt,
    // }).onConflictDoNothing();
    void workspace;
    throw new Error(
      "DrizzleWorkspaceRepository.save: stub offline — installer drizzle-orm pour activer",
    );
  }

  async findById(id: string): Promise<Workspace | null> {
    // TODO(drizzle): remplacer par la requête Drizzle réelle :
    // const rows = await (this.db as NodePgDatabase)
    //   .select()
    //   .from(workspacesTable)
    //   .where(eq(workspacesTable.id, id))
    //   .limit(1);
    // if (rows.length === 0) return null;
    // return rowToEntity(rows[0] as WorkspaceRow);
    void id;
    void rowToEntity; // utilisé dans la version réelle
    throw new Error(
      "DrizzleWorkspaceRepository.findById: stub offline — installer drizzle-orm pour activer",
    );
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: WorkspaceRepository = new DrizzleWorkspaceRepository({} as DrizzleDb);
void _conformance;
