-- Service: Messaging (schéma "messaging") — M-020 Corrélation DLR (débloque core-processor)
-- Migration STRICTEMENT additive : ADD COLUMN IF NOT EXISTS, CREATE INDEX IF NOT EXISTS
-- uniquement. Aucune colonne existante modifiée, supprimée ou renommée.
-- La migration 0003_messaging.sql N'EST PAS modifiée.
--
-- ══════════════════════════════════════════════════════════════════════════════
-- CONTEXTE (preuve QA M-019 / architect-reviewer)
--
-- `messaging.messages` (0003) n'a ni provider_id, ni external_id, ni cost.
-- Conséquence constatée : src/api/messages_send.go (insertMessage) reçoit ces
-- 3 valeurs depuis le résultat du provider (ProviderResult{Status, ExternalID,
-- Cost}) puis les jette (`_ = providerID; _ = externalID; _ = cost`) alors que
-- la réponse HTTP POST /messages les expose déjà (sendMessageResponse.Cost,
-- .ProviderID). Aucune corrélation DLR n'est possible : webhooks_telegram.go
-- doit se rabattre sur un champ non standard "fleece_message_id" au lieu de
-- retrouver le message par l'external_id retourné par le provider à l'envoi.
-- Ceci bloque M-020 (core-processor, consumers "message.delivered" /
-- "message.failed" qui doivent recorréler par external_id pour les providers
-- qui ne renvoient pas l'UUID Fleece dans leur callback, ex. Telegram, SMS,
-- WhatsApp).
--
-- ══════════════════════════════════════════════════════════════════════════════
-- DÉCISIONS DE MODÉLISATION
--
-- 1. provider_id text NULL
--    Identifiant du provider Fleece ayant traité l'envoi (ex. "telegram-bot",
--    "whatsapp-meta", "sms-twilio" — cf. registry src/provider ou constantes
--    src/api). Texte libre, pas de FK cross-schéma (D11 isolation) : le
--    registre des providers vit dans le schéma `provider`. Nullable : les
--    messages historiques (avant cette migration) n'ont pas cette donnée et
--    les messages en échec précoce (avant sélection du provider) peuvent ne
--    jamais l'obtenir.
--
-- 2. external_id text NULL
--    Identifiant du message côté provider externe (ex. message_id Telegram,
--    SID Twilio, message ID WhatsApp Cloud API). C'EST la clé de corrélation
--    DLR : les webhooks/consumers "message.delivered"/"message.failed"
--    (M-020) reçoivent cet identifiant dans leur payload et doivent retrouver
--    la ligne messaging.messages correspondante par
--      UPDATE messaging.messages SET status = $1 WHERE external_id = $2
--    Nullable : rempli uniquement après un Send() provider réussi (résultat
--    ProviderResult.ExternalID) ; NULL pour les messages en échec avant envoi
--    ou pour les providers qui ne renvoient aucun identifiant.
--
-- 3. cost bigint NULL — UNITÉ : CENTIMES
--    Coût réel de l'envoi retourné par le provider (ProviderResult.Cost),
--    déjà exposé par sendMessageResponse.Cost côté API mais jamais persisté
--    (dette M-019). Type bigint aligné sur la convention monétaire du dépôt :
--      - wallet.wallet_transactions.amount bigint (migration 0002)
--      - campaign.campaigns.estimated_cost bigint (migration 0016, commentaire
--        "Unité : centimes … cohérent avec wallet.wallets.balance")
--      - analytics.message_daily.cost (consommé en centimes, cf. 0017 avg_cost_per_msg)
--    bigint (et non int) : cohérence de type avec les colonnes monétaires
--    existantes, marge suffisante pour des devises à forte valeur nominale
--    (XAF) sans risque de dépassement int32. Nullable : les messages non
--    encore envoyés ou en échec précoce n'ont pas de coût connu ; NULL ≠ 0
--    (0 = coût réellement nul, ex. Telegram gratuit, cf. D27/T-014).
--
-- 4. INDEX SUR external_id — PARTIEL, NON-UNIQUE
--    Chemin de lecture chaud des webhooks/workers DLR (M-020) :
--      WHERE external_id = $1
--    Décision UNIQUE vs non-UNIQUE : NON-UNIQUE retenu. Bien que l'external_id
--    d'un provider donné soit normalement unique côté provider, rien ne
--    garantit l'unicité GLOBALE inter-providers (deux providers différents
--    pourraient théoriquement générer le même identifiant, ex. un entier
--    séquentiel côté Telegram et un SID Twilio qui collisionnent par hasard).
--    Une contrainte UNIQUE globale sur external_id seul romprait sur ce cas
--    et bloquerait l'ingestion DLR (violation de contrainte) — inacceptable
--    pour un webhook qui doit toujours répondre 200. Voir point 5 pour la
--    combinaison (provider_id, external_id) qui, elle, est logiquement unique.
--    PARTIEL (WHERE external_id IS NOT NULL) : external_id est NULL pour tout
--    message non encore envoyé ou en échec précoce (cf. point 2) — indexer
--    les NULL n'apporterait aucune valeur de lecture (aucun webhook ne
--    recherche external_id IS NULL) et alourdirait inutilement l'index à
--    mesure que le volume de messages grandit (NFR gros volumes, PRD §14).
--
-- 5. INDEX COMPOSITE (provider_id, external_id) — PARTIEL, NON-UNIQUE
--    Complète l'index simple pour les consumers qui connaissent le provider
--    d'origine de l'événement (cas le plus fréquent : un consumer RabbitMQ
--    "message.delivered" dédié à un provider, ou un webhook par provider comme
--    webhooks_telegram.go/webhooks_om.go). Pattern :
--      WHERE provider_id = $1 AND external_id = $2
--    Ce couple EST logiquement unique (un provider donné n'attribue jamais le
--    même external_id à deux messages distincts) mais on NE POSE PAS de
--    contrainte UNIQUE dessus : certains providers peuvent renvoyer un DLR
--    dupliqué (retry réseau) avant que le premier INSERT ne soit committé, et
--    un webhook doit pouvoir écrire de façon idempotente (UPDATE, pas INSERT)
--    sans jamais échouer sur une contrainte — cohérent avec le principe
--    "Telegram/OM/MTN doivent toujours répondre 200" déjà appliqué dans
--    webhooks_telegram.go/webhooks_om.go/webhooks_mtn.go. PARTIEL pour la
--    même raison qu'au point 4 (external_id NULL = non pertinent en lecture).
--
-- ISOLATION D11 : aucune FK cross-schéma. provider_id reste un identifiant
-- texte libre (le référentiel provider vit dans le schéma `provider`).
-- ══════════════════════════════════════════════════════════════════════════════

CREATE SCHEMA IF NOT EXISTS messaging;

-- ─── 1. Enrichissement de messaging.messages ──────────────────────────────────

ALTER TABLE messaging.messages
  ADD COLUMN IF NOT EXISTS provider_id  text,
  ADD COLUMN IF NOT EXISTS external_id  text,
  ADD COLUMN IF NOT EXISTS cost         bigint;

-- ─── 2. Index de corrélation DLR ───────────────────────────────────────────────

-- Chemin de lecture chaud des webhooks/consumers DLR (M-020) qui ne
-- connaissent que l'identifiant externe du provider.
-- Pattern : WHERE external_id = $1
CREATE INDEX IF NOT EXISTS idx_messages_external_id
  ON messaging.messages (external_id)
  WHERE external_id IS NOT NULL;

-- Complète l'index simple pour les webhooks/consumers scoping par provider.
-- Pattern : WHERE provider_id = $1 AND external_id = $2
CREATE INDEX IF NOT EXISTS idx_messages_provider_external
  ON messaging.messages (provider_id, external_id)
  WHERE external_id IS NOT NULL;
