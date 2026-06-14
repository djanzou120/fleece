-- Service: Webhook (schéma "webhook") — ajout des colonnes manquantes (dette T-005)
ALTER TABLE webhook.webhook_endpoints
  ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE webhook.webhook_deliveries
  ADD COLUMN IF NOT EXISTS payload       BYTEA,
  ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ;
