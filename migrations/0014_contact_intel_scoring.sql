-- Service: Contact Intelligence (schéma "contact_intel") — T-011 Scoring de joignabilité
-- Migration STRICTEMENT additive : ADD COLUMN IF NOT EXISTS, CREATE TABLE IF NOT EXISTS,
-- CREATE INDEX IF NOT EXISTS uniquement. Aucune colonne existante modifiée ni supprimée.
-- La migration 0008_contact_intel.sql n'est pas touchée.
--
-- Choix de modélisation — scoring par canal :
--   Table dédiée `contact_channel_scores` (plutôt que colonne JSONB sur contacts).
--   Raisons : indexabilité par canal (routage T-016 "highest_delivery"), verrouillage
--   de ligne sur (phone, channel) lors des upserts incrémentaux (vs lock sur toute la
--   ligne contacts avec JSONB), lisibilité des requêtes de scoring, upsert ciblé sans
--   manipulation de document JSONB côté applicatif.
CREATE SCHEMA IF NOT EXISTS contact_intel;

-- ─── Enrichissement de contacts ────────────────────────────────────────────────
-- Colonnes géographiques, traçabilité du dernier envoi réussi et compteurs
-- agrégés permettant un score global rapide sans ré-agréger l'historique.
ALTER TABLE contact_intel.contacts
  ADD COLUMN IF NOT EXISTS country         VARCHAR(10)  NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS last_channel    text         NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS last_success_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS success_count   INTEGER      NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS failure_count   INTEGER      NOT NULL DEFAULT 0;

-- ─── Score de joignabilité par canal ───────────────────────────────────────────
-- Stocke le score agrégé (0-100) et la taille d'échantillon pour chaque
-- combinaison (contact, canal). Alimenté par le service contact-intelligence
-- à partir de contact_channel_history lors du recalcul incrémental du score.
CREATE TABLE IF NOT EXISTS contact_intel.contact_channel_scores (
  phone           text        NOT NULL,
  channel         text        NOT NULL,
  score           INTEGER     NOT NULL DEFAULT 0,
  sample_size     INTEGER     NOT NULL DEFAULT 0,
  last_success_at TIMESTAMPTZ,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (phone, channel)
);

-- ─── Index de scoring / lecture ────────────────────────────────────────────────

-- Filtrage géographique sur contacts (enrichissement T-012, segmentation).
CREATE INDEX IF NOT EXISTS idx_contacts_country
  ON contact_intel.contacts (country);

-- Tri par délivrabilité globale au sein d'un pays (routage "highest_delivery", T-016).
CREATE INDEX IF NOT EXISTS idx_contacts_country_score
  ON contact_intel.contacts (country, delivery_score DESC);

-- Chargement de tous les scores d'un contact (enrichissement T-012 : profil complet).
CREATE INDEX IF NOT EXISTS idx_channel_scores_phone
  ON contact_intel.contact_channel_scores (phone);

-- Recherche des contacts les plus joignables sur un canal donné (routage T-016).
CREATE INDEX IF NOT EXISTS idx_channel_scores_channel_score
  ON contact_intel.contact_channel_scores (channel, score DESC);

-- Calcul incrémental du score : agréger l'historique par (phone, canal).
CREATE INDEX IF NOT EXISTS idx_channel_history_phone_channel
  ON contact_intel.contact_channel_history (phone, channel);
