-- Service: Analytics (schéma "analytics") — V1
CREATE SCHEMA IF NOT EXISTS analytics;

-- Agrégat alimenté par les événements (vues matérialisées à venir).
CREATE TABLE analytics.message_daily (
  day          date NOT NULL,
  workspace_id uuid NOT NULL,
  country      text NOT NULL,
  channel      text NOT NULL,
  sent         bigint NOT NULL DEFAULT 0,
  delivered    bigint NOT NULL DEFAULT 0,
  failed       bigint NOT NULL DEFAULT 0,
  cost         bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (day, workspace_id, country, channel)
);
