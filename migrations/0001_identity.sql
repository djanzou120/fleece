-- Service: Identity (schéma "identity")
CREATE SCHEMA IF NOT EXISTS identity;

CREATE TABLE identity.workspaces (
  id          uuid PRIMARY KEY,
  name        text NOT NULL,
  country     text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE identity.users (
  id            uuid PRIMARY KEY,
  workspace_id  uuid NOT NULL REFERENCES identity.workspaces(id),
  email         text NOT NULL UNIQUE,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE identity.api_keys (
  id            uuid PRIMARY KEY,
  workspace_id  uuid NOT NULL REFERENCES identity.workspaces(id),
  hashed_key    text NOT NULL UNIQUE,
  status        text NOT NULL DEFAULT 'active',
  created_at    timestamptz NOT NULL DEFAULT now(),
  revoked_at    timestamptz
);

CREATE TABLE identity.audit_logs (
  id            bigserial PRIMARY KEY,
  workspace_id  uuid NOT NULL,
  action        text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now()
);
