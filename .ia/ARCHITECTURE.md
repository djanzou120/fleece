# Fleece — Document d'Architecture & Structure de Fichiers (Clean Architecture)

**Version :** 1.0
**Documents de référence :** [PRD.md](./PRD.md) v1.0 · [TDD.md](./TDD.md) v1.0
**Objet :** Définir l'organisation des fichiers de chaque service en respectant les règles de la **Clean Architecture**.
**Public visé :** Développeurs (Go et TypeScript), tech leads.

---

## 1. Principes directeurs

Ce document applique la **Clean Architecture** (Robert C. Martin) à l'architecture microservices event-driven décrite dans le [TDD](./TDD.md). Chaque service est structuré en **couches concentriques**, et **toutes les dépendances pointent vers l'intérieur**.

### 1.1 La règle de dépendance

> **Le code d'une couche interne ne connaît jamais une couche externe.**
> Les flux de contrôle peuvent traverser la frontière vers l'extérieur, mais les **dépendances de code** (imports) ne pointent que vers l'intérieur. L'inversion se fait par des **interfaces (ports)** définies dans les couches internes et **implémentées** dans les couches externes.

```text
          ┌───────────────────────────────────────────────┐
          │  4. Infrastructure / Frameworks & Drivers      │  ← Postgres, RabbitMQ,
          │   ┌─────────────────────────────────────────┐  │    Redis, HTTP, Better Auth
          │   │  3. Interface Adapters                   │  │  ← handlers, repositories,
          │   │   ┌───────────────────────────────────┐  │  │    clients, presenters
          │   │   │  2. Application (Use Cases)        │  │  │  ← orchestration + ports
          │   │   │   ┌─────────────────────────────┐  │  │  │
          │   │   │   │  1. Domain (Entities)        │  │  │  │  ← règles métier pures
          │   │   │   └─────────────────────────────┘  │  │  │
          │   │   └───────────────────────────────────┘  │  │
          │   └─────────────────────────────────────────┘  │
          └───────────────────────────────────────────────┘
                  Les dépendances pointent vers l'intérieur ▲
```

### 1.2 Les quatre couches appliquées à Fleece

| Couche | Rôle | Contenu Fleece | Dépend de |
|--------|------|----------------|-----------|
| **1. Domain** | Règles métier d'entreprise, pures | `Message`, `Wallet`, `Workspace`, `RoutingDecision`, value objects (`Money`, `Status`), machine à états, erreurs métier | Rien (aucun import externe) |
| **2. Application** | Cas d'usage, orchestration | `SendMessage`, `DebitWallet`, `CreateApiKey` + **ports** (interfaces) | Domain uniquement |
| **3. Interface Adapters** | Traduction entre le monde extérieur et l'application | Handlers HTTP/REST, résolveurs GraphQL, repositories Postgres, clients REST inter-services, consumers/publishers RabbitMQ, adapters fournisseurs | Application + Domain |
| **4. Infrastructure** | Frameworks et détails techniques | Connexions Postgres/Redis, serveur HTTP, client RabbitMQ, **Better Auth**, config, composition root | Toutes les couches (câblage) |

### 1.3 Conséquences clés pour Fleece

- **Les frameworks sont des détails.** **Better Auth**, RabbitMQ, GraphQL, Postgres vivent en **couche 3/4**, jamais dans le domaine. Le domaine de l'Identity Service définit `Workspace`, `User`, `ApiKey` ; Better Auth n'est qu'un détail d'implémentation branché à la frontière.
- **L'interface `Provider`** (TDD §7) est un **port** défini en couche Application du Provider Service ; chaque adapter fournisseur (couche 3) l'implémente.
- **Les contrats d'événements** (`message.delivered`, `wallet.refunded`, …) sont des ports de sortie ; RabbitMQ est le détail qui les transporte.
- **L'isolation des données par service** (TDD §3.4) est respectée : aucun service n'importe le domaine d'un autre ; les échanges passent par des **clients REST** (ports de sortie) ou des **événements**.

---

## 2. Organisation du dépôt (monorepo)

Décision : **monorepo** avec un **système de build maison** déjà en place : `Makefile` + includes
`mk/<type>.mk`, où chaque package déclare son type dans un descripteur `src/<pkg>/pkg`. On build avec
`make build pkg=<nom>` et on produit une image avec `make image pkg=<nom>`. Go vit dans un module unique
`module fleece` ; le TypeScript est géré en **workspaces npm** (`src/ts/*`).

```text
fleece/
├── src/                           # Tous les packages : services ET libs (1 dossier = 1 pkg)
│   ├── <pkg>/pkg                  #   descripteur : type=go | node | react | docker | graphql
│   │
│   ├── auth-api/                  # Identity Service — TypeScript + Better Auth   🟢
│   ├── graphql-api/               # GraphQL Gateway (BFF dashboard) — TypeScript  🟢
│   ├── platform-app/             # Frontend dashboard — Next.js/React            🟢
│   ├── messaging/                 # Go                                            🟢
│   ├── routing/                   # Go                                            🟢
│   ├── provider/                  # Go                                            🟢
│   ├── wallet/                    # Go                                            🟢
│   ├── webhook/                   # Go                                            🟢
│   ├── campaign/                  # Go                                            🟡
│   ├── contact-intelligence/      # Go                                            🟡
│   ├── analytics/                 # Go                                            🟡
│   ├── bastion/                   # Conteneur outillage dev/CI (psql, atlas, …)
│   ├── graphql/                   # Schéma GraphQL + codegen (type=graphql)
│   │
│   ├── go/                        # Lib Go transverse partagée (PAS de domaine métier)
│   │   └── app/                   #   Version/Name (injectés au build) + Bootstrap
│   └── ts/                        # Libs TS transverses partagées (workspaces npm)
│       ├── logger/  config/  form/  gql/  mail/
│
├── migrations/                    # ◀── Migrations de la BASE UNIFIÉE (un seul lieu, outil Atlas)
│   ├── 0001_identity.sql          #     numérotées globalement, ordre déterministe
│   ├── 0002_wallet.sql            #     un fichier = un schéma de service
│   ├── … 0009_analytics.sql       #     (0007–0009 = 🟡 V1)
│   └── README.md                  # conventions + commandes Atlas (voir §6.4)
├── atlas.hcl                      # config Atlas (env, dossier migrations, base dev)
│
├── docker/                        # ◀── Dockerfiles regroupés PAR LANGAGE (ARG PKG)
│   ├── go.dockerfile              #     build de tout service Go     → make image pkg=<svc>
│   └── node.dockerfile            #     build des services TS
│
├── mk/                            # Règles de build par type de package
│   ├── go.mk  node.mk  esbuild.mk  graphql.mk  docker.mk
│
├── Makefile                       # point d'entrée : make build/image/test pkg=<nom>
├── go.mod                         # module unique « fleece »
├── package.json                   # workspaces npm : src/ts/*, src/auth-api, src/graphql-api
├── tools/                         # scripts (génération de code, lint de frontières)
└── .ia/                           # Documentation : PRD.md, TDD.md, ARCHITECTURE.md, user-story.md
```

> **Règle sur les libs partagées (`src/go`, `src/ts/*`) :** ces bibliothèques sont **transverses et sans logique métier** (logging, config, transport). Le **domaine n'est jamais partagé** entre services — chaque service possède son propre domaine. Partager une entité métier briserait l'isolation et la règle de dépendance.

### 2.2 Correspondance « nom d'architecture → package du dépôt »

Les noms logiques du TDD se traduisent par des dossiers `src/<pkg>` (parfois nommés différemment pour des raisons historiques) :

| Service (TDD) | Package (`src/…`) | Type | Phase |
|---------------|-------------------|------|-------|
| Identity | `src/auth-api` | node (TS + Better Auth) | 🟢 |
| GraphQL Gateway (BFF) | `src/graphql-api` | node (TS) | 🟢 |
| Dashboard | `src/platform-app` | react | 🟢 |
| Messaging | `src/messaging` | go | 🟢 |
| Routing | `src/routing` | go | 🟢 |
| Provider | `src/provider` | go | 🟢 |
| Wallet | `src/wallet` | go | 🟢 |
| Webhook | `src/webhook` | go | 🟢 |
| Campaign | `src/campaign` | go | 🟡 |
| Contact Intelligence | `src/contact-intelligence` | go | 🟡 |
| Analytics | `src/analytics` | go | 🟡 |
| — (schéma/codegen GraphQL) | `src/graphql` | graphql | 🟢 |
| — (toolbox dev/CI) | `src/bastion` | docker | — |

### 2.1 Base de données unifiée

Décision : **une base de données PostgreSQL unique** partagée par tous les services, avec **un schéma logique par service** (`identity`, `wallet`, `messaging`, …). On conserve ainsi la **séparation logique** des données — chaque service ne lit/écrit que son schéma — tout en simplifiant l'exploitation (une seule instance, un seul jeu de migrations, transactions cross-schéma possibles si nécessaire).

```text
PostgreSQL (instance unique : fleece)
├── schema identity      ── workspaces, users, api_keys, audit_logs
├── schema wallet        ── wallets, wallet_transactions
├── schema messaging     ── messages, message_attempts
├── schema routing       ── provider_pricing, provider_scores, routing_rules
├── schema provider      ── providers, provider_credentials, provider_messages
├── schema webhook       ── webhook_endpoints, webhook_deliveries
├── schema campaign      ── campaigns, campaign_recipients, campaign_runs        🟡
├── schema contact_intel ── contacts, contact_channel_history                    🟡
└── schema analytics     ── vues / agrégats                                      🟡
```

> **Implication Clean Architecture inchangée.** La base unifiée est un **détail d'infrastructure (couche 4)**. La règle de discipline reste : **un service n'accède qu'à son propre schéma**, exclusivement via ses repositories (couche 3). Aucun use case ni domaine ne « sait » que la base est partagée. La séparation par schéma matérialise cette frontière au niveau base.
>
> **Compromis assumé :** une base unique est un point de couplage opérationnel (montée en charge, disponibilité 99.9 %) ; il pourra être scindé plus tard par schéma sans changer le code applicatif, puisque chaque service est déjà isolé à son schéma.

---

## 3. Structure d'un service Go (Clean Architecture)

Exemple de référence : **Messaging Service** (TDD §4.2), implémenté sous `src/messaging/`. Tous les services Go suivent ce gabarit. Build : `make build pkg=messaging` (→ `go build ./src/messaging`).

```text
src/messaging/
├── pkg                            # descripteur de build : type=go
├── main.go                        # Composition root (package main) : câble les couches (DI manuelle)
│
├── internal/
│   ├── domain/                    # ── COUCHE 1 : Domain (pur, zéro import externe) ──
│   │   ├── message.go             # Entité Message + invariants
│   │   ├── attempt.go             # Entité MessageAttempt (tentatives de fallback)
│   │   ├── status.go              # Value object Status + machine à états (TDD §6.1)
│   │   ├── channel.go             # Value object Channel (sms, whatsapp, …)
│   │   └── errors.go              # Erreurs métier (ErrInvalidTransition, …)
│   │
│   ├── application/               # ── COUCHE 2 : Use Cases + Ports ──
│   │   ├── ports/
│   │   │   ├── input/             # Ports pilotants (driving) — ce que le service offre
│   │   │   │   └── send_message.go        # interface SendMessageUseCase
│   │   │   └── output/           # Ports pilotés (driven) — ce dont le service a besoin
│   │   │       ├── message_repository.go  # interface (persistance)
│   │   │       ├── routing_gateway.go     # interface (appel Routing Service)
│   │   │       ├── wallet_gateway.go      # interface (débit/remboursement)
│   │   │       ├── provider_gateway.go    # interface (envoi via Provider Service)
│   │   │       └── event_publisher.go     # interface (publication d'événements)
│   │   └── usecases/
│   │       ├── send_message.go            # orchestration du flux d'envoi (TDD §5.2)
│   │       └── handle_delivery_receipt.go # mise à jour du statut sur DLR
│   │
│   ├── adapters/                  # ── COUCHE 3 : Interface Adapters ──
│   │   ├── http/                  # Driving : contrôleurs REST internes
│   │   │   ├── handler.go         # mappe requête HTTP → use case
│   │   │   └── dto.go             # DTO ↔ types domaine (jamais d'entité exposée brute)
│   │   ├── consumer/              # Driving : consumers RabbitMQ (workers d'envoi)
│   │   │   └── send_worker.go
│   │   ├── persistence/           # Driven : implémente message_repository (Postgres)
│   │   │   ├── message_repository.go
│   │   │   └── record.go          # modèle de table ↔ entité
│   │   ├── publisher/             # Driven : implémente event_publisher (RabbitMQ)
│   │   │   └── rabbitmq_publisher.go
│   │   └── clients/               # Driven : clients REST vers Routing/Wallet/Provider
│   │       ├── routing_client.go
│   │       ├── wallet_client.go
│   │       └── provider_client.go
│   │
│   └── infrastructure/            # ── COUCHE 4 : Frameworks & Drivers ──
│       ├── config/                # chargement de la config (env)
│       ├── postgres/              # pool de connexions, migrations runner
│       ├── rabbitmq/              # connexion, déclaration des queues/exchanges
│       ├── redis/                 # client (idempotence, verrous)
│       └── httpserver/            # serveur HTTP, routing, middlewares
│
└── test/                          # Tests d'intégration / e2e du service
```

> Le module Go est **unique** (`module fleece`, à la racine) : les imports internes sont du type
> `fleece/src/messaging/internal/domain`. Le `go.mod`/`go.sum` est partagé par tous les services Go.

> **Pas de dossier `migrations/` par service.** Toutes les migrations vivent dans le dossier racine **`migrations/`** unique (§2). Le service ne référence que **son schéma** (`messaging`) via la config de connexion (`search_path`).

**Points d'application Clean Architecture (Go) :**
- Le domaine sous `internal/domain` n'importe **aucun** package de `adapters` ou `infrastructure`.
- Les use cases dépendent **uniquement** des **interfaces** de `application/ports`, jamais des implémentations concrètes.
- L'injection se fait au **composition root** (`src/<svc>/main.go`) : il instancie les adapters concrets et les passe aux use cases (amorçage transverse via `fleece/src/go/app`).
- Le mot-clé `internal/` de Go empêche tout import depuis l'extérieur du service, même dans le module unique.

### 3.1 Sens des dépendances (Go)

```text
main.go ──(câble tout)──▶ infrastructure ──▶ adapters ──▶ application ──▶ domain
                                                              ▲
    adapters implémentent les ports ─────────────────────────┘ (inversion de dépendance)
```

---

## 4. Structure d'un service TypeScript (Clean Architecture)

Deux services sont en **TypeScript** (TDD §3.3, §4.1, §4.10) : l'**Identity Service** (`src/auth-api`, avec Better Auth) et le **GraphQL Gateway** (`src/graphql-api`). Mêmes couches, vocabulaire idiomatique TS. Les couches sont **directement** sous le dossier du package (pas de `src/` imbriqué) ; l'entrypoint est `index.ts` (bundle esbuild via `mk/node.mk`). Build : `make build pkg=auth-api`.

### 4.1 Identity Service — `src/auth-api` (TypeScript + Better Auth) 🟢

```text
src/auth-api/
├── pkg                            # descripteur de build : type=node
├── index.ts                       # Composition root (DI), compilé par esbuild
├── package.json   tsconfig.json
│
├── domain/                        # ── COUCHE 1 : Domain (pur) ──
│   ├── workspace.ts               # Entité Workspace
│   ├── api-key.ts                 # Entité ApiKey (statut, rotation, hash)
│   └── …                          # user.ts, errors.ts
│
├── application/                   # ── COUCHE 2 : Use Cases + Ports ──
│   ├── ports/output/
│   │   └── repositories.ts        # WorkspaceRepository, ApiKeyRepository, AuthProvider (port)
│   └── use-cases/
│       ├── create-api-key.ts
│       └── …                      # create-workspace, rotate/revoke/validate-api-key
│
├── adapters/                      # ── COUCHE 3 : Interface Adapters ──
│   ├── http/                      # Driving : contrôleurs REST internes
│   ├── auth/
│   │   └── better-auth.adapter.ts # ← Better Auth CONFINÉ ICI (implémente le port AuthProvider)
│   └── persistence/               # Driven : repositories (Drizzle ORM)
│
└── infrastructure/                # ── COUCHE 4 : Frameworks & Drivers ──
    └── db/
        └── better-auth.config.ts  # config Better Auth (adapter Drizzle, schéma identity)
```

> **Migrations centralisées.** L'Identity Service n'embarque pas ses migrations : elles sont dans le dossier racine **`migrations/`** (`0001_identity.sql`). L'ORM/Better Auth se connecte au **schéma `identity`** de la base unifiée.

> **Better Auth = détail d'infrastructure.** La couche Application déclare un **port** `AuthProvider`. L'adapter `better-auth.adapter.ts` (couche 3) l'implémente et la config (couche 4) l'initialise. Conséquence : remplacer ou faire évoluer Better Auth n'impacte ni le domaine ni les cas d'usage.

### 4.2 GraphQL Gateway / BFF — `src/graphql-api` (TypeScript) 🟢

Le BFF a un domaine **minimal** (il orchestre, il ne possède pas de règles métier d'entreprise ni de base de données propre — TDD §4.10). La Clean Architecture s'applique en privilégiant **Application + Adapters**.

```text
src/graphql-api/
├── pkg                            # type=node
├── index.ts                       # Composition root
├── package.json   tsconfig.json
│
├── application/                   # ── COUCHE 2 : Orchestration ──
│   ├── ports/output/
│   │   └── clients.ts             # IdentityClient, WalletClient (interfaces)
│   └── use-cases/
│       └── get-workspace-overview.ts
│
├── adapters/                      # ── COUCHE 3 : Interface Adapters ──
│   ├── graphql/                   # Driving : schéma + résolveurs
│   │   ├── schema.graphql
│   │   └── resolvers/
│   └── clients/                   # Driven : clients REST vers les services métier
│       └── identity.client.ts
│
└── infrastructure/                # ── COUCHE 4 ──
    └── server.ts                  # Apollo / GraphQL Yoga
```

---

## 5. Le Provider Service : un cas d'école Clean Architecture

L'extensibilité multi-fournisseur (TDD §7, mitigation du risque « dépendance fournisseurs ») illustre parfaitement la séparation des couches.

```text
src/provider/
├── internal/
│   ├── domain/
│   │   ├── message.go             # ce qui est envoyé
│   │   ├── money.go               # value object Money
│   │   └── status.go
│   ├── application/
│   │   ├── ports/output/
│   │   │   └── provider.go        # ◀── interface Provider (Send/EstimateCost/GetStatus)
│   │   └── usecases/
│   │       └── dispatch.go        # choisit l'adapter et envoie
│   ├── adapters/
│   │   └── providers/             # une implémentation par fournisseur
│   │       ├── whatsapp_meta.go   # implémente Provider   🟢
│   │       ├── sms_<operateur>.go # implémente Provider   🟢
│   │       └── telegram.go        # implémente Provider   🟡
│   └── infrastructure/
│       └── ...
```

Ajouter un fournisseur = **ajouter un fichier en couche 3** qui implémente le port `Provider`. Aucune couche interne n'est modifiée. C'est la traduction concrète de la règle de dépendance.

---

## 6. Conventions transverses

### 6.1 Nommage des dossiers (identique Go / TS)

| Dossier | Couche | Contenu |
|---------|--------|---------|
| `domain/` | 1 | Entités, value objects, erreurs métier, règles pures |
| `application/usecases` (`use-cases`) | 2 | Orchestration des cas d'usage |
| `application/ports/input` | 2 | Interfaces pilotantes (ce que le service expose) |
| `application/ports/output` | 2 | Interfaces pilotées (ce que le service requiert) |
| `adapters/http` · `adapters/graphql` | 3 | Entrées (controllers/resolvers) |
| `adapters/persistence` · `adapters/clients` · `adapters/publisher` | 3 | Sorties (implémentations des ports) |
| `infrastructure/` | 4 | Frameworks, connexions, serveur, config |
| `cmd/server/main.go` · `src/main.ts` | 4 | Composition root |

### 6.2 Flux de données à travers les couches (envoi d'un message)

```text
HTTP request
   │  adapters/http/handler        (3) traduit DTO → commande
   ▼
application/usecases/SendMessage   (2) orchestration
   │  appelle les PORTS de sortie :
   ├─▶ wallet_gateway   ── impl. adapters/clients (3) ─▶ Wallet Service (REST)
   ├─▶ routing_gateway  ── impl. adapters/clients (3) ─▶ Routing Service (REST)
   ├─▶ provider_gateway ── impl. adapters/clients (3) ─▶ Provider Service (REST)
   ├─▶ message_repository ─ impl. adapters/persistence (3) ─▶ Postgres (4)
   └─▶ event_publisher  ── impl. adapters/publisher (3) ─▶ RabbitMQ (4)
   │
   ▼  manipule des entités domaine (1) : Message, Status (machine à états)
```

### 6.3 Tests par couche

- **Domain (1)** : tests unitaires purs, sans mock (logique métier, machine à états).
- **Application (2)** : tests unitaires avec **mocks des ports** (output) — aucun framework.
- **Adapters (3)** : tests d'intégration ciblés (repo ↔ Postgres de test, client ↔ service stub).
- **Service complet** : tests e2e sous `test/` avec dépendances conteneurisées.

### 6.4 Migrations (base unifiée, dossier unique)

- **Emplacement unique :** `migrations/` à la racine. Aucune migration dans les services.
- **Nommage :** préfixe numérique global croissant + service (`0003_messaging.sql`) → **ordre d'exécution déterministe** sur l'ensemble.
- **Périmètre d'un fichier :** chaque migration crée/modifie **les tables d'un seul schéma** (`CREATE SCHEMA IF NOT EXISTS messaging; ...`). On ne mélange pas deux schémas dans un même fichier.
- **Outil : Atlas.** Migrateur unique, language-agnostic, appliqué en job d'init du déploiement Kubernetes avant le démarrage des services. Retenu pour son **linting de migrations en CI** (détection des changements destructeurs/verrouillants, en appui de la cible 99.9 %) et sa **gestion native du multi-schéma** (base unifiée). Les migrations restent des fichiers SQL versionnés du dossier `migrations/`.
- **Discipline :** une PR qui ajoute une table modifie `migrations/` **et** le repository du service concerné — jamais le schéma d'un autre service.
- **L'ORM ne possède pas les migrations.** Côté TypeScript, **Drizzle** est utilisé comme query builder + typage du schéma ; il **n'exécute pas** ses propres migrations. **Atlas** reste l'unique source de vérité du schéma. Cela évite tout doublon entre l'ORM et le dossier `migrations/`.

### 6.5 Build des images Docker (par langage)

Les Dockerfiles sont **regroupés par langage** dans `docker/<type>.dockerfile` et **paramétrés par package** (`ARG PKG`). Le `Makefile` sélectionne automatiquement le Dockerfile selon le `type` déclaré dans `src/<pkg>/pkg`, et builde via `make image pkg=<x>` :

| Fichier | Sert à builder | Invocation |
|---------|----------------|-----------|
| `docker/go.dockerfile` | Tous les services Go | `make image pkg=messaging` |
| `docker/node.dockerfile` | Services TS (`auth-api`, `graphql-api`) | `make image pkg=auth-api` |
| `src/bastion/Dockerfile` | Conteneur outillage (type=docker) | `make image pkg=bastion` |

- Builds **multi-stage** (compilation via `make build pkg=$PKG` puis image runtime minimale — distroless pour Go) ; le contexte de build est la **racine du monorepo** (accès à `src/go`, `src/ts`).
- L'argument `PKG` sélectionne le dossier `src/$PKG` à compiler → **un seul Dockerfile par langage** pour N services.
- Sous le capot : `make image pkg=<x>` → `docker buildx build . -f docker/<type>.dockerfile --build-arg PKG=<x> …` (voir `Makefile`).
- Les manifests `deploy/k8s/` (à venir) référenceront les images produites.

---

## 7. Application de la règle de dépendance (garde-fous)

Pour éviter la dérive architecturale, les frontières sont **vérifiées automatiquement** en CI :

- **Go** : `golangci-lint` avec le linter **depguard** — interdit à `internal/domain` et `internal/application` d'importer `adapters`/`infrastructure`. Le découpage `internal/` empêche tout import inter-services.
- **TypeScript** : **dependency-cruiser** — règles sur le graphe de dépendances interdisant à `domain`/`application` d'importer `adapters`/`infrastructure`, et interdisant tout import de `node_modules` framework (Better Auth, Apollo, Drizzle) hors des couches 3/4. Tourne en CI indépendamment d'ESLint et génère un graphe visuel des dépendances.
- **Inter-services** : aucun import du domaine d'un autre service ; toute communication passe par `adapters/clients` (REST interne) ou les événements.

---

## 8. Décisions et points ouverts

| # | Décision | Choix retenu | Note |
|---|----------|--------------|------|
| 1 | Organisation du dépôt | **Monorepo** | Outillage/CI partagés ; déploiement indépendant conservé. |
| 2 | Système de build | **Existant conservé** : `Makefile` + `mk/<type>.mk` + descripteur `src/<pkg>/pkg` | Adapté au squelette en place plutôt qu'une arborescence `services/`/`libs/` idéalisée ; module Go unique, workspaces npm. |
| 3 | Base de données | **Unifiée** (1 instance PostgreSQL, 1 schéma par service) | Séparation logique conservée ; scindable plus tard sans changer le code (§2.1). |
| 4 | Migrations | **Dossier racine unique** `migrations/`, outil **Atlas** | Numérotation globale déterministe ; un fichier = un schéma ; linting CI + multi-schéma (§6.4). Câblé dans `src/bastion`. |
| 5 | ORM TypeScript | **Drizzle** | SQL-first, léger, adapter Better Auth natif ; ne possède pas les migrations (Atlas est la source de vérité). |
| 6 | Dockerfiles | **Regroupés par langage** dans `docker/<type>.dockerfile` | Un Dockerfile par langage, paramétré `ARG PKG`, sélectionné par le Makefile (§6.5). |
| 7 | Domaine partagé entre services | **Non** | Chaque service possède son domaine ; `src/go` et `src/ts/*` ne contiennent que du transverse. |
| 8 | Style Clean Architecture côté BFF | **Allégé** | Le GraphQL Gateway, sans règles d'entreprise, privilégie Application + Adapters. |
| 9 | Injection de dépendances | **Manuelle au composition root** | Pas de framework DI lourd ; câblage explicite dans `main.go` / `index.ts`. |
| 10 | Lint de frontières TS | **dependency-cruiser** | Règles sur le graphe de dépendances par dossier ; CI indépendante d'ESLint ; graphe visuel (§7). |

Toutes les décisions d'architecture structurantes sont désormais arrêtées et reflétées dans le dépôt.

---

*Document dérivé de [PRD.md](./PRD.md) et [TDD.md](./TDD.md). La structure de fichiers ci-dessus est le gabarit de référence ; toute nouvelle fonctionnalité doit respecter la règle de dépendance et le placement par couche décrits ici.*
