-- Service: Provider (schéma "provider") — persistance du statut DLR (dette T-xxx)
-- Migration STRICTEMENT additive : aucune colonne existante n'est modifiée ni supprimée.
-- external_id existe déjà dans 0005_provider.sql (text NOT NULL) → non redéclarée.

ALTER TABLE provider.provider_messages
  ADD COLUMN IF NOT EXISTS status     VARCHAR(50)  NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ;
