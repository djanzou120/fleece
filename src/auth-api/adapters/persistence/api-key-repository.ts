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
// Mappage entité domaine ↔ lignes 0001_identity.sql
// Colonnes disponibles : id, workspace_id, hashed_key, status, created_at, revoked_at
// ---------------------------------------------------------------------------

/** Ligne telle que retournée par identity.api_keys (colonnes 0001 exactes). */
interface ApiKeyRow {
  id: string;
  workspace_id: string;
  hashed_key: string;
  status: string; // 'active' | 'revoked'
  created_at: Date;
  revoked_at: Date | null;
  // Colonnes absentes de 0001 :
  // name         : TODO(schema): aligner migration avant production
  // prefix       : TODO(schema): aligner migration avant production
  // permissions  : TODO(schema): aligner migration avant production
  // last_used_at : TODO(schema): aligner migration avant production
  // expires_at   : TODO(schema): aligner migration avant production
}

function rowToEntity(row: ApiKeyRow): ApiKey {
  return new ApiKey(
    row.id,
    row.workspace_id,
    row.hashed_key,
    // prefix      : TODO(schema): absent de 0001 — chaîne vide par défaut
    "",
    // name        : TODO(schema): absent de 0001 — chaîne vide par défaut
    "",
    // permissions : TODO(schema): absent de 0001 — tableau vide par défaut
    [],
    // active : dérivé de status
    row.status === "active",
    // lastUsedAt  : TODO(schema): absent de 0001 — null par défaut
    null,
    // expiresAt   : TODO(schema): absent de 0001 — null par défaut
    null,
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
    //   created_at: key.createdAt,
    //   revoked_at: key.revokedAt,
    //   // name, prefix, permissions, last_used_at, expires_at : TODO(schema)
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

  async updateLastUsed(id: string, at: Date): Promise<void> {
    // TODO(schema): last_used_at absent de 0001 — no-op jusqu'à la migration.
    // TODO(drizzle): quand la colonne existe :
    // await (this.db as NodePgDatabase)
    //   .update(apiKeysTable)
    //   .set({ last_used_at: at })
    //   .where(eq(apiKeysTable.id, id));
    void id;
    void at;
    // No-op intentionnel : la colonne last_used_at n'existe pas encore dans 0001.
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
