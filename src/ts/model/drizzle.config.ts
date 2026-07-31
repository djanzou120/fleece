// Configuration drizzle-kit pour @fleece/model (M-027).
//
// ┌───────────────────────────────────────────────────────────────────────────┐
// │ `out` NE DOIT JAMAIS POINTER SUR migrations/.                             │
// │                                                                           │
// │ migrations/ appartient à Atlas, seule source de vérité (CLAUDE.md). Ce    │
// │ répertoire de sortie est un ESPACE DE TRAVAIL JETABLE : il ne sert qu'à   │
// │ produire le DDL que M-028 compare aux migrations Atlas. Rien de ce qui y  │
// │ est écrit n'est appliqué à une base, ni commité (voir .gitignore).        │
// │                                                                           │
// │ Faire pointer `out` sur migrations/ ferait cohabiter deux générateurs sur │
// │ le même répertoire et casserait le checksum atlas.sum au premier          │
// │ `drizzle-kit generate`.                                                   │
// └───────────────────────────────────────────────────────────────────────────┘
//
// Il n'y a délibérément AUCUNE section `dbCredentials` : sans URL de base,
// `drizzle-kit push`/`migrate` — les commandes qui écrivent dans une vraie
// base — ne peuvent pas s'exécuter par accident. Seul `generate`, qui produit
// du SQL hors-ligne, fonctionne avec cette configuration.
//
// CHEMINS ANCRÉS SUR LA RACINE DU DÉPÔT : drizzle-kit résout `schema` et `out`
// depuis le RÉPERTOIRE COURANT, jamais depuis l'emplacement de ce fichier.
// Ils sont donc écrits relativement à la racine, et `make migrate pkg=model`
// s'exécute depuis la racine — comme toutes les autres cibles du Makefile.

import { defineConfig } from "drizzle-kit";

export default defineConfig({
  dialect: "postgresql",
  schema: "./src/ts/model/schema/identity.ts",
  out: "./src/ts/model/.drizzle-out",
  schemaFilter: ["identity"],
  verbose: true,
  strict: true,
});
