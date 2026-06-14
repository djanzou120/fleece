// Couche 3 — Adapter persistence : implémentation de ApiKeyRepository avec Drizzle.
// Drizzle est confiné dans adapters/persistence/ et infrastructure/db/.
//
// OFFLINE STUB : drizzle-orm non installé.
// TODO: import { eq } from 'drizzle-orm'
// TODO: import { NodePgDatabase } from 'drizzle-orm/node-postgres'

import { ApiKey } from "../../domain/api-key.js";
import { ApiKeyRepository } from "../../application/ports/output/repositories.js";
import { DrizzleDb } from "./workspace-repository.js";

// ---------------------------------------------------------------------------
// Mappage entité domaine ↔ lignes SQL (0001 + 0011_identity_schema.sql)
// Colonnes 0001 : id, workspace_id, hashed_key, status, created_at, revoked_at
// Colonnes 0011 : name, prefix, permissions (TEXT[]), last_used_at, expires_at
// ---------------------------------------------------------------------------

/**
 * Ligne telle que retournée par identity.api_keys.
 * Colonnes 0001 : id, workspace_id, hashed_key, status, created_at, revoked_at
 * Colonnes 0011 :
 *   name         : VARCHAR(255) NOT NULL DEFAULT ''
 *   prefix       : VARCHAR(20)  NOT NULL DEFAULT ''
 *   permissions  : TEXT[]       NOT NULL DEFAULT '{}'
 *   last_used_at : TIMESTAMPTZ  nullable
 *   expires_at   : TIMESTAMPTZ  nullable
 */
interface ApiKeyRow {
  id: string;
  workspace_id: string;
  hashed_key: string;
  status: string; // 'active' | 'revoked'
  /** name : VARCHAR(255) NOT NULL DEFAULT '' — disponible depuis 0011 */
  name: string;
  /** prefix : VARCHAR(20) NOT NULL DEFAULT '' — disponible depuis 0011 */
  prefix: string;
  /** permissions : TEXT[] NOT NULL DEFAULT '{}' — disponible depuis 0011 */
  permissions: string[];
  /** last_used_at : TIMESTAMPTZ nullable — disponible depuis 0011 */
  last_used_at: Date | null;
  /** expires_at : TIMESTAMPTZ nullable — disponible depuis 0011 */
  expires_at: Date | null;
  created_at: Date;
  revoked_at: Date | null;
}

function rowToEntity(row: ApiKeyRow): ApiKey {
  return new ApiKey(
    row.id,
    row.workspace_id,
    row.hashed_key,
    // prefix : persisté depuis 0011
    row.prefix,
    // name : persisté depuis 0011
    row.name,
    // permissions : persisté depuis 0011
    row.permissions,
    // active : dérivé de status
    row.status === "active",
    // last_used_at : persisté depuis 0011
    row.last_used_at,
    // expires_at : persisté depuis 0011
    row.expires_at,
    row.created_at,
    row.revoked_at,
  );
}

// ---------------------------------------------------------------------------
// Implémentation
// ---------------------------------------------------------------------------

export class DrizzleApiKeyRepository implements ApiKeyRepository {
  constructor(private readonly db: DrizzleDb) {}

  async save(key: ApiKey): Promise<void> {
    // TODO(drizzle): remplacer par l'insert Drizzle réel :
    // await (this.db as NodePgDatabase).insert(apiKeysTable).values({
    //   id: key.id,
    //   workspace_id: key.workspaceId,
    //   hashed_key: key.keyHash,
    //   status: key.active ? "active" : "revoked",
    //   name: key.name,
    //   prefix: key.prefix,
    //   permissions: key.permissions,
    //   last_used_at: key.lastUsedAt,
    //   expires_at: key.expiresAt,
    //   created_at: key.createdAt,
    //   revoked_at: key.revokedAt,
    // });
    void key;
    throw new Error(
      "DrizzleApiKeyRepository.save: stub offline — installer drizzle-orm pour activer",
    );
  }

  async findById(id: string): Promise<ApiKey | null> {
    // TODO(drizzle): remplacer par la requête Drizzle réelle :
    // const rows = await (this.db as NodePgDatabase)
    //   .select().from(apiKeysTable).where(eq(apiKeysTable.id, id)).limit(1);
    // if (rows.length === 0) return null;
    // return rowToEntity(rows[0] as ApiKeyRow);
    void id;
    void rowToEntity;
    throw new Error(
      "DrizzleApiKeyRepository.findById: stub offline — installer drizzle-orm pour activer",
    );
  }

  async findByHash(hash: string): Promise<ApiKey | null> {
    // TODO(drizzle): remplacer par la requête Drizzle réelle :
    // const rows = await (this.db as NodePgDatabase)
    //   .select().from(apiKeysTable).where(eq(apiKeysTable.hashed_key, hash)).limit(1);
    // if (rows.length === 0) return null;
    // return rowToEntity(rows[0] as ApiKeyRow);
    void hash;
    throw new Error(
      "DrizzleApiKeyRepository.findByHash: stub offline — installer drizzle-orm pour activer",
    );
  }

  /**
   * Met à jour last_used_at pour la clé donnée.
   * Colonne disponible depuis 0011_identity_schema.sql.
   *
   * TODO(drizzle): décommenter l'UPDATE réel une fois drizzle-orm installé.
   */
  async updateLastUsed(id: string, at: Date): Promise<void> {
    // UPDATE réel (colonne last_used_at disponible depuis 0011) :
    // await (this.db as NodePgDatabase)
    //   .update(apiKeysTable)
    //   .set({ last_used_at: at })
    //   .where(eq(apiKeysTable.id, id));
    //
    // STUB offline — lève une erreur explicite (comme les autres méthodes mutantes).
    // Remplacé par le no-op lors du build offline uniquement pour ne pas bloquer les tests.
    void id;
    void at;
    throw new Error(
      "DrizzleApiKeyRepository.updateLastUsed: stub offline — installer drizzle-orm pour activer",
    );
  }

  async markRevoked(id: string, at: Date): Promise<void> {
    // TODO(drizzle): remplacer par la mise à jour Drizzle réelle :
    // await (this.db as NodePgDatabase)
    //   .update(apiKeysTable)
    //   .set({ status: "revoked", revoked_at: at })
    //   .where(eq(apiKeysTable.id, id));
    void id;
    void at;
    throw new Error(
      "DrizzleApiKeyRepository.markRevoked: stub offline — installer drizzle-orm pour activer",
    );
  }
}

// Assertion de conformité au port — erreur de compilation si l'interface diverge.
const _conformance: ApiKeyRepository = new DrizzleApiKeyRepository({} as DrizzleDb);
void _conformance;
