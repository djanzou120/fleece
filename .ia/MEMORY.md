# Fleece — Mémoire projet (à lire au démarrage de chaque session)

> **But de ce fichier.** Journal de référence de l'assistant : il consolide l'historique du travail,
> les **décisions techniques et leurs raisons**, les conventions du dépôt et l'état d'avancement.
> **À lire en premier** au début d'une session pour retrouver le contexte. **À mettre à jour** à la fin
> de tout changement structurant (nouvelle décision, nouveau module, changement de convention).
>
> Dernière mise à jour : **2026-06-13**.

---

## 1. Le projet en une phrase

**Fleece** = plateforme de communication omnicanale **API-first** : une seule API pour envoyer un message
sur le meilleur canal (SMS, WhatsApp, Telegram, …), au meilleur coût, avec la meilleure délivrabilité
(routage + fallback intelligents, wallet prépayé). Cible : Afrique francophone + Europe.

## 2. Documents de référence (dossier `.ia/`)

| Fichier | Rôle |
|---------|------|
| `PRD.md` | Product Requirements (existant, amendé). Vision, marché, exigences, roadmap. |
| `TDD.md` | Technical Design Document. Objectifs système, services, parcours, architecture cible. |
| `user-story.md` | User stories Dashboard + Intégration API (format complet + critères d'acceptation). |
| `ARCHITECTURE.md` | Structure de fichiers + Clean Architecture + conventions réelles du dépôt. |
| `MEMORY.md` | **Ce fichier** — mémoire/journal. |

## 3. Phases produit (étiquettes utilisées partout)

- 🟢 **MVP (P0)** — WhatsApp + SMS ; API REST, Wallet, Dashboard, Webhooks, Routing, Fallback (~3 mois).
- 🟡 **V1 (P1)** — Contact Intelligence, Telegram, Campagnes, Analytics avancées (~3 mois).
- 🔵 **V2 (P2)** — SSO, Messenger, RCS, optimisation IA du routage (~6 mois).

---

## 4. Décisions techniques (avec raisons)

| # | Décision | Choix | Raison |
|---|----------|-------|--------|
| D1 | Style d'architecture | **Microservices event-driven** | Tenir P95 < 200 ms (réponse API) ET 10 M msg/j (absorption par queue). |
| D2 | Langage du cœur métier | **Go** | Performance, concurrence (pipeline d'envoi). |
| D3 | Service d'authentification | **TypeScript + Better Auth** (`src/auth-api`) | Tirer parti de l'écosystème Better Auth. |
| D4 | Gateway privé dashboard | **TypeScript GraphQL** (`src/graphql-api`) | Agrège les services Go via REST interne ; sert exclusivement le dashboard Next.js. |
| D22 | Gateway public API REST | **TypeScript REST** (`src/rest-api`) | API Key + rate limiting + TLS ; P95 < 200 ms ; sert les clients externes. Symétrique avec D4 : deux BFF, deux audiences. |
| D23 | Lib partagée entre gateways | `src/ts/api-common` | Types purs (ApiContext, ApiError, pagination) sans règle métier ni dépendance framework. Conforme Clean Architecture : couche transverse 0, importable par couches 3/4. |
| D5 | Communication inter-services (sync) | **REST interne** (pas gRPC) | Cohérence avec l'API publique, simplicité d'outillage. |
| D6 | Communication asynchrone | **RabbitMQ** (événements) | Découplage du pipeline d'envoi + effets de bord. |
| D7 | Déploiement | **Kubernetes** | Services sans état, scaling horizontal (HPA), 99.9 %. |
| D8 | Recharge wallet | **Mobile Money** (Afrique) + **Stripe** (Europe) | Adapter de paiement sélectionné selon le pays du workspace. |
| D9 | RGPD / souveraineté des données | **Reportée** | À traiter au passage sur le marché européen. |
| D10 | Organisation du dépôt | **Monorepo** | Outillage/CI partagés ; déploiement indépendant conservé. |
| D11 | Base de données | **Unifiée** : 1 PostgreSQL, **1 schéma par service** | Simplicité d'exploitation ; séparation logique conservée ; scindable plus tard sans changer le code. **Compromis** : point de couplage vs 99.9 % (atténué par la séparation par schéma). |
| D12 | Migrations | **Dossier racine unique** `migrations/` + outil **Atlas** | Language-agnostic ; **linting CI** (changements destructeurs/verrouillants) ; **multi-schéma** natif. |
| D13 | ORM TypeScript | **Drizzle** | SQL-first, léger (images plus petites), adapter Better Auth natif, support schémas/`search_path`. **Ne possède pas les migrations** (Atlas = source de vérité ; évite le doublon). |
| D14 | Lint de frontières | **depguard** (Go) + **dependency-cruiser** (TS) | Interdire domain/application → adapters/infrastructure ; bloquer imports de frameworks hors couches 3/4. |
| D15 | Méthode de conception | **Clean Architecture** (4 couches, règle de dépendance) | Frameworks = détails ; ports/interfaces dans les couches internes. |
| D16 | Système de build | **Existant conservé** : `Makefile` + `mk/<type>.mk` + descripteur `src/<pkg>/pkg` | Le dépôt avait déjà un build maison délibéré ; on l'adapte plutôt que d'imposer `services/`/`libs/`. |
| D17 | Injection de dépendances | **Manuelle au composition root** | Pas de framework DI lourd ; câblage explicite (`main.go` / `index.ts`). |
| D18 | Clean Architecture côté BFF | **Allégée** | Le GraphQL Gateway n'a pas de règles d'entreprise → Application + Adapters. |
| D19 | Domaine partagé entre services | **Non** | Chaque service possède son domaine ; `src/go`, `src/ts/*` = transverse uniquement. |
| D20 | Organisation du travail | **Équipe d'agents pilotée par un PM** | Agents projet dans `.ia/.claude/agents/` ; le PM (`fleece-pm`) dispatche, suit et fait l'acceptance. Suivi écrit dans `.ia/PROJECT_TRACKER.md` (voir §9). |
| D21 | Mémoire de session | **`.ia/MEMORY.md` + hook SessionStart** | Hook dans `.ia/.claude/settings.json` qui injecte ce fichier au démarrage de chaque session. |

---

## 5. Conventions du dépôt (`/Users/djanzou120/Documents/Projects/fleece`)

> ⚠️ Le dépôt préexistait avec ses conventions. **Ne pas** imposer `services/`/`libs/`/`apps/` :
> on remplit la Clean Architecture **dans** `src/<pkg>`.

- **Packages** : `src/<pkg>/` (services ET libs). Chaque package a un descripteur `src/<pkg>/pkg`
  déclarant `type=go | node | react | docker | graphql`.
- **Build** : `make build pkg=<x>` ; image : `make image pkg=<x>`. Le Makefile inclut `src/<pkg>/pkg`
  puis `mk/<type>.mk`, et choisit `docker/<type>.dockerfile`.
- **Go** : module **unique** `module fleece` (go.mod racine). Imports : `fleece/src/<svc>/internal/...`.
  `go build ./src/<svc>` compile le `package main` (fichier `src/<svc>/main.go`).
- **TypeScript** : **workspaces npm** (`src/ts/*`, `src/auth-api`, `src/graphql-api`). Entrypoint `index.ts`
  bundlé par esbuild (`mk/node.mk`). Couches **directement** sous le dossier service (pas de `src/` imbriqué).
- **Migrations** : dossier racine unique `migrations/` (`0001_<service>.sql`…), config `atlas.hcl`.
- **Docker** : `docker/<type>.dockerfile` (`go.dockerfile`, `node.dockerfile`) + `src/bastion/Dockerfile`.
- **Docs** : dans `.ia/` (pas `docs/`).

### Correspondance nom logique (TDD) → package du dépôt

| Service | Package | Type |
|---------|---------|------|
| Identity | `src/auth-api` | node (TS + Better Auth) |
| **Gateway REST public** | `src/rest-api` | node (TS) 🟢 |
| **Gateway GraphQL privé (BFF dashboard)** | `src/graphql-api` | node (TS) 🟢 |
| Dashboard | `src/platform-app` | react |
| Messaging / Routing / Provider / Wallet / Webhook | `src/<même nom>` | go (🟢) |
| Campaign / Contact-Intelligence / Analytics | `src/<même nom>` | go (🟡) |
| Schéma + codegen GraphQL | `src/graphql` | graphql |
| Toolbox dev/CI (psql, **atlas**) | `src/bastion` | docker |
| Lib Go transverse (Version/Name + Bootstrap) | `src/go/app` | — |
| Libs TS transverses | `src/ts/*` (logger, config, form, gql, mail, **api-common**) | esbuild |

### Couches Clean Architecture (rappel)

1. **domain** (pur) → 2. **application** (usecases + `ports/{input,output}`) →
3. **adapters** (http, persistence, clients, messaging, providers) → 4. **infrastructure** (config, db, broker, serveur).
Dépendances **vers l'intérieur uniquement** ; inversion via ports.

---

## 6. État d'avancement (au 2026-06-12)

**Fait & vérifié :**
- 8 services Go scaffolés (`internal/{domain,application,adapters,infrastructure}` + `pkg` + `main.go`).
- **messaging** rempli comme référence : entité `Message` + machine à états (`internal/domain/message.go`),
  ports de sortie (`.../ports/output/ports.go`), use case `SendMessage` (`.../usecases/send_message.go`).
- **provider** a `internal/adapters/providers/` (le port `Provider` reste interne au service).
- Lib `src/go/app` (Version/Name injectés au build + `Bootstrap`) ; `mk/go.mk` corrigé (`anthill`→`fleece`).
- Services TS scaffolés : `src/auth-api` (Better Auth confiné dans `adapters/auth`) et `src/graphql-api` (BFF)
  avec `package.json`/`tsconfig.json`/`index.ts` ; workspace `auth-api` ajouté au `package.json` racine ; `mk/node.mk` créé.
- Migrations `migrations/0001..0009` + `README.md` ; `atlas.hcl`.
- `docker/go.dockerfile` (distroless, `ARG PKG`).
- `src/bastion/Dockerfile` : golang-migrate **remplacé par Atlas**.
- ✅ `go vet ./src/...` + `go build ./src/...` OK ; `make build pkg=messaging` produit le binaire (exécuté).

**Pas encore fait / à valider :**
- `src/rest-api` scaffolé (D22) — adapters HTTP + clients REST Go + infrastructure serveur à implémenter.
- `src/ts/api-common` scaffolé (D23) — types purs partagés ; middleware concrets à implémenter dans chaque gateway.
- Chaîne TS non exécutée (réseau) : `npm install` + `tsc --noEmit` + esbuild (`make build pkg=auth-api` / `graphql-api` / `rest-api`).
- Commandes Atlas non exécutées : `atlas migrate hash` / `lint` / `apply` (+ générer `atlas.sum`).
- `deploy/k8s/` (manifests) : à créer.
- `src/platform-app` (frontend react/Next.js) : non scaffolé (pas de `mk/react.mk` ni `docker/react.dockerfile`).
- `src/graphql` (codegen via `tools/make-gql`) : non câblé.
- Adapters concrets (Postgres/RabbitMQ/Redis/HTTP) et autres use cases : couches présentes mais vides (`doc.go`).

---

## 7. Journal des sessions

### Session 2026-06-13 (suite)
4. Séparation API publique / API privée : ajouté `src/rest-api` (gateway REST TS public, D22) et `src/ts/api-common`
   (lib partagée types purs, D23). `src/graphql-api` reste le BFF privé dashboard (GraphQL). Package.json racine mis à jour.
   ARCHITECTURE.md, MEMORY.md, CLAUDE.md mis à jour en conséquence.

### Session 2026-06-13
1. Mis en place un **hook SessionStart** (`.ia/.claude/settings.json`) qui charge `.ia/MEMORY.md` au démarrage (D21).
2. Créé l'**équipe d'agents** dans `.ia/.claude/agents/` : `fleece-pm` (orchestrateur), `fleece-go-engineer`,
   `fleece-ts-engineer`, `fleece-frontend-engineer`, `fleece-db-engineer`, `fleece-devops-engineer`,
   `fleece-qa-engineer`, `fleece-architect-reviewer` (D20).
3. Créé `.ia/PROJECT_TRACKER.md` (mémoire de suivi du PM) avec backlog MVP T-001..T-010.

### Session 2026-06-12
1. Rédigé `TDD.md` (architecture cible complète, niveau intermédiaire) à partir du PRD.
2. Rédigé `user-story.md` (Dashboard + API), personas Dev/Admin/Marketer.
3. Précisé que auth + GraphQL interne sont en **TypeScript** (D3, D4) → MAJ PRD + TDD.
4. Rédigé `ARCHITECTURE.md` (Clean Architecture, structure de fichiers).
5. Tranché : base **unifiée** (D11), `migrations/` unique (D12), `docker/` par langage, **Atlas** (D12),
   **Drizzle** (D13), **dependency-cruiser** (D14).
6. Découvert un squelette de monorepo préexistant (build maison) → décision **adapter à l'existant** (D16)
   + **passer à Atlas** dans bastion. Scaffolé le code, vérifié les builds Go.
7. Mis `ARCHITECTURE.md` en cohérence avec les conventions réelles (`src/<pkg>`, Makefile, docker/<type>).
8. Créé ce fichier `MEMORY.md`.

---

## 8. Points de vigilance / pièges à éviter

- **Shell = fish** : pour les scripts (boucles, heredocs, word-splitting), utiliser `bash -c '…'` ou un
  fichier `bash /tmp/x.sh`. `for x in $var` ne split pas en fish.
- **Ne pas** réintroduire `services/`/`libs/`/`apps/` : la structure réelle est `src/<pkg>` (D16).
- **Atlas est la source de vérité du schéma**, pas Drizzle (D13) — ne pas activer les migrations Drizzle.
- Base unifiée : un service **n'accède qu'à son schéma** ; pas d'accès cross-schéma (D11, isolation).
- Toujours respecter la **règle de dépendance** Clean Architecture (vers l'intérieur ; frameworks en couche 3/4).

---

## 9. Équipe d'agents & suivi projet

Agents définis dans `.ia/.claude/agents/` ; suivi tenu par le PM dans **`.ia/PROJECT_TRACKER.md`**.

| Agent | Rôle |
|-------|------|
| `fleece-pm` | **Orchestrateur** : découpe, dispatche, fait l'acceptance après chaque implémentation, tient `PROJECT_TRACKER.md`. Point d'entrée pour toute demande de feature/coordination/statut. |
| `fleece-go-engineer` | Services Go du cœur métier (`src/messaging,routing,provider,wallet,webhook,…`). |
| `fleece-ts-engineer` | `src/auth-api` (Identity/Better Auth) + `src/graphql-api` (BFF). |
| `fleece-frontend-engineer` | Dashboard `src/platform-app` (Next.js + shadcn). |
| `fleece-db-engineer` | Schéma PostgreSQL unifié + migrations Atlas (`migrations/`). |
| `fleece-devops-engineer` | `Makefile`/`mk/`, `docker/`, `deploy/` (K8s), infra RabbitMQ/Redis, CI. |
| `fleece-qa-engineer` | Tests + exécution des critères d'acceptance (verdict PASS/FAIL). |
| `fleece-architect-reviewer` | Garde-fous Clean Architecture & frontières (lecture seule). |

**Flux type** : PM → (db → go/ts → qa → architect-reviewer) → PM consigne le verdict dans `PROJECT_TRACKER.md`.
Pour lancer un travail, s'adresser au PM (`fleece-pm`), qui dispatche.
