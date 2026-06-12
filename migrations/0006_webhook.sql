-- Service: Webhook (schéma "webhook")
CREATE SCHEMA IF NOT EXISTS webhook;

CREATE TABLE webhook.webhook_endpoints (
  id            uuid PRIMARY KEY,
  workspace_id  uuid NOT NULL,
  url           text NOT NULL,
  secret        text NOT NULL,
  events        text[] NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE webhook.webhook_deliveries (
  id            bigserial PRIMARY KEY,
  endpoint_id   uuid NOT NULL REFERENCES webhook.webhook_endpoints(id),
  event         text NOT NULL,
  status        text NOT NULL,
  attempts      int NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now()
);
