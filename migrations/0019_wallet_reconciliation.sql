-- Service: Wallet (schéma "wallet") — M-018 Réconciliation paiement (débloque webhooks OM/MTN)
-- Migration STRICTEMENT additive : ADD COLUMN IF NOT EXISTS, CREATE INDEX IF NOT EXISTS
-- uniquement. Aucune colonne existante modifiée, supprimée ou renommée.
-- La migration 0002_wallet.sql N'EST PAS modifiée.
--
-- ══════════════════════════════════════════════════════════════════════════════
-- CONTEXTE (preuve QA M-019 / architect-reviewer)
--
-- `wallet.wallet_transactions` (0002) n'a ni status, ni reference_id. Colonnes
-- réelles : id bigserial, workspace_id uuid, kind text, amount bigint,
-- message_id uuid (nullable), created_at timestamptz.
-- Conséquence constatée : src/api/webhooks_om.go et webhooks_mtn.go (callbacks
-- Orange Money / MTN MoMo) valident la signature HMAC, parsent le payload,
-- logguent et répondent 200 — mais n'écrivent RIEN en base : le
--   UPDATE wallet.wallet_transactions SET status = $1 WHERE reference_id = $2
-- demandé par M-018 est structurellement impossible avec ce schéma. Les deux
-- handlers documentent explicitement cette dette (commentaires "DETTE SCHEMA
-- CRITIQUE (M-018)") et laissent l'écriture en TODO(production).
--
-- ══════════════════════════════════════════════════════════════════════════════
-- DÉCISIONS DE MODÉLISATION
--
-- 1. status text NOT NULL DEFAULT 'completed'
--    Machine à états minimale de la transaction de paiement :
--      pending → completed | failed
--    (le service Go câblera les valeurs exactes ; le schéma reste du texte
--    libre, cohérent avec le pattern déjà en place pour `kind`, `channel_strategy`
--    (0016), `strategy` (routing.routing_rules) — aucune colonne de ce dépôt
--    n'utilise de CHECK/ENUM pour un statut texte).
--    DEFAULT 'completed' (et non 'pending') : les lignes déjà existantes dans
--    wallet.wallet_transactions ont TOUTES été insérées par des chemins qui
--    appliquent immédiatement l'effet sur le solde dans la même transaction
--    SQL (cf. src/api/wallet_topup.go, wallet_deduct.go — commentaires
--    "kind = 'credit'/'debit' … valeur reconnue, migration 0002" : l'INSERT
--    wallet_transactions est fait dans le même tx.Exec que l'UPDATE du solde
--    wallets.balance). Il n'existe donc, à ce jour, AUCUNE transaction en
--    base qui soit réellement "en attente" — toutes sont déjà des opérations
--    comptables terminées. Rétro-marquer l'historique 'completed' est donc
--    sémantiquement exact (pas d'invention de statut), et évite de faire
--    apparaître soudainement tout l'historique comme "en attente" pour les
--    lectures existantes (GET /wallets/{workspaceId}/transactions,
--    src/api/wallet_transactions.go). Les NOUVELLES lignes créées par un flux
--    asynchrone (top-up Mobile Money en attente de callback OM/MTN) devront
--    explicitement insérer status='pending' avant callback — cf. contrat
--    ci-dessous pour le go-engineer.
--
-- 2. reference_id text NULL
--    Référence externe de l'opérateur de paiement (Orange Money
--    "transaction_id", MTN MoMo "reference_id"/"external_id") permettant la
--    réconciliation du callback avec la transaction Fleece initiée :
--      UPDATE wallet.wallet_transactions SET status = $1 WHERE reference_id = $2
--    Nullable : les transactions historiques et les transactions non liées à
--    un paiement externe (ex. refund manuel, débit d'envoi de message) n'ont
--    et n'auront jamais de référence opérateur. Texte libre (pas de FK,
--    identifiant propriétaire à chaque opérateur, formats hétérogènes —
--    numérique chez MTN, alphanumérique chez OM).
--
-- 3. INDEX SUR reference_id — PARTIEL, NON-UNIQUE
--    Chemin de lecture chaud du callback paiement (webhooks_om.go,
--    webhooks_mtn.go) :
--      UPDATE wallet.wallet_transactions SET status = $1 WHERE reference_id = $2
--    Décision UNIQUE vs non-UNIQUE : NON-UNIQUE retenu, par symétrie avec la
--    décision prise pour messaging.messages.external_id (0018) : rien ne
--    garantit qu'un opérateur ne réutilise jamais un identifiant de
--    transaction (ex. rejeu de sandbox, migration de compte marchand), et une
--    contrainte UNIQUE romprait l'ingestion d'un webhook qui doit toujours
--    répondre 200 (principe déjà appliqué dans webhooks_om.go/webhooks_mtn.go
--    : "on ne peut pas echouer sur un callback paiement — risque de replays
--    infinis"). L'unicité applicative (1 reference_id = 1 topup) reste
--    assurée au niveau service (le go-engineer doit vérifier qu'au plus une
--    ligne 'pending' correspond à un reference_id avant de la faire passer à
--    'completed'/'failed', et journaliser sans échouer si plusieurs lignes
--    matchent — cf. contrat ci-dessous).
--    PARTIEL (WHERE reference_id IS NOT NULL) : reference_id est NULL pour
--    toute transaction non liée à un paiement d'opérateur externe (débit
--    d'envoi de message, refund interne) — indexer les NULL n'apporte aucune
--    valeur de lecture (les webhooks ne recherchent jamais reference_id IS
--    NULL) et alourdit inutilement l'index à mesure que le volume de
--    transactions grandit (NFR gros volumes, PRD §14).
--
-- ISOLATION D11 : aucune FK cross-schéma. Migration ne touche que le schéma
-- `wallet` (0018_messaging_dlr_correlation.sql traite `messaging` séparément).
-- ══════════════════════════════════════════════════════════════════════════════

CREATE SCHEMA IF NOT EXISTS wallet;

-- ─── 1. Enrichissement de wallet.wallet_transactions ──────────────────────────

ALTER TABLE wallet.wallet_transactions
  ADD COLUMN IF NOT EXISTS status       text NOT NULL DEFAULT 'completed',
  ADD COLUMN IF NOT EXISTS reference_id text;

-- ─── 2. Index de réconciliation paiement ───────────────────────────────────────

-- Chemin de lecture chaud des callbacks OM/MTN (M-018) :
-- UPDATE wallet.wallet_transactions SET status = $1 WHERE reference_id = $2
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_reference_id
  ON wallet.wallet_transactions (reference_id)
  WHERE reference_id IS NOT NULL;
