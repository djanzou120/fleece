# Fleece — Mémoire projet (à lire au démarrage de chaque session)

> **But de ce fichier.** Journal de référence de l'assistant : il consolide l'historique du travail,
> les **décisions techniques et leurs raisons**, les conventions du dépôt et l'état d'avancement.
> **À lire en premier** au début d'une session pour retrouver le contexte. **À mettre à jour** à la fin
> de tout changement structurant (nouvelle décision, nouveau module, changement de convention).
>
> Dernière mise à jour : **2026-07-05**.

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
| `design/DESIGN.md` | **Design système complet** — tokens CSS, typographie, composants, layouts. Chargé au SessionStart. |
| `design/Fleece Dashboard.dc.html` | Design interactif du dashboard (source claude.ai/design). |
| `design/Fleece Auth & Onboarding.dc.html` | Design auth + onboarding 4 étapes. |
| `design/Fleece Design System.dc.html` | Design system interactif (couleurs, typo, composants). |

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
| D24 | Modélisation du scoring par canal (Contact Intelligence) | **Table dédiée `contact_intel.contact_channel_scores`** (PK `(phone, channel)`) plutôt qu'une colonne `JSONB` sur `contacts` | Indexabilité native `(channel, score DESC)` requise par `highest_delivery` (routing T-016), impossible sur JSONB sans index fonctionnel complexe ; granularité de verrouillage sur upsert incrémental (lock ligne `(phone,channel)` vs toute la ligne `contacts`) ; lisibilité SQL + upsert `ON CONFLICT` simple côté service Go. Migration additive `0014_contact_intel_scoring.sql` (T-011). |
| D25 | Phase de livraison du gateway REST public `src/rest-api` (D22) | **Livré en V2 🔵** comme track indépendant (tâches T-043 impl / T-044 rate limiting Redis / T-045 DevOps) | Dette MVP « API-first » non implémentée (seul le BFF GraphQL privé a été livré au MVP). Le rest-api est la surface publique des développeurs externes ; le planifier dans la fenêtre V2 est cohérent (les nouveaux canaux Messenger/RCS doivent être atteignables via l'API publique). Non bloquant pour SSO/canaux/IA → track parallèle. Alternative écartée : backlog « dette » séparé (aurait laissé le produit API-first incomplet sans jalon). |
| D26 | Formule de scoring de joignabilité (Contact Intelligence, T-012) | **Lissage de Laplace α=1** : `score = floor((success+1)/(success+failure+2)*100)`, borné [0,100] | Évite les extrêmes 0/100 sur petit échantillon (prior neutre 50 sans données) ; déterministe, monotone (succès↑→score↑, échec↑→score↓), incrémental ; entièrement en domaine pur (aucun I/O). Consommé par routing `highest_delivery` (T-016) via REST interne (`GET /contacts/{phone}/score`), jamais en cross-schéma (D11). Limite connue : `success_count` par canal non persisté dans `contact_channel_scores` → reconstruit par inversion (précision ±1 pt) ; colonne dédiée = amélioration future (migration additive dédiée) si précision absolue requise. |
| D27 | Stockage des credentials & capabilities du canal Telegram (T-013) | **Credentials : réutilise `provider.provider_credentials.secret_enc bytea`** (bot token chiffré AES-GCM, 1 secret global par provider) ; **capabilities : constantes Go dans l'adapter C3** (pas de colonne base) | Le bot token Telegram est global au provider (non multi-tenant à ce stade), symétrique de `whatsapp-meta`/`sms-twilio` → réutiliser le pattern base chiffrée existant (option retenue vs env/Secret pur ou table par workspace). Aucun secret n'est écrit en clair dans une migration (ligne credentials insérée hors migration, ex. job d'init). Les capabilities (rich media, taille max) ne font l'objet d'aucune requête SQL → les garder en constantes de l'adapter évite un couplage schéma/logique (colonne JSONB = migration additive triviale si besoin dynamique en V2). Provider seedé `id='telegram-bot'` (convention `{channel}-{vendor}` du registry `src/provider/main.go`). Migration `0015_provider_telegram.sql`. **Note T-014** : l'adapter lit en réalité le token via config/env (comme tous les adapters existants) ; la lecture chiffrée en base est un `TODO(production)` transverse (dette tracée, dépend de T-024). |
| D28 | Intégration du score Contact Intelligence dans le routing `highest_delivery` (T-016) | **Score contact récupéré par REST interne (jamais SQL cross-schéma), combiné au score provider via une fonction PURE `CombineScore` avec fallback gracieux** | Le score contact est une I/O (service externe) → il ne peut PAS entrer dans le domaine pur : pattern = port output `ContactScoreClient` en C2 (types purs) + client REST en C3 (parle HTTP, n'importe jamais `fleece/src/contact-intelligence`, isolation D11/D19). Combinaison = `clamp((providerScore+contactScore+1)/2, 0,100)` (moyenne 50/50), **le score contact s'applique au CANAL** (tous les providers d'un canal reçoivent le même delta → l'ordre relatif entre providers du canal est préservé, mais un canal peu joignable pour le destinataire est globalement pénalisé). **Fallback gracieux obligatoire** : client indisponible/timeout(2s)/erreur/JSON invalide/`recipient` vide/client nil → aucune erreur remontée, dégradation vers `provider_scores` seuls (le routage ne doit jamais échouer à cause de contact-intelligence). Appel effectué uniquement pour la stratégie `highest_delivery`. Le `selector.go` reste pur (le use case orchestre l'enrichissement sur une copie de slice). |

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

### Session 2026-07-13 (T-016 — routing × Contact Intelligence)
1. **T-016 Done/PASS** (Vague 2 V1, tâche 2/4) : `highest_delivery` combine le score provider (`provider_scores`) et le score de
   joignabilité par canal du destinataire (contact-intelligence T-012) via REST interne. go-engineer → PM (pré-vérif mécanique pendant une
   indispo modèle) → qa (PASS 9/9) + architect-reviewer (CONFORME) → PM (nitpicks soldés).
2. **Décision D28** : port C2 `ContactScoreClient` + client REST C3 (n'importe jamais le service contact, isolation D11/D19) ; fonction pure
   `CombineScore = clamp((ps+cs+1)/2,0,100)` (le score contact s'applique au canal) ; **fallback gracieux** (nil/timeout 2s/erreur/recipient
   vide → dégradation vers provider_scores, jamais d'échec du routage) ; appel limité à la stratégie `highest_delivery`.
3. **Impact signatures** : `GetRoutingDecision.Execute` +`recipient string` ; DTO `RouteRequest.recipient` optionnel `omitempty` (rétrocompat).
   Config `CONTACT_INTEL_API_URL` ; câblage `main.go` (nil si URL vide). Nitpicks (ligne morte, gofmt) soldés par le PM.
4. **Vigilances suite** : T-015 (le fallback consomme `FallbackChain` dans l'ordre, sans recalcul de score) ; T-018 (campaign doit passer
   `recipient` par destinataire pour bénéficier de l'enrichissement, sinon routage global dégradé) ; T-020 (les événements ne transportent pas
   le contactScore — à enrichir seulement si on veut tracer l'impact de l'enrichissement, à arbitrer par le PM).

### Session 2026-07-12 (suite — T-014 adapter provider Telegram ; Vague 2 démarrée)
1. **T-014 Done/PASS** (Vague 2 V1, tâche 1/4) : adapter `src/provider/internal/adapters/providers/telegram.go` implémentant le port
   `output.Provider` (stub `net/http` + `TODO(production)`, pattern WhatsAppMeta/SMSTwilio). go-engineer → qa (PASS 9/9, 64 tests) +
   architect-reviewer (CONFORME) → PM (correction observation) → re-vérif. Registry `"telegram-bot"` câblé dans `main.go`.
2. **Faits/décisions** : mapping statut pur `MapTelegramResponseStatus` (ok→Sent, 400/403→Rejected, ≥500/timeout→Failed) ; **coût = 0**
   (Telegram Bot API gratuit) ; **capabilities en constantes Go** dans l'adapter (D27, pas de colonne base) ; assertion de port corrigée en
   `var _ output.Provider = (*TelegramBot)(nil)` (observation QA/archi soldée).
3. **Dette D27 (token en config/env)** : l'adapter lit le token via env `TELEGRAM_BOT_TOKEN` — comme TOUS les adapters existants
   (WhatsApp/SMS), aucun ne lit `provider_credentials` en base. La lecture chiffrée AES-GCM (D27) reste un `TODO(production)` transverse au
   service provider, à câbler quand T-024 (Secret K8s clé AES-GCM) sera prêt. Tracée en Blocages.
4. **Vigilances suite** : T-015 (routing : channel `'telegram'`, fallback WhatsApp→SMS→Telegram) ; T-038 (seed : `provider_pricing` cost=0
   pour telegram-bot, score de délivrabilité par défaut ~75-80 proposé — à trancher par db-engineer) ; DLR Telegram réel = webhook (adapter
   distinct ultérieur, pas de pull).

### Session 2026-07-12 (suite — T-019 schéma Analytics ; ✅ Vague 1 V1 complète)
1. **T-019 Done/PASS** (dernière tâche schéma de la Vague 1 V1) : `migrations/0017_analytics_kpi.sql` (db-engineer → qa → PM ; pas
1. **T-019 Done/PASS** (dernière tâche schéma de la Vague 1 V1) : `migrations/0017_analytics_kpi.sql` (db-engineer → qa → PM ; pas
   d'architect-reviewer, SQL pur). Ajoute la latence (`delivery_latency_ms_sum`), la **vue matérialisée `analytics.kpi_daily`** (4 KPIs
   produit : délivrabilité, échec, coût moyen/msg, latence moyenne), les index de perf et l'index UNIQUE requis pour `REFRESH CONCURRENTLY`.
2. **Choix de modélisation** : **latence stockée en SOMME** (pas en moyenne) → agrégat additif correct (une moyenne stockée n'est pas
   cumulable) ; MV `WITH NO DATA` (1er refresh au boot du service, pas dans la migration) ; requête MV strictement `FROM analytics.message_daily`
   (D11, données par événements). Protection division/0 via `CASE WHEN dénom>0`. QA PASS 8/8 ; observation cosmétique (commentaire NULLIF vs
   DDL CASE WHEN) soldée par le PM (commentaire aligné + atlas hash/validate re-EXIT 0).
3. **Vigilances T-020 (service Analytics Go)** : consumer `message.delivered`/`message.failed` → UPSERT `message_daily`
   (`ON CONFLICT (day,workspace_id,country,channel) DO UPDATE`, incrémente sent/delivered/failed/cost/latency_sum ; **clarifier quel événement
   incrémente `sent`** pour éviter le double-comptage) ; job de refresh (1er non-concurrent car MV vide, puis `REFRESH ... CONCURRENTLY`,
   timeout ~30s, intervalle `ANALYTICS_REFRESH_INTERVAL` défaut 5 min, en couche 4) ; use cases `GetKpis`/`GetTimeSeries` lisent `kpi_daily` ;
   `search_path=analytics` strict, aucun accès cross-schéma (D11).
4. **Jalon** : **Vague 1 V1 COMPLÈTE** — T-011 (schéma Contact Intel), T-012 (service Contact Intel), T-013 (schéma provider Telegram),
   T-017 (schéma Campaign), T-019 (schéma Analytics) tous Done/PASS. Suite = Vague 2 : T-014 → T-016 → T-018 → T-020.

### Session 2026-07-12 (suite — T-017 schéma Campaign)
1. **T-017 Done/PASS** (Vague 1 V1) : `migrations/0016_campaign_execution.sql` (db-engineer → qa → PM ; pas d'architect-reviewer, SQL pur).
1. **T-017 Done/PASS** (Vague 1 V1) : `migrations/0016_campaign_execution.sql` (db-engineer → qa → PM ; pas d'architect-reviewer, SQL pur).
   Enrichit les 3 tables `campaign` : `campaigns` (message_body/channel_strategy/estimated_cost/currency/rate_limit),
   `campaign_recipients` (status + `message_id` + UNIQUE dédup `(campaign_id, recipient)`), `campaign_runs` (total/sent/delivered/failed
   counters) + 3 index (dont `campaigns(status, scheduled_at)` pour le scheduler). QA PASS 7/7.
2. **Choix de modélisation** (cohérents avec décisions existantes, pas de nouvelle décision structurante) : `estimated_cost bigint` en
   centimes (aligné wallet/D8) ; `rate_limit` 0=illimité ; **`message_id` = colonne libre non-FK** pour corréler le DLR sans franchir la
   frontière de schéma (D11) ; déduplication **enforced en base** (index UNIQUE nommé ; pattern `ON CONFLICT DO NOTHING` côté service).
3. **Vigilances T-018 (service Campaign Go)** : machine à états (draft→scheduled[require message_body≠''+scheduled_at]→running→completed/failed) ;
   estimation coût via **client REST routing** (`provider_pricing`), jamais en SQL cross-schéma (D11) ; scheduler interroge
   `status='scheduled' AND scheduled_at<=now()` puis `UPDATE ... WHERE status='scheduled'` atomique (anti-double-run) ; rate_limit =
   token bucket par run ; consumer DLR corrèle par `message_id` (gérer le cas NULL avant injection) ; reprise via `status='pending'`.

### Session 2026-07-12 (suite — T-013 schéma provider Telegram)
1. **T-013 Done/PASS** (Vague 1 V1) : `migrations/0015_provider_telegram.sql` (db-engineer → qa → PM ; pas d'architect-reviewer,
   SQL pur). Additive : seed idempotent du provider `telegram-bot` (`channel='telegram'`, `ON CONFLICT DO NOTHING`).
2. **Décisions (D27)** : credentials Telegram → réutilise `provider_credentials.secret_enc` (AES-GCM, hors migration, 0 secret en clair) ;
   capabilities → constantes Go dans l'adapter C3. QA PASS 8/8 (additivité, isolation `provider`, 0005/0012 intacts, atlas hash/validate OK).
3. **Vigilances T-014 (adapter Telegram)** : câbler `"telegram-bot"` dans le registry `src/provider/main.go` ; lire le token via
   `provider_credentials WHERE provider_id='telegram-bot'` (déchiffrement AES-GCM, même chemin que les adapters existants) ; définir les
   capabilities en constantes Go dans `telegram.go` (C3) ; `channel='telegram'` déjà accepté (`text` libre, pas de CHECK).
   **Vigilance T-024 (DevOps)** : Secret K8s clé AES-GCM + insertion de la ligne credentials chiffrée hors migration (job d'init).

### Session 2026-07-12 (T-012 — Contact Intelligence)
1. **T-012 livré Done/PASS** (Vague 1 V1) : service `src/contact-intelligence` complet en Clean Architecture 4 couches, stdlib-only,
   go.mod/go.sum inchangés. go-engineer → qa + architect-reviewer (parallèle) → PM. PASS du 1er coup.
2. **Décisions/faits** : scoring **Laplace α=1** (D26) ; **upsert transactionnel** (`OutcomeStore.RecordAtomically` : score `ON CONFLICT (phone,channel)`
   + compteurs `contacts` + historique dans un seul `sql.Tx`) — la vigilance de T-011 est honorée ; port **8086** (suite 8081-8085) ; consumer DLR
   `message.delivered`/`message.failed` → `RecordDeliveryOutcome`. QA PASS 8/8 (29 tests, 0 régression, 17 pkgs ok). Architecture CONFORME.
3. **Contrat pour T-016** : `GET /contacts/{phone}/score` → `{phone, delivery_score, found, channel_scores[{channel, score, sample_size,
   last_success_at}]}` ; contact inconnu → `found=false` + `delivery_score=50` (jamais 404) ; `channel_scores` trié `score DESC`.
   **T-016 doit y accéder par REST interne** (client `adapters/clients/`, port output `ContactScoreClient` C2) avec **fallback gracieux**
   si le service est indisponible (dégradation vers `provider_scores` seul) — jamais d'accès cross-schéma (D11).
4. **Dette pré-existante relevée** (par l'architect-reviewer, non introduite par T-012) : **aucune config depguard/golangci** dans le dépôt
   alors que CLAUDE.md/ARCHITECTURE.md §7 l'affirment → conformité manuelle seule ; tâche devops à créer (voir Blocages tracker). Vigilances mineures
   T-012 : `success_count` par canal reconstruit par inversion (±1 pt) ; parseur `text[]` manuel ; `EnrichRecipient` non exposé.

### Session 2026-07-11 (suite — planification V2)
1. **Affinage du backlog V2 🔵** au niveau de détail V1 (planification pure, aucun code, aucun commit).
   Audit PRD §15 / TDD §9 : périmètre V2 = **SSO, Messenger, RCS, optimisation IA du routage** (Email hors roadmap ; A/B testing
   rattaché au routage cf. « optimisation IA »).
2. **Scissions d'atomicité** (1 tâche = 1 agent = 1 livrable) : **T-039** (schéma `identity.sso_connections`, extrait de T-026) ·
   **T-040** (seed pricing/scores Messenger/RCS, extrait de T-031) · **T-041** (schémas A/B : `routing.experiments` + métriques variantes
   `analytics`, 2 fichiers, extrait de T-033) · **T-042** (analytics métriques par variante + feedback, côté analytics de T-033).
   T-026/T-031/T-033/T-035/T-037 ajustés (dépendances + critères).
3. **Décision D25** : le gateway REST public `src/rest-api` **sort en V2** → track T-043 (impl) + T-044 (rate limiting Redis, PRD §Rate
   Limiting) + T-045 (DevOps K8s + Redis). QA V2 (T-037) étendue à l'acceptance de l'API publique (API-01..04).
4. **Ordre d'exécution V2 en 8 vagues (A→H)** ajouté au tracker (prérequis : V1 Done/PASS pour les tâches IA ; SSO/canaux/rest-api ne
   dépendent que du MVP). Observations tracées (canaux = dimension absorbée par campaign/analytics/webhook ; Email hors scope).

### Session 2026-07-11
1. **Remboursement de dette technique transverse** (branche `chore/tech-debt-paydown`, commit dédié). Orchestration PM :
   go-engineer (wallet) ‖ devops → ts-engineer (BFF+auth-api) → frontend-engineer (finalisé après un stall de l'agent) → qa + architect-reviewer.
2. **Soldé** : D-E01/T-007.4 (wallet `ListTransactions` + endpoint REST `GET /wallets/{workspaceId}/transactions` → `Query.transactions`) ;
   D-E03/T-008.1a (`Mutation.rotateApiKey` atomique) ; T-008.1f (`Mutation.createWorkspace`) ; D-E02/T-008.1c (`WebhookEndpoint.secretPreview`
   masqué) ; D-E06/T-008.4 (session dashboard : `lib/session.ts` + `WorkspaceContext` + résolution serveur ; fin du placeholder
   `WORKSPACE_ID_FROM_SESSION`) ; harmonisation `@anthill/`→`@fleece/` (auth-api, graphql-api, 5 libs `src/ts/*`) ; T-009.2 (CI
   `make build pkg=rest-api`) ; T-009.4 (cible Makefile `k8s-configmaps` idempotente). Verdict **PASS 6/6 + CONFORME**, go.mod inchangé,
   `tsc --noEmit` exit 0 (3 projets), 0 import backend au frontend, frameworks GraphQL confinés C4.
3. **Résiduels NON soldables offline** (réseau) : clients REST BFF encore stubs (`TODO(production)`, D-E05/T-007.1/.2) ; Apollo/Yoga non
   installés ; `createWorkspace` auth-api attend `ownerEmail` (à dériver du token de session) ; `WebhookEndpoint.secret` à exposer côté
   service webhook Go ; top-up wallet réel = V2 (paiement, D8) ; `npm install`/`next build`/`docker`/`kubectl`/`atlas apply` à valider en CI.
4. **Backlog V1** revu : découpage cohérent ; scission d'atomicité → **T-038** (seed pricing/scores Telegram, db-engineer, débloque T-015) ;
   ajout d'un **ordre d'exécution V1 en 5 vagues** dans le tracker. Observations : Contact Intelligence sans écran dashboard (backend-only,
   assumé) ; gateway REST public `src/rest-api` (D22) toujours non implémenté = dette MVP hors scope V1 (à planifier séparément).

### Session 2026-07-05
1. **Démarrage V1 🟡.** MVP P0 (T-001..T-010) clôturé Done/PASS. Première tâche V1 traitée : **T-011** (schéma
   Contact Intelligence), sans dépendance, prérequis de T-012.
2. Migration additive `migrations/0014_contact_intel_scoring.sql` (db-engineer) : enrichit `contact_intel.contacts`
   (country/last_channel/last_success_at + compteurs success_count/failure_count), crée la table
   `contact_intel.contact_channel_scores` (**D24** — table dédiée vs JSONB) et 5 index de scoring/lecture.
   100% additive (`IF NOT EXISTS`), `0008` intact, schéma `contact_intel` isolé. `atlas migrate hash`+`validate` OK,
   `atlas.sum` régénéré ; `atlas migrate lint` réservé Atlas Pro (v0.38+) → à faire en CI (`atlas login`).
3. Acceptance QA **PASS** 7/7 (offline-safe, revue statique + grep additivité/isolation + git diff 0008 + atlas hash/validate).
   Pas d'architect-reviewer (SQL pur, aucune frontière Clean Architecture). T-011 Done/PASS ; prérequis de T-012 levé.

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
