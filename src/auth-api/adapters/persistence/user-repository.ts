// Couche 3 — Adapter persistence : implémentation de UserRepository avec Drizzle.
// Drizzle est confiné dans adapters/persistence/ et infrastructure/db/.
//
// OFFLINE STUB : drizzle-orm non installé.
// TODO: import { eq } from 'drizzle-orm'
// TODO: import { NodePgDatabase } from 'drizzle-orm/node-postgres'

import { User } from "../../domain/user.js";
import { UserRepository } from "../../application/ports/output/repositories.js";
import { DrizzleDb } from "./workspace-repository.js";

// ---------------------------------------------------------------------------
// Mappage entité domaine ↔ lignes 0001_identity.sql
// Colonnes disponibles : id, workspace_id, email, created_at
// ---------------------------------------------------------------------------

/** Ligne telle que retournée par identity.users (colonnes 0001 exactes). */
interface UserRow {
  id: string;
  workspace_id: string;
  email: string;
  created_at: Date;
  // Colonnes absentes de 0001 :
  // name : TODO(schema): aligner migration avant production
  // role : TODO(schema): aligner migration avant production
}

function rowToEntity(row: UserRow): User {
  return {
    id: row.id,
    email: row.email,
    workspaceId: row.workspace_id,
    // name : TODO(schema): absent de 0001 — chaîne vide par défaut
    name: "",
    // role : TODO(schema): absent de 0001 — "owner" par défaut (convention : 1er utilisateur = owner)
    role: "owner",
    createdAt: row.created_at,
  };
}

// ---------------------------------------------------------------------------
// Implémentation
// ---------------------------------------------------------------------------

export class DrizzleUserRepository implements UserRepository {
  constructor(private readonly db: DrizzleDb) {}

  async save(user: User): Promise<void> {
    // TODO(drizzle): remplacer par l'insert Drizzle réel :
    // await (this.db as NodePgDatabase).insert(usersTable).values({
    //   id: user.id,
    //   workspace_id: user.workspaceId,
    //   email: user.email,
    //   created_at: user.createdAt,
    //   // name, role : TODO(schema) — colonnes absentes de 0001
    // });
    void user;
    throw new Error(
      "DrizzleUserRepository.save: stub offline — installer drizzle-orm pour activer",
    );
  }

  async findById(id: string): Promise<User | null> {
    // TODO(drizzle): remplacer par la requête Drizzle réelle :
    // const rows = await (this.db as NodePgDatabase)
    //   .select().from(usersTable).where(eq(usersTable.id, id)).limit(1);
    // if (rows.length === 0) return null;
    // return rowToEntity(rows[0] as UserRow);
    void id;
    void rowToEntity;
    throw new Error(
      "DrizzleUserRepository.findById: stub offline — installer drizzle-orm pour activer",
    );
  }

  async findByWorkspace(workspaceId: string): Promise<User[]> {
    // TODO(drizzle): remplacer par la requête Drizzle réelle :
    // const rows = await (this.db as NodePgDatabase)
    //   .select().from(usersTable).where(eq(usersTable.workspace_id, workspaceId));
    // return rows.map(r => rowToEntity(r as UserRow));
    void workspaceId;
    throw new Error(
      "DrizzleUserRepository.findByWorkspace: stub offline — installer drizzle-orm pour activer",
    );
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: UserRepository = new DrizzleUserRepository({} as DrizzleDb);
void _conformance;
