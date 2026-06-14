-- Service: Identity (schéma "identity") — ajout des colonnes manquantes (dette T-006)
-- Migration STRICTEMENT additive : aucune colonne existante n'est modifiée ni supprimée.

-- Table: identity.workspaces
ALTER TABLE identity.workspaces
  ADD COLUMN IF NOT EXISTS slug     VARCHAR(100) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS owner_id UUID,
  ADD COLUMN IF NOT EXISTS plan     VARCHAR(50)  NOT NULL DEFAULT 'free';

-- Contrainte UNIQUE sur slug : index nommé explicitement pour l'idempotence et la
-- lisibilité Atlas/pg_indexes. Préféré à ADD COLUMN ... UNIQUE (contrainte anonyme,
-- difficile à gérer en introspection et en rollback futur).
CREATE UNIQUE INDEX IF NOT EXISTS uq_workspaces_slug ON identity.workspaces (slug);

-- Table: identity.users
ALTER TABLE identity.users
  ADD COLUMN IF NOT EXISTS name VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS role VARCHAR(50)  NOT NULL DEFAULT 'member';

-- Table: identity.api_keys
ALTER TABLE identity.api_keys
  ADD COLUMN IF NOT EXISTS name         VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS prefix       VARCHAR(20)  NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS permissions  TEXT[]       NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS expires_at   TIMESTAMPTZ;
