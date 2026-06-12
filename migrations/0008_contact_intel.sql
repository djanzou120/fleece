-- Service: Contact Intelligence (schéma "contact_intel") — V1
CREATE SCHEMA IF NOT EXISTS contact_intel;

CREATE TABLE contact_intel.contacts (
  phone           text PRIMARY KEY,
  known_channels  text[] NOT NULL DEFAULT '{}',
  delivery_score  int NOT NULL DEFAULT 0,
  updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE contact_intel.contact_channel_history (
  id          bigserial PRIMARY KEY,
  phone       text NOT NULL,
  channel     text NOT NULL,
  success     boolean NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
