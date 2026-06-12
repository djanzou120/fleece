-- Service: Routing (schéma "routing")
CREATE SCHEMA IF NOT EXISTS routing;

CREATE TABLE routing.provider_pricing (
  id          bigserial PRIMARY KEY,
  provider    text NOT NULL,
  channel     text NOT NULL,
  country     text NOT NULL,
  cost        bigint NOT NULL
);

CREATE TABLE routing.provider_scores (
  provider    text NOT NULL,
  channel     text NOT NULL,
  score       int NOT NULL,
  PRIMARY KEY (provider, channel)
);

CREATE TABLE routing.routing_rules (
  workspace_id uuid PRIMARY KEY,
  strategy     text NOT NULL DEFAULT 'highest_delivery'
);
