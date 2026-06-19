-- Service: Routing (schéma "routing")
-- Dette 3 — Routing Fastest/Custom non fonctionnels
-- Ajout des colonnes manquantes pour les stratégies fastest et custom.
-- Migration strictement additive : ADD COLUMN IF NOT EXISTS uniquement.
CREATE SCHEMA IF NOT EXISTS routing;

ALTER TABLE routing.provider_scores
  ADD COLUMN IF NOT EXISTS avg_latency_ms INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS country        VARCHAR(10) NOT NULL DEFAULT '';

ALTER TABLE routing.routing_rules
  ADD COLUMN IF NOT EXISTS channel VARCHAR(50) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS config  JSONB;
