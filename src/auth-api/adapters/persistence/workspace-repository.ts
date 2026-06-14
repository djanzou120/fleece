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
// Mappage entité domaine ↔ lignes 0001_identity.sql
// ---------------------------------------------------------------------------

/** Ligne telle que retournée par la table identity.workspaces (colonnes 0001 exactes). */
interface WorkspaceRow {
  id: string;
  name: string;
  country: string;
  created_at: Date;
  // Colonnes absentes de 0001 — non présentes dans cette interface :
  // slug     : TODO(schema): aligner migration avant production
  // owner_id : TODO(schema): aligner migration avant production
  // plan     : TODO(schema): aligner migration avant production
}

function rowToEntity(row: WorkspaceRow): Workspace {
  return new Workspace(
    row.id,
    row.name,
    row.country,
    // ownerId  : TODO(schema): absent de 0001 — valeur par défaut en attendant la migration
    "",
    // slug     : TODO(schema): absent de 0001 — dérivé du nom à la lecture
    row.name.toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, ""),
    // plan     : TODO(schema): absent de 0001 — "free" par défaut
    "free",
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
    //   // slug, owner_id, plan : TODO(schema) — colonnes absentes de 0001
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
