-- Service: Messaging (schéma "messaging")
CREATE SCHEMA IF NOT EXISTS messaging;

CREATE TABLE messaging.messages (
  id            uuid PRIMARY KEY,
  workspace_id  uuid NOT NULL,
  recipient     text NOT NULL,
  content       text NOT NULL,
  status        text NOT NULL,
  channel       text,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE messaging.message_attempts (
  id            bigserial PRIMARY KEY,
  message_id    uuid NOT NULL REFERENCES messaging.messages(id),
  channel       text NOT NULL,
  provider      text NOT NULL,
  status        text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now()
);
