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
// Mappage entité domaine ↔ lignes SQL (0001 + 0011_identity_schema.sql)
// Colonnes 0001 : id, workspace_id, email, created_at
// Colonnes 0011 : name, role
// ---------------------------------------------------------------------------

/**
 * Ligne telle que retournée par identity.users.
 * Colonnes 0001 : id, workspace_id, email, created_at
 * Colonnes 0011 : name (varchar 255, NOT NULL DEFAULT ''), role (varchar 50, NOT NULL DEFAULT 'member')
 */
interface UserRow {
  id: string;
  workspace_id: string;
  email: string;
  /** name : VARCHAR(255) NOT NULL DEFAULT '' — disponible depuis 0011 */
  name: string;
  /** role : VARCHAR(50) NOT NULL DEFAULT 'member' — disponible depuis 0011 */
  role: string;
  created_at: Date;
}

function rowToEntity(row: UserRow): User {
  return {
    id: row.id,
    email: row.email,
    workspaceId: row.workspace_id,
    // name : persisté depuis 0011
    name: row.name,
    // role : persisté depuis 0011 ; cast sûr car la base enforce 'owner'|'member'
    role: (row.role === "owner" ? "owner" : "member") as User["role"],
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
    //   name: user.name,
    //   role: user.role,
    //   created_at: user.createdAt,
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
