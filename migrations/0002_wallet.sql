-- Service: Wallet (schéma "wallet")
CREATE SCHEMA IF NOT EXISTS wallet;

CREATE TABLE wallet.wallets (
  workspace_id  uuid PRIMARY KEY,
  balance       bigint NOT NULL DEFAULT 0,
  currency      text NOT NULL DEFAULT 'XAF',
  updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Ledger append-only : debit | credit | refund
CREATE TABLE wallet.wallet_transactions (
  id            bigserial PRIMARY KEY,
  workspace_id  uuid NOT NULL,
  kind          text NOT NULL,
  amount        bigint NOT NULL,
  message_id    uuid,
  created_at    timestamptz NOT NULL DEFAULT now()
);
