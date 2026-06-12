-- Service: Campaign (schéma "campaign") — V1
CREATE SCHEMA IF NOT EXISTS campaign;

CREATE TABLE campaign.campaigns (
  id            uuid PRIMARY KEY,
  workspace_id  uuid NOT NULL,
  name          text NOT NULL,
  status        text NOT NULL DEFAULT 'draft',
  scheduled_at  timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE campaign.campaign_recipients (
  id            bigserial PRIMARY KEY,
  campaign_id   uuid NOT NULL REFERENCES campaign.campaigns(id),
  recipient     text NOT NULL
);

CREATE TABLE campaign.campaign_runs (
  id            bigserial PRIMARY KEY,
  campaign_id   uuid NOT NULL REFERENCES campaign.campaigns(id),
  started_at    timestamptz,
  finished_at   timestamptz
);
