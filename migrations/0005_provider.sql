-- Service: Provider (schéma "provider")
CREATE SCHEMA IF NOT EXISTS provider;

CREATE TABLE provider.providers (
  id        text PRIMARY KEY,
  channel   text NOT NULL,
  enabled   boolean NOT NULL DEFAULT true
);

CREATE TABLE provider.provider_credentials (
  provider_id text PRIMARY KEY REFERENCES provider.providers(id),
  secret_enc  bytea NOT NULL
);

CREATE TABLE provider.provider_messages (
  internal_id uuid NOT NULL,
  provider_id text NOT NULL,
  external_id text NOT NULL,
  PRIMARY KEY (internal_id, provider_id)
);
