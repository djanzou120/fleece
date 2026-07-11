-- Service: Campaign (schéma "campaign") — T-017 Modèle d'exécution des campagnes
-- Migration STRICTEMENT additive : ADD COLUMN IF NOT EXISTS, CREATE INDEX IF NOT EXISTS,
-- CREATE UNIQUE INDEX IF NOT EXISTS uniquement. Aucune colonne existante modifiée, supprimée
-- ou renommée. La migration 0007_campaign.sql n'est pas touchée.
--
-- ══════════════════════════════════════════════════════════════════════════════
-- DÉCISIONS DE MODÉLISATION
--
-- 1. message_body text DEFAULT ''
--    Nullable écarté : une valeur vide signale explicitement que le contenu n'a
--    pas encore été rédigé (draft). Le service Go (T-018) doit valider que la
--    colonne n'est pas vide avant de passer status → 'scheduled' ou 'running'.
--    DEFAULT '' = cohérent avec le pattern texte libre du dépôt (routing.routing_rules.strategy).
--
-- 2. channel_strategy text DEFAULT 'lowest_cost'
--    Valeurs attendues (libres, text) : 'lowest_cost' | 'highest_delivery' |
--    'fastest' | 'custom'. Alignées sur routing.routing_rules.strategy et les
--    stratégies de routing (T-016). DEFAULT 'lowest_cost' = comportement le plus
--    prévisible économiquement pour une campagne de masse (cohérent TDD §4.7).
--
-- 3. estimated_cost bigint DEFAULT 0
--    Unité : centimes (plus petite unité monétaire), cohérent avec wallet.wallets.balance
--    et wallet.wallet_transactions.amount (tous deux bigint). Calculé par le service
--    Campaign avant lancement : sum(pricing × destinataires).
--
-- 4. currency text DEFAULT 'XAF'
--    Portée ici pour que l'estimation soit auto-portée (sans JOIN sur le wallet).
--    DEFAULT 'XAF' = devise primaire Afrique francophone (cohérent wallet.wallets.currency).
--    Le service Campaign peut lire la devise réelle depuis wallet.wallets lors de
--    l'estimation et la persister ici.
--
-- 5. rate_limit int DEFAULT 0
--    Sémantique : 0 = illimité. Valeur > 0 = messages/minute max pour ce run.
--    Le scheduler Go (T-018) lit cette colonne et lève/baisse son token bucket en
--    conséquence. DEFAULT 0 = illimité permet aux campagnes créées avant T-018
--    de fonctionner sans configuration de rate limit explicite.
--
-- 6. campaign_recipients.status text DEFAULT 'pending'
--    Machine à états par destinataire : pending → sent → delivered | failed.
--    Le service Campaign met à jour cette colonne via DLR (message.delivered /
--    message.failed) consommés depuis RabbitMQ. Permet de reprendre un run
--    interrompu en ignorant les lignes déjà 'sent' ou 'delivered'.
--
-- 7. UNIQUE (campaign_id, recipient) — déduplication exigée
--    Empêche l'insertion du même destinataire deux fois dans une campagne.
--    Index nommé `uq_campaign_recipients_campaign_recipient` (nommage explicite
--    pour les migrations rollback / introspection). T-018 peut utiliser
--    INSERT ... ON CONFLICT DO NOTHING pour les imports en masse.
--
-- 8. campaign_recipients.message_id uuid (nullable)
--    Corrèle la ligne au message injecté dans le pipeline Messaging.
--    Nullable : non connu à l'insertion, rempli après injection (T-018 usecase
--    SendCampaignBatch). Permet au service Campaign de retrouver le statut DLR
--    d'un message sans passer par l'API Messaging, et de corréler
--    message.delivered / message.failed → recipient.status. Pas de FK
--    cross-schéma (D11 isolation) : simple colonne libre uuid.
--
-- 9. campaign_runs.total_count int DEFAULT 0
--    Snapshot de la taille de la campagne au lancement du run (nombre de
--    destinataires actifs). Permet un suivi de progression sans COUNT dynamique :
--    progress = (sent_count + failed_count) / total_count. Utile pour le dashboard.
--
-- 10. Index sur campaigns(workspace_id) — lecture par workspace
--     Toutes les requêtes de listing (GET /campaigns?workspace_id=…) filtrent sur
--     cette colonne.
--
-- 11. Index sur campaigns(status, scheduled_at) — scheduler
--     Le scheduler interroge : WHERE status = 'scheduled' AND scheduled_at <= now()
--     Cet index composite couvre exactement ce pattern de lecture.
--
-- 12. Index sur campaign_recipients(campaign_id, status) — reprise de run
--     Chargement des destinataires 'pending' d'un run : WHERE campaign_id = $1
--     AND status = 'pending'. Compositing sur le FK + filtre de statut.
--
-- AUCUNE FK cross-schéma (D11) : message_id, workspace_id sont des colonnes
-- libres (uuid/text) sans contrainte REFERENCES vers d'autres schémas.
-- ══════════════════════════════════════════════════════════════════════════════

CREATE SCHEMA IF NOT EXISTS campaign;

-- ─── 1. Enrichissement de campaign.campaigns ──────────────────────────────────

ALTER TABLE campaign.campaigns
  ADD COLUMN IF NOT EXISTS message_body      text    NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS channel_strategy  text    NOT NULL DEFAULT 'lowest_cost',
  ADD COLUMN IF NOT EXISTS estimated_cost    bigint  NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS currency          text    NOT NULL DEFAULT 'XAF',
  ADD COLUMN IF NOT EXISTS rate_limit        int     NOT NULL DEFAULT 0;

-- ─── 2. Enrichissement de campaign.campaign_recipients ────────────────────────

ALTER TABLE campaign.campaign_recipients
  ADD COLUMN IF NOT EXISTS status     text NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS message_id uuid;

-- Déduplication : un destinataire ne peut être inscrit qu'une seule fois par campagne.
-- INSERT ... ON CONFLICT (campaign_id, recipient) DO NOTHING pour les imports en masse.
CREATE UNIQUE INDEX IF NOT EXISTS uq_campaign_recipients_campaign_recipient
  ON campaign.campaign_recipients (campaign_id, recipient);

-- ─── 3. Enrichissement de campaign.campaign_runs ──────────────────────────────

ALTER TABLE campaign.campaign_runs
  ADD COLUMN IF NOT EXISTS total_count      int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS sent_count       int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS delivered_count  int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS failed_count     int NOT NULL DEFAULT 0;

-- ─── 4. Index de lecture ──────────────────────────────────────────────────────

-- Listing des campagnes d'un workspace (GET /campaigns?workspace_id=…).
CREATE INDEX IF NOT EXISTS idx_campaigns_workspace_id
  ON campaign.campaigns (workspace_id);

-- Scheduler : campagnes planifiées prêtes à lancer.
-- Pattern : WHERE status = 'scheduled' AND scheduled_at <= now()
CREATE INDEX IF NOT EXISTS idx_campaigns_status_scheduled_at
  ON campaign.campaigns (status, scheduled_at);

-- Chargement des destinataires par statut pour reprise de run.
-- Pattern : WHERE campaign_id = $1 AND status = 'pending'
CREATE INDEX IF NOT EXISTS idx_campaign_recipients_campaign_status
  ON campaign.campaign_recipients (campaign_id, status);
