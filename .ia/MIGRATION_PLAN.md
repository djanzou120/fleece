# Plan de migration — Architecture simplifiée (inspiration winmarket)

> **Créé le :** 2026-07-14
> **Statut global :** ✅ Phase 0 Done/PASS — Phase 1 prête à démarrer
> **Dernière mise à jour :** 2026-07-14
>
> Ce fichier est le **tracker de référence de la migration**. L'agent PM l'utilise pour
> dispatcher les tâches et suivre l'avancement. Il est prioritaire sur `PROJECT_TRACKER.md`
> pour tout ce qui concerne la migration.

---

## Contexte et motivation

### Ce qu'on quitte

L'architecture actuelle applique la Clean Architecture stricte dans **8 services Go indépendants**
(messaging, routing, provider, wallet, webhook, contact-intelligence, campaign, analytics),
chacun avec 4 couches (domain / application / adapters / infrastructure), des repositories,
des ports/interfaces, une composition root manuelle, son propre serveur HTTP et son propre
consumer RabbitMQ. Résultat : ~30 fichiers par service, beaucoup de boilerplate, overhead
opérationnel élevé (8 pods K8s, 8 ports internes, 8 connexions DB distinctes).

### Ce qu'on vise

Architecture inspirée de `../winmarket/backend` :
- **1 service HTTP Go unifié** (`src/api`) — tous les endpoints internes
- **2 workers Go** (`src/core-processor`, `src/intelligence-processor`) — consumers + schedulers
- **Libs Go partagées** (`src/go/sql`, `src/go/log`, `src/go/syncx`, `src/go/amqp`)
- **Structure plate** : 1 fichier par endpoint, `zconfig` pour la DI, accès DB direct
- **Clean Architecture uniquement là où c'est justifié** : interface `Provider` (multiples impls),
  machines à états (`Message`, `Campaign`), fonctions pures testables (`CombineScore`, `Laplace`)

### Services TS inchangés

`src/auth-api`, `src/graphql-api`, `src/rest-api` — pas touchés par cette migration.

---

## Architecture cible

```
Clients externes
  ├── src/auth-api      (TS — Better Auth, sessions)          inchangé
  ├── src/graphql-api   (TS — BFF dashboard, GraphQL)         inchangé
  └── src/rest-api      (TS — API publique, API Key)           inchangé
           │
           ▼
      src/api           (Go — TOUS les endpoints HTTP internes)
           │
           ├── src/core-processor         (Go worker — consumers DLR)
           └── src/intelligence-processor (Go worker — consumers events + schedulers)

Infra : PostgreSQL (schémas inchangés) · RabbitMQ · Redis
Libs  : src/go/sql · src/go/log · src/go/syncx · src/go/app · src/go/amqp
```

### Structure interne de `src/api/`

```
src/api/
  service.go          // struct Service{DB, Logger, AMQP, Providers, Config} + Init()
  main.go             // zconfig.Configure + routes + http.ListenAndServe
  provider.go         // interface Provider (Send/EstimateCost/GetDeliveryStatus)
  routing.go          // SelectProvider(), CombineScore() — fonctions pures
  scorer.go           // Laplace() — fonction pure
  message.go          // struct Message + Transition() — machine à états
  campaign.go         // struct Campaign + Transition() — machine à états

  providers/          // implémentations de l'interface Provider
    sms.go
    whatsapp.go
    telegram.go

  messages/           // endpoints /messages
    send.go           // POST   /messages
    get.go            // GET    /messages/:id
    list.go           // GET    /messages

  routing/            // endpoints /routing
    select.go         // POST   /routing/select
    estimate.go       // POST   /routing/estimate

  contacts/           // endpoints /contacts
    get_score.go      // GET    /contacts/:phone/score
    record_outcome.go // POST   /contacts/outcomes

  campaigns/          // endpoints /campaigns
    create.go         // POST   /campaigns
    schedule.go       // PATCH  /campaigns/:id/schedule
    cancel.go         // PATCH  /campaigns/:id/cancel
    import.go         // POST   /campaigns/:id/recipients
    get.go            // GET    /campaigns/:id
    list.go           // GET    /campaigns

  analytics/          // endpoints /analytics
    get_kpis.go       // GET    /analytics/kpis
    get_timeseries.go // GET    /analytics/timeseries

  wallet/             // endpoints /wallet
    get.go            // GET    /wallet/:workspaceId
    transactions.go   // GET    /wallet/:workspaceId/transactions
    deduct.go         // POST   /wallet/:workspaceId/deduct
    topup.go          // POST   /wallet/:workspaceId/topup

  webhooks/           // callbacks entrants
    om.go             // POST   /webhooks/om
    mtn.go            // POST   /webhooks/mtn
    telegram.go       // POST   /webhooks/telegram
```

### Structure interne des workers

```
src/core-processor/
  service.go              // struct Service{DB, Logger, AMQP} + Init()
  consumer.go             // connect AMQP + dispatch par type d'événement
  on_message_delivered.go // message.delivered → UPDATE status=delivered
  on_message_failed.go    // message.failed → UPDATE status=failed, emit wallet.refund
  main.go

src/intelligence-processor/
  service.go
  consumer.go
  on_message_delivered.go // → score contact + compteurs campaign + agrégat analytics
  on_message_failed.go    // → score contact + compteurs campaign + agrégat analytics
  on_campaign_run.go      // → envoie les messages d'un run via src/api HTTP
  campaign_scheduler.go   // ticker : démarre les campagnes scheduled
  analytics_refresh.go    // ticker : REFRESH MATERIALIZED VIEW analytics.kpi_daily
  main.go
```

---

## Règles transverses pour l'implémentation

1. **1 fichier par endpoint** — chaque route HTTP a son propre fichier dans le sous-dossier du domaine.
2. **1 fichier par handler d'événement** — chaque type d'événement RabbitMQ a son propre fichier dans le worker.
3. **Accès DB direct** — `s.DB.Select(ctx, &result, "SELECT ...")` dans le handler, pas de repository.
4. **`zconfig` pour la DI** — struct tags `key:""` et `inject:""`, pas de composition root manuelle.
5. **Libs partagées obligatoires** — tout code Go utilise `fleece/src/go/sql`, `fleece/src/go/log`, etc.
6. **Abstractions uniquement si justifiées** :
   - Interface `Provider` → oui (SMS/WhatsApp/Telegram sont interchangeables)
   - Machine à états `Message`/`Campaign` → oui (transitions complexes à tester)
   - Fonctions pures `CombineScore`/`Laplace` → oui (testables isolément)
   - Repositories, ports input/output, use case structs → **non**
7. **`search_path` par requête** — `src/api` accède à tous les schémas ; préfixer les tables
   (`messaging.messages`, `routing.provider_scores`, etc.) ou SET LOCAL search_path par handler.
8. **go.mod** — ajouter `sqlx`, `lib/pq`, `zconfig` ; stdlib reste la base.
9. **Tests** — chaque handler a au moins 1 test (table-driven, httptest), chaque fonction pure
   a ses tests unitaires. Workers testés via consumer mock.

---

## Statut des tâches

Légende : 🔴 Backlog · 🔵 En cours · ✅ Done · ❌ Bloqué

---

## Phase 0 — Libs Go partagées `src/go/`

> Fondation de toute l'implémentation. Sans ces libs, aucune Phase suivante ne peut démarrer.
> Agent : `fleece-go-engineer` pour chaque tâche, `fleece-qa-engineer` après chaque groupe.

| ID | Tâche | Agent | Dépend de | Statut |
|----|-------|-------|-----------|--------|
| M-001 | `src/go/sql/` — wrapper sqlx : `DB` (Select/Get/Exec/Begin/Close), `Tx` (Select/Get/Exec/Commit/Rollback), `Status` string type. Pattern exact de `winmarket/src/go/sql/`. Ajouter `sqlx` + `lib/pq` au `go.mod`. | go-engineer | — | ✅ |
| M-002 | `src/go/log/` — logger structuré slog : `Logger` struct + `Init()` (format json/text, level), `With()`, `WithContext()`. Pattern exact de `winmarket/src/go/log/`. | go-engineer | — | ✅ |
| M-003 | `src/go/syncx/` — `Map[Input, Output]()` concurrent avec semaphore. Copie directe de `winmarket/src/go/syncx/syncx.go`, module renommé `fleece`. | go-engineer | — | ✅ |
| M-004 | `src/go/amqp/` — connexion RabbitMQ partagée : `Conn` struct (DSN, Logger via zconfig), `Channel()`, `Consume()`, `Publish()`, `Close()`. Basé sur `github.com/rabbitmq/amqp091-go`. | go-engineer | M-001, M-002 | ✅ |
| M-005 | `src/go/app/` — enrichir l'existant : adopter le pattern winmarket (`Name`/`Version` injectés, `Context()` root avec cancel SIGINT/SIGTERM, `Cleanup()` par réflexion). Ajouter `github.com/synthesio/zconfig` au `go.mod`. | go-engineer | M-001, M-002 | ✅ |
| M-006 | QA Phase 0 : `go build ./src/go/...`, `go vet ./src/go/...`, `go test ./src/go/...` — tests unitaires pour sql (mock driver), log (writer), syncx (cas nominaux + annulation), amqp (mock). 0 régression monorepo. | qa-engineer | M-001..M-005 | ✅ |

> **Verdict Phase 0 : PASS** (2026-07-14). 4 libs créées (`gosql`, `golog`, `syncx`, `goamqp`) + `app` enrichi (zconfig/Bootstrap/Cleanup/Context). `go.mod` : sqlx v1.4.0, lib/pq v1.10.9, amqp091-go v1.10.0, zconfig v1.4.1 (tidy stable). `go build`/`go vet ./src/...` exit 0, 0 régression (8 appelants legacy de `Bootstrap(string)` préservés via rétrocompat). 13 tests verts (syncx 8, app 5) + smoke test d'importabilité des 5 packages (`src/go/syncx/import_smoke_test.go`, conservé). QA M-006 initialement FAIL sur 1 écart (descripteur `src/go/app/pkg` absent) → **soldé par le PM** (`src/go/app/pkg` = `type=go` créé), critère 7 re-vérifié 5/5 OK, build/vet/test re-verts. **NON COMMITÉ** (le coordinateur committera).

---

## Phase 1 — Scaffold `src/api/` (squelette + providers + domaine)

> Crée la coquille du service unifié avec les abstractions conservées.
> Agent : `fleece-go-engineer`, revue `fleece-architect-reviewer` en fin de phase.

| ID | Tâche | Agent | Dépend de | Statut |
|----|-------|-------|-----------|--------|
| M-007 | `src/api/service.go` + `src/api/main.go` + `src/api/pkg` — struct `Service{DB *gosql.DB, Logger *golog.Logger, AMQP *goamqp.Conn, Config ApiConfig}` + `Init()` via zconfig. Routeur HTTP stdlib (`http.ServeMux`). Port 8080. Aucun endpoint encore. `go build ./src/api` doit passer. | go-engineer | M-005 | 🔴 |
| M-008 | `src/api/provider.go` — interface `Provider{Send/EstimateCost/GetDeliveryStatus}`. `src/api/providers/sms.go`, `whatsapp.go`, `telegram.go` — migrer depuis `src/provider/internal/adapters/providers/` (stubs + TODO(production) conservés). Registry `map[string]Provider` dans `Service`. | go-engineer | M-007 | 🔴 |
| M-009 | `src/api/message.go` — struct `Message` + `Transition()` machine à états (migré depuis `src/messaging/internal/domain/`). `src/api/campaign.go` — struct `Campaign` + `Transition()` (migré depuis `wip/T-018-campaign`). Tests unitaires des machines à états. | go-engineer | M-007 | 🔴 |
| M-010 | `src/api/routing.go` — `SelectProvider(providers []ProviderScore, strategy string) (string, error)` + `CombineScore(providerScore, contactScore int) int` (migrés depuis `src/routing/`). `src/api/scorer.go` — `Laplace(success, failure int) int` (migré depuis `src/contact-intelligence/`). Tests unitaires. | go-engineer | M-007 | 🔴 |
| M-011 | QA + revue architecture Phase 1 : `go build/vet/test ./src/api/...`, interface `Provider` vérifiée (pas de leak infra dans domain), machines à états testées, fonctions pures testées, 0 import cross-service. | qa-engineer + architect-reviewer | M-007..M-010 | 🔴 |

---

## Phase 2 — Endpoints `src/api/`

> Implémente tous les endpoints HTTP. Chaque tâche = un sous-dossier = un agent.
> Pattern : handler reçoit `*Service`, accès DB direct via `s.DB.Select/Get/Exec`.
> Pas de repository. Un fichier par route.

| ID | Tâche | Agent | Dépend de | Statut |
|----|-------|-------|-----------|--------|
| M-012 | `src/api/messages/` — `send.go` (POST /messages : valider, `SelectProvider()`, `INSERT messaging.messages`, publier event AMQP), `get.go` (GET /messages/:id), `list.go` (GET /messages). Tests httptest. | go-engineer | M-008, M-009, M-010 | 🔴 |
| M-013 | `src/api/routing/` — `select.go` (POST /routing/select : lire `routing.provider_scores`, appeler `CombineScore` si `highest_delivery`, retourner sélection), `estimate.go` (POST /routing/estimate : lire pricing, calculer coût). Tests httptest. | go-engineer | M-010 | 🔴 |
| M-014 | `src/api/contacts/` — `get_score.go` (GET /contacts/:phone/score : lire `contact_intel.contact_channel_scores` + `contacts`, retourner score Laplace ou prior 50 si inconnu, jamais 404), `record_outcome.go` (POST /contacts/outcomes : UPSERT `ON CONFLICT (phone,channel)` + compteurs `contacts` en une tx). Tests httptest. | go-engineer | M-009, M-010 | 🔴 |
| M-015 | `src/api/campaigns/` — `create.go`, `schedule.go`, `cancel.go` (machines à états `Campaign.Transition()`), `import.go` (INSERT recipients `ON CONFLICT DO NOTHING`), `get.go`, `list.go`. Estimation coût via `s.DB` (routing.provider_pricing) + `SelectProvider()`. Tests httptest. | go-engineer | M-009, M-010, M-013 | 🔴 |
| M-016 | `src/api/analytics/` — `get_kpis.go` (GET /analytics/kpis : SELECT depuis `analytics.kpi_daily` avec filtres workspace/période/canal/pays), `get_timeseries.go` (GET /analytics/timeseries). Tests httptest. | go-engineer | M-011 | 🔴 |
| M-017 | `src/api/wallet/` — `get.go` (GET /wallet/:workspaceId : SELECT balance depuis `wallet.wallets`), `transactions.go` (GET transactions, migré depuis `src/wallet/`), `deduct.go` (POST deduct : CHECK balance >= amount en tx), `topup.go` (POST topup). Tests httptest. | go-engineer | M-011 | 🔴 |
| M-018 | `src/api/webhooks/` — `om.go` (POST /webhooks/om : callback Orange Money → UPDATE `wallet.transactions` status), `mtn.go` (POST /webhooks/mtn : callback MTN MoMo), `telegram.go` (POST /webhooks/telegram : DLR Telegram → publier event AMQP `message.delivered/failed`). Tests httptest. | go-engineer | M-012 | 🔴 |
| M-019 | QA Phase 2 : `go build/vet/test ./src/api/...`, tous les handlers testés via httptest, 0 repository, 0 port input/output, accès DB direct confirmé par revue, `go.mod` stabilisé, 0 régression monorepo. | qa-engineer | M-012..M-018 | 🔴 |

---

## Phase 3 — Workers `src/core-processor/` et `src/intelligence-processor/`

> Pattern winmarket `transaction-processor` : struct Service + zconfig, consumer AMQP,
> un fichier par type d'événement, pas d'endpoint HTTP (sauf /health optionnel).

| ID | Tâche | Agent | Dépend de | Statut |
|----|-------|-------|-----------|--------|
| M-020 | `src/core-processor/` — `service.go` (Service struct zconfig), `consumer.go` (connect AMQP, dispatch par routing key), `on_message_delivered.go` (UPDATE `messaging.messages` status=delivered, incrémente `wallet` si remboursement), `on_message_failed.go` (UPDATE status=failed, emit wallet.refund si applicable), `main.go`. Tests avec consumer mock. | go-engineer | M-004, M-005, M-012 | 🔴 |
| M-021 | `src/intelligence-processor/` — `service.go`, `consumer.go`, `on_message_delivered.go` (UPSERT `contact_intel.contact_channel_scores` + MAJ compteurs `contacts` + UPSERT `analytics.message_daily` + MAJ compteurs `campaign.campaign_runs` en **une seule tx** si message_id connu), `on_message_failed.go` (même logique, statut failed), `on_campaign_run.go` (appel HTTP src/api pour envoyer les messages d'un run, rate limit token bucket), `campaign_scheduler.go` (ticker : SELECT campaigns WHERE status='scheduled' AND scheduled_at<=now(), UPDATE atomique anti-double-run, emit event), `analytics_refresh.go` (ticker REFRESH MATERIALIZED VIEW analytics.kpi_daily CONCURRENTLY), `main.go`. Tests avec mocks. | go-engineer | M-004, M-005, M-015, M-016 | 🔴 |
| M-022 | QA Phase 3 : build/vet/test des deux workers, consumers testés (événements nominaux + erreurs + double-traitement), schedulers testés (anti-double-run), 0 appel SQL cross-schéma direct (D11), 0 régression monorepo. | qa-engineer | M-020, M-021 | 🔴 |

---

## Phase 4 — Suppression des anciens services Go

> Ne supprimer qu'après que src/api + workers passent QA complète.

| ID | Tâche | Agent | Dépend de | Statut |
|----|-------|-------|-----------|--------|
| M-023 | Supprimer `src/messaging/`, `src/routing/`, `src/provider/`, `src/wallet/`, `src/webhook/`, `src/contact-intelligence/`, `src/campaign/` (+ branche `wip/T-018-campaign`), `src/analytics/` (si existant). Vérifier que `go build ./src/...` passe encore. | go-engineer | M-019, M-022 | 🔴 |
| M-024 | Mettre à jour `deploy/k8s/` : remplacer les 8 Deployments/Services/HPAs par 3 (api, core-processor, intelligence-processor). Mettre à jour les ConfigMaps URLs internes (toutes les URLs services Go → `http://api:<port>`). Mettre à jour `.github/workflows/ci.yml`. | devops-engineer | M-023 | 🔴 |
| M-025 | Mettre à jour `src/graphql-api` et `src/rest-api` : les clients REST internes pointent désormais tous vers `src/api` (une seule URL `API_URL`). Supprimer les URLs séparées (MESSAGING_API_URL, ROUTING_API_URL, etc.). | ts-engineer | M-023 | 🔴 |

---

## Phase 5 — QA de régression globale

| ID | Tâche | Agent | Dépend de | Statut |
|----|-------|-------|-----------|--------|
| M-026 | Suite d'acceptance migration : reprendre les critères des T-001..T-019 et vérifier qu'ils passent sur la nouvelle architecture (go build/vet/test, endpoints répondent correctement, workers traitent les événements, migrations DB inchangées). Verdict PASS/FAIL global. | qa-engineer | M-023, M-024, M-025 | 🔴 |

---

## Phase 6 — Drizzle comme source de vérité du schéma TS (optionnel, parallélisable)

> Indépendant des phases Go. Peut démarrer dès que Phase 0 est Done.

| ID | Tâche | Agent | Dépend de | Statut |
|----|-------|-------|-----------|--------|
| M-027 | `src/ts/model/schema.ts` — convertir les migrations `0001..0003` + `0006` (schémas `identity`) en définitions Drizzle (tables, relations, enums). Configurer `drizzle-kit generate` pour produire du SQL compatible Atlas. Baseline = état actuel de la DB. | ts-engineer | — | 🔴 |
| M-028 | Valider que `drizzle-kit generate` produit un DDL identique (sémantiquement) aux migrations SQL existantes. Intégrer dans `Makefile` (`make migrate pkg=model`). | ts-engineer + db-engineer | M-027 | 🔴 |
| M-029 | Mettre à jour `src/auth-api` et `src/graphql-api` pour importer le schéma depuis `src/ts/model` et utiliser Drizzle comme query builder (déjà le cas pour graphql-api — vérifier la cohérence). | ts-engineer | M-027 | 🔴 |

---

## Ordre d'exécution recommandé

```
Phase 0  →  Phase 1  →  Phase 2 (tâches parallèles M-012..M-018)
                                        │
                                        ▼
                                    Phase 3 (workers)
                                        │
                                        ▼
                                    Phase 4 (nettoyage)
                                        │
                                        ▼
                                    Phase 5 (QA globale)

Phase 6 (Drizzle TS) : parallèle à partir de Phase 0, indépendante.
```

**Dans Phase 2, priorité d'exécution :**
1. M-012 (messages) — déblocante pour M-018 (webhooks) et M-020 (core-processor)
2. M-013 (routing) — déblocante pour M-015 (campaigns)
3. M-014 (contacts) + M-017 (wallet) — parallèles
4. M-015 (campaigns) + M-016 (analytics) — après M-013
5. M-018 (webhooks) — après M-012

---

## Travail existant réutilisé

| Artefact existant | Réutilisé dans |
|---|---|
| `src/provider/internal/adapters/providers/{sms,whatsapp,telegram}.go` | `src/api/providers/` (déplacement) |
| `src/messaging/internal/domain/` — entity + state machine | `src/api/message.go` (simplifié) |
| `src/routing/internal/domain/` — strategies, CombineScore | `src/api/routing.go` |
| `src/contact-intelligence/internal/domain/score.go` — Laplace | `src/api/scorer.go` |
| `wip/T-018-campaign` — domain Campaign + usecases | `src/api/campaign.go` + `src/api/campaigns/` |
| `migrations/0001..0017.sql` | Inchangées — les schémas DB ne bougent pas |
| `src/wallet/internal/application/usecases/list_transactions.go` | `src/api/wallet/transactions.go` |
| `src/go/app/` | Enrichi en Phase 0 (M-005) |

---

## Dettes et points de vigilance transverses

| Ref | Point | Impact |
|-----|-------|--------|
| D-M01 | `search_path` multi-schéma dans `src/api` : préfixer toutes les tables SQL (`messaging.messages`, `wallet.wallets`, etc.) plutôt que SET LOCAL (plus simple, pas de state). | Tous les handlers Phase 2 |
| D-M02 | `zconfig` nécessite que les variables d'env soient préfixées par le nom de la clé struct — documenter la convention dans `src/api/service.go`. | M-007 |
| D-M03 | `REFRESH MATERIALIZED VIEW CONCURRENTLY` exige que la MV ait un index UNIQUE — déjà présent dans `0017_analytics_kpi.sql`. Premier refresh doit être non-concurrent (MV vide au 1er boot). | M-021 |
| D-M04 | `atlas migrate lint` non disponible offline (Atlas Pro) — SKIP documenté, à faire en CI. | M-024 |
| D-M05 | Bot token Telegram : toujours en TODO(production) — à câbler avec Secret K8s en M-024. | M-018, M-024 |
| D-M06 | `src/rest-api` (gateway public TS) pas encore implémenté — non bloquant pour cette migration, planifié séparément (T-043..T-045 dans PROJECT_TRACKER). | M-025 |

---

## Journal d'exécution

| Date | Tâche | Verdict | Notes |
|------|-------|---------|-------|
| 2026-07-14 | M-001 `src/go/sql/` | Done | Package `gosql` (DB/Tx wrapper sqlx, `Exec` retourne `error` par contrat). go.mod +sqlx v1.4.0 +lib/pq v1.10.9. build/vet exit 0. |
| 2026-07-14 | M-002 `src/go/log/` | Done | Package `golog` (Logger slog, Init/With/WithContext, ContextKey). Stdlib-only, go.mod inchangé. build/vet exit 0. |
| 2026-07-14 | M-003 `src/go/syncx/` | Done | Package `syncx` (Map concurrent, signature `(ctx, inputs, concurrency, fn)`). Stdlib-only + 8 tests (dont -race). |
| 2026-07-14 | M-004 `src/go/amqp/` | Done | Package `goamqp` (Conn RabbitMQ, tags zconfig `key`/`inject`, Init/Channel/Publish/Consume/Close). go.mod +amqp091-go v1.10.0. build/vet exit 0. |
| 2026-07-14 | M-005 `src/go/app/` | Done | Enrichi : Bootstrap(interface{}) error / Cleanup(interface{}) / Context() (ctx,cancel) + config.go (zconfig). go.mod +zconfig v1.4.1. Rétrocompat `Bootstrap(string)` (8 appelants legacy intacts). 5 tests. |
| 2026-07-14 | M-006 QA Phase 0 | PASS | 9/9 critères après correction PM. Écart initial `src/go/app/pkg` absent → soldé (créé `type=go`). build/vet/test ./src/... verts, go.mod tidy stable, 5 packages importables. |
| 2026-07-14 | **Phase 0** | **PASS** | Fondation Go partagée livrée. Non commité (coordinateur). Phase 1 (scaffold `src/api/`, M-007..M-011) débloquée. |
