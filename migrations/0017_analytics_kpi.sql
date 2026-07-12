-- Service: Analytics (schéma "analytics") — T-019 KPIs & Vues matérialisées
-- Migration STRICTEMENT additive : ADD COLUMN IF NOT EXISTS, CREATE MATERIALIZED VIEW IF NOT EXISTS,
-- CREATE [UNIQUE] INDEX IF NOT EXISTS uniquement. Aucune colonne existante modifiée, supprimée
-- ou renommée. La migration 0009_analytics.sql n'est PAS modifiée.
--
-- ══════════════════════════════════════════════════════════════════════════════
-- DÉCISIONS DE MODÉLISATION
--
-- 1. LATENCE — agrégat additif (somme + dénominateur), pas de moyenne stockée
--    La table message_daily ne portait pas la latence de livraison. On ajoute
--    `delivery_latency_ms_sum bigint DEFAULT 0` : chaque événement
--    message.delivered incrémente la somme de la latence (en ms) de ce message.
--    La moyenne est calculée à la lecture : delivery_latency_ms_sum / NULLIF(delivered, 0).
--    Pourquoi une somme et non une moyenne ?
--      - Une moyenne est un agrégat NON-ADDITIF : la moyenne de deux moyennes
--        n'est pas la moyenne globale sauf si les échantillons sont de même taille.
--      - Une somme est un agrégat ADDITIF : les résumés peuvent être cumulés,
--        agrégés par période ou par dimension sans perte de précision.
--      - Le dénominateur (nombre de messages livrés) est déjà présent dans la
--        colonne `delivered`, qui fait partie de la PK dimensionnelle.
--    Corollaire : le service Go (T-020) incrémente delivery_latency_ms_sum et
--    delivered dans la même instruction UPSERT. La vue calcule la moyenne.
--
-- 2. VUE MATÉRIALISÉE analytics.kpi_daily — KPIs dimensionnels quotidiens
--    Une seule vue suffit, suffisamment dimensionnée (workspace, country, channel,
--    day) pour couvrir tous les filtres produit et business :
--      - Filtres produit : taux délivrabilité, taux échec, coût moyen/msg, latence moy.
--      - Filtres business : revenus (cost) par pays, revenus par canal.
--    Pourquoi une seule vue et non plusieurs ?
--      - La table source message_daily est déjà l'agrégat minimal (une ligne par
--        dimension par jour). Une MV distincte "par pays" ou "par canal" serait
--        un simple rollup de kpi_daily : GROUP BY en SQL à la requête suffit.
--      - Minimiser les objets matérialisés réduit le coût de refresh total.
--      - Si un rollup pays ou canal devient une requête critique (latence < 50 ms
--        sur plusieurs années de données), ajouter une 2e MV pré-agrégée sera
--        une migration additive triviale.
--    Colonnes calculées (métriques dérivées ; protection division par zéro via
--    CASE WHEN <dénominateur> > 0 dans le DDL de la vue ci-dessous) :
--      - delivery_rate    = delivered / sent (0 si sent=0)               → taux de délivrabilité
--      - failure_rate     = failed    / sent (0 si sent=0)               → taux d'échec
--      - avg_cost_per_msg = cost      / sent (0 si sent=0)               → coût moyen par message
--      - avg_delivery_ms  = delivery_latency_ms_sum / delivered
--                           (NULL si delivered=0)                        → temps moyen de livraison
--    Type des ratios : NUMERIC(10,6) pour une précision suffisante sans overflow.
--    Type des latences : NUMERIC(16,3) (ms avec décimales pour les agrégats précis).
--
-- 3. INDEX SUR message_daily — performance des requêtes filtrées (NFR gros volumes)
--    Les lectures analytiques filtrent systématiquement sur workspace_id + une
--    dimension temporelle ou géographique. Trois index couvrent les patterns
--    identifiés dans PRD §14 (filtres : période, pays, canal, workspace) :
--      a. (workspace_id, day)     → séries temporelles par workspace
--      b. (workspace_id, country) → revenus par pays (business KPI)
--      c. (workspace_id, channel) → revenus par canal (business KPI)
--
-- 4. INDEX UNIQUE sur kpi_daily — requis pour REFRESH CONCURRENTLY
--    PostgreSQL exige un index UNIQUE sur toute MV pour autoriser l'option
--    CONCURRENTLY du REFRESH (sans lock table, compatible avec des lectures
--    simultanées). L'index porte sur les colonnes de dimension (day, workspace_id,
--    country, channel), identiques à la PK de la table source.
--    ATTENTION (vigilance T-020) : le premier REFRESH doit être non-concurrent
--    (REFRESH MATERIALIZED VIEW analytics.kpi_daily) pour peupler la MV avant
--    que l'index UNIQUE soit utilisable. Les refreshs suivants peuvent être
--    CONCURRENTLY.
--
-- 5. REFRESH STRATEGY — refresh périodique via job Go (T-020)
--    La MV se rafraîchit via :
--      REFRESH MATERIALIZED VIEW CONCURRENTLY analytics.kpi_daily;
--    Le service Go analytics (T-020) aura un job périodique (ex. toutes les 5 min)
--    qui exécute cette instruction. CONCURRENTLY = aucun lock sur la MV en lecture
--    pendant le refresh (cohérent avec la cible 99.9 %). Nécessite :
--      (a) L'index UNIQUE défini au §4 ci-dessus.
--      (b) Un premier refresh initial NON-concurrent (peupler la MV une 1re fois).
--    Détail Go (T-020) : le job de refresh peut utiliser un context avec timeout
--    conservateur (ex. 30 s) et ignorer l'erreur si la MV est vide (1er boot).
--
-- ISOLATION D11 : AUCUNE FK cross-schéma, AUCUNE requête cross-schéma.
--   La MV lit uniquement analytics.message_daily (même schéma).
--   Les colonnes workspace_id, country, channel sont des types libres (uuid/text)
--   sans contrainte REFERENCES vers d'autres schémas.
-- ══════════════════════════════════════════════════════════════════════════════

CREATE SCHEMA IF NOT EXISTS analytics;

-- ─── 1. Ajout de la latence sur analytics.message_daily ──────────────────────
-- Somme des latences de livraison (ms) pour les messages livrés sur cette ligne.
-- Moyenne = delivery_latency_ms_sum / NULLIF(delivered, 0) — calculée à la lecture.
-- Agrégat ADDITIF : le consumer T-020 incrémente cette somme à chaque événement
-- message.delivered, dans le même UPSERT que les colonnes sent/delivered/cost.
ALTER TABLE analytics.message_daily
  ADD COLUMN IF NOT EXISTS delivery_latency_ms_sum bigint NOT NULL DEFAULT 0;

-- ─── 2. Index de performance sur analytics.message_daily ─────────────────────

-- Séries temporelles par workspace (GET /kpi?workspace_id=&from=&to=).
CREATE INDEX IF NOT EXISTS idx_message_daily_workspace_day
  ON analytics.message_daily (workspace_id, day);

-- Agrégation par pays pour un workspace (revenus par pays — PRD §14 business KPI).
-- Pattern : WHERE workspace_id = $1 GROUP BY country
CREATE INDEX IF NOT EXISTS idx_message_daily_workspace_country
  ON analytics.message_daily (workspace_id, country);

-- Agrégation par canal pour un workspace (revenus par canal — PRD §14 business KPI).
-- Pattern : WHERE workspace_id = $1 GROUP BY channel
CREATE INDEX IF NOT EXISTS idx_message_daily_workspace_channel
  ON analytics.message_daily (workspace_id, channel);

-- ─── 3. Vue matérialisée analytics.kpi_daily ─────────────────────────────────
-- KPIs dimensionnels quotidiens : taux de délivrabilité, taux d'échec,
-- coût moyen par message, temps moyen de livraison + volumes bruts.
-- Dimensions : (day, workspace_id, country, channel) — identiques à la PK source.
-- Lue par le service Go analytics (T-020) : use cases GetKpis, GetTimeSeries.
--
-- Contrat de lecture pour T-020 :
--   SELECT day, workspace_id, country, channel,
--          sent, delivered, failed, cost,
--          delivery_rate,   -- NUMERIC(10,6) : delivered/sent (0 si sent=0)
--          failure_rate,    -- NUMERIC(10,6) : failed/sent    (0 si sent=0)
--          avg_cost_per_msg,-- NUMERIC(10,6) : cost/sent      (0 si sent=0) [centimes]
--          avg_delivery_ms  -- NUMERIC(16,3) : latency_sum/delivered (NULL si delivered=0)
--   FROM analytics.kpi_daily
--   WHERE workspace_id = $1
--     AND day BETWEEN $2 AND $3
--     [AND country = $4]
--     [AND channel = $5]
--   ORDER BY day;
--
-- Pour les agrégats business (revenus par pays, par canal) :
--   SELECT country, SUM(cost) AS total_cost FROM analytics.kpi_daily
--   WHERE workspace_id = $1 AND day BETWEEN $2 AND $3
--   GROUP BY country ORDER BY total_cost DESC;
--
CREATE MATERIALIZED VIEW IF NOT EXISTS analytics.kpi_daily AS
SELECT
  day,
  workspace_id,
  country,
  channel,
  sent,
  delivered,
  failed,
  cost,
  -- Taux de délivrabilité (produit) : messages livrés / messages envoyés
  CASE WHEN sent > 0
    THEN ROUND((delivered::numeric / sent::numeric), 6)
    ELSE 0::numeric
  END AS delivery_rate,
  -- Taux d'échec (produit) : messages échoués / messages envoyés
  CASE WHEN sent > 0
    THEN ROUND((failed::numeric / sent::numeric), 6)
    ELSE 0::numeric
  END AS failure_rate,
  -- Coût moyen par message envoyé (centimes) — business KPI
  CASE WHEN sent > 0
    THEN ROUND((cost::numeric / sent::numeric), 6)
    ELSE 0::numeric
  END AS avg_cost_per_msg,
  -- Temps moyen de livraison (ms) — produit KPI ; NULL si aucun message livré
  CASE WHEN delivered > 0
    THEN ROUND((delivery_latency_ms_sum::numeric / delivered::numeric), 3)
    ELSE NULL
  END AS avg_delivery_ms
FROM analytics.message_daily
WITH NO DATA;

-- ─── 4. Index UNIQUE sur kpi_daily (requis pour REFRESH CONCURRENTLY) ─────────
-- PostgreSQL exige un index UNIQUE pour REFRESH MATERIALIZED VIEW CONCURRENTLY.
-- L'index porte sur les 4 colonnes de dimension = clé naturelle de la MV
-- (héritée de la PK de message_daily).
-- ATTENTION T-020 : le premier REFRESH doit être non-concurrent (MV vide) ;
-- les refreshs suivants peuvent être CONCURRENTLY.
CREATE UNIQUE INDEX IF NOT EXISTS uq_kpi_daily_dimensions
  ON analytics.kpi_daily (day, workspace_id, country, channel);
