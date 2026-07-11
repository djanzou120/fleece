-- Service: Provider (schéma "provider") — T-013 Canal Telegram
-- Migration STRICTEMENT additive : CREATE SCHEMA IF NOT EXISTS, INSERT ... ON CONFLICT DO NOTHING.
-- Les fichiers 0005_provider.sql et 0012_provider_dlr_status.sql ne sont PAS modifiés.
--
-- ══════════════════════════════════════════════════════════════════════════════
-- DÉCISION CAPABILITIES (T-013 → T-014)
--   Aucune colonne `capabilities` n'est ajoutée à provider.providers.
--   Raison : les capabilities d'un canal (rich media, messages longs, absence de
--   template obligatoire pour Telegram vs WhatsApp) sont des constantes métier
--   connues à la compilation — elles relèvent de la couche adapter Go (couche 3,
--   fichier `telegram.go`, T-014), pas du schéma relationnel.
--   Stocker des capabilities en JSONB créerait un couplage schéma/logique sans
--   bénéfice (aucune requête SQL ne filtre par capability). Si une exploration
--   dynamique des capabilities devient nécessaire (V2), une colonne additive
--   `ADD COLUMN IF NOT EXISTS capabilities JSONB` pourra être ajoutée à ce
--   moment-là sans casser la présente migration.
--   → T-014 (adapter Telegram) : lire les capabilities directement depuis le code
--   Go, pas depuis la base.
--
-- DÉCISION CREDENTIALS (T-013 → T-014 + T-024 DevOps)
--   Option retenue : (a) réutilisation du pattern existant `provider.provider_credentials`
--   (`secret_enc bytea`). Le bot token Telegram est un secret PAR PROVIDER
--   (un bot = un token), pas par workspace. Ce pattern est cohérent avec whatsapp-meta
--   et sms-twilio (même table, même chiffrement AES-GCM côté service Go).
--   → AUCUN secret en clair dans cette migration. Le credential chiffré doit être
--     inséré hors migration (script d'init, Secret K8s monté en env, ou opération
--     DBA) selon la procédure de rotation des secrets (T-024).
--   Option (b) env/Secret K8s pur : écarté car incohérent avec le pattern existant
--   déjà en base ; romprait la symétrie avec whatsapp-meta/sms-twilio.
--   Option (c) table par workspace : écarté car un bot Telegram est global au
--   provider (pas par client/workspace). Si des bots multi-tenant deviennent
--   nécessaires, une table `provider.telegram_workspace_credentials` pourra être
--   ajoutée plus tard (migration additive).
--   → T-014 (adapter Telegram) : lire le token via `provider_credentials.secret_enc`
--     après déchiffrement AES-GCM (même chemin que whatsapp-meta / sms-twilio).
--   → T-024 (DevOps) : provisionner le Secret K8s pour le chiffrement + insérer
--     la ligne chiffrée dans `provider.provider_credentials` pour `provider_id = 'telegram-bot'`.
-- ══════════════════════════════════════════════════════════════════════════════

CREATE SCHEMA IF NOT EXISTS provider;

-- ─── Seed du provider Telegram ────────────────────────────────────────────────
-- Convention d'ID : {channel}-{vendor} (cohérent avec 'whatsapp-meta', 'sms-twilio').
-- L'unique vendor officiel Telegram est l'API Bot (api.telegram.org).
-- INSERT idempotent : ON CONFLICT (id) DO NOTHING.
INSERT INTO provider.providers (id, channel, enabled)
VALUES ('telegram-bot', 'telegram', true)
ON CONFLICT (id) DO NOTHING;
