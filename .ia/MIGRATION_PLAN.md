# Plan de migration — Architecture simplifiée (inspiration winmarket)

> **Créé le :** 2026-07-14
> **Statut global :** ✅ Phase 0 · ✅ Phase 1 · ✅ Phase 2 · ✅ Phase 3 (PASS) · 🔵 Phase 4 partielle (M-024 ✅ · M-025 ✅ · **M-023 ⛔ EN ATTENTE DU FEU VERT UTILISATEUR**) — branche `migration/phase-3`
> **Bloquants avant Phase 5 :** (1) feu vert pour M-023 (suppression irréversible de 8 services Go + branche `wip/T-018-campaign`) ; (2) **D-M40** — le CRUD des webhooks sortants n'a jamais été migré, T-005 échouerait en QA de régression.
> **Dernière mise à jour :** 2026-07-27
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
| M-007 | `src/api/service.go` + `src/api/main.go` + `src/api/pkg` — struct `Service{DB *gosql.DB, Logger *golog.Logger, AMQP *goamqp.Conn, Config ApiConfig}` + `Init()` via zconfig. Routeur HTTP stdlib (`http.ServeMux`). Port 8080. Aucun endpoint encore. `go build ./src/api` doit passer. | go-engineer | M-005 | ✅ |
| M-008 | `src/api/provider.go` — interface `Provider{Send/EstimateCost/GetDeliveryStatus}`. `src/api/providers/sms.go`, `whatsapp.go`, `telegram.go` — migrer depuis `src/provider/internal/adapters/providers/` (stubs + TODO(production) conservés). Registry `map[string]Provider` dans `Service`. | go-engineer | M-007 | ✅ |
| M-009 | `src/api/message.go` — struct `Message` + `Transition()` machine à états (migré depuis `src/messaging/internal/domain/`). `src/api/campaign.go` — struct `Campaign` + `Transition()` (migré depuis `wip/T-018-campaign`). Tests unitaires des machines à états. | go-engineer | M-007 | ✅ |
| M-010 | `src/api/routing.go` — `SelectProvider(providers []ProviderScore, strategy string) (string, error)` + `CombineScore(providerScore, contactScore int) int` (migrés depuis `src/routing/`). `src/api/scorer.go` — `Laplace(success, failure int) int` (migré depuis `src/contact-intelligence/`). Tests unitaires. | go-engineer | M-007 | ✅ |
| M-011 | QA + revue architecture Phase 1 : `go build/vet/test ./src/api/...`, interface `Provider` vérifiée (pas de leak infra dans domain), machines à états testées, fonctions pures testées, 0 import cross-service. | qa-engineer + architect-reviewer | M-007..M-010 | ✅ |

> **Verdict Phase 1 : PASS** (2026-07-14). Service unifié `src/api/` (package `api`) scaffolé + abstractions conservées migrées. **Écart structurel** : `main.go` placé dans `src/api/cmd/api/main.go` (package `main`) car un répertoire Go = un seul package ; `src/api` reste `package api` importable (pattern lib+binaire standard). Impact DevOps : le build du binaire cible `./src/api/cmd/api` (à porter en M-024 Makefile/CI). **M-007** : `Service{DB,Logger,AMQP}` (tags `inject:""`) + `Init()`/`registerRoutes()` (GET /health) + `ServeHTTP`/`Close` ; DI manuelle dans le composition root (DB/AMQP ouverts explicitement). **M-008** : interface `Provider{Send/EstimateCost/GetDeliveryStatus}` + `ProviderResult{ExternalID,Status(string),Cost(int64 centimes)}` dans `provider.go` ; 3 impls `providers/{sms,whatsapp,telegram}.go` (assertions `var _ api.Provider`, stubs `net/http` + TODO(production), mapping `domain.DeliveryStatus`→string / `Money`→int64) ; `providers/registry.go` `BuildRegistry(ProviderConfig)` clés `sms-twilio`/`whatsapp-meta`/`telegram-bot` (= IDs base) ; câblage dans `cmd/api/main.go` (cycle sain : `providers`→`api`, `api` n'importe pas `providers`). **M-009** : machines à états `Message` (draft→pending→sent→delivered/failed/rejected) et `Campaign` (draft→scheduled[garde MessageBody]→running→completed/failed/paused/cancelled) — méthode `Transition()`, `ErrInvalidTransition` défini 1× (message.go) réutilisé, structs purs (errors/fmt/time). **M-010** : `CombineScore=clamp((ps+cs+1)/2,0,100)`, `SelectProvider(providers,strategy)` (highest_delivery=max score, lowest_cost=tri par `Cost int64` ajouté, round_robin=déterministe lexicographique — stateful réel en Phase 2), `Laplace(s,f)=clamp((s+1)*100/(s+f+2),0,100)` prior 50, `clamp` défini 1× (routing.go). **QA PASS 10/10** (build/vet/test `./src/api` + `./src/...` exit 0, 0 régression, `pkg`=type=go, interface+3 assertions, transitions valides ET invalides testées Message+Campaign, Laplace 50/66/91/8 + CombineScore 90/40→65 ; 15 fonctions de test top-level, centaines de sous-cas, 0 FAIL ; go.mod/go.sum inchangés). **architect-reviewer CONFORME 7/7** (interface au niveau `api` pas sous-package domain, 0 import des 8 anciens services, machines à états et fonctions pures sans I/O, cycle d'import sain, `providers/*` = stdlib+`fleece/src/api` seuls, conventions dépôt OK). Nitpick commentaire trompeur `service.go` (Init via Bootstrap) **soldé par le PM** (commentaire corrigé, re-build/vet/test/gofmt verts). **NON COMMITÉ** (le coordinateur committera). Dettes Phase 2 tracées ci-dessous (D-M07..D-M09).

---

## Phase 2 — Endpoints `src/api/`

> Implémente tous les endpoints HTTP. Chaque tâche = un sous-dossier = un agent.
> Pattern : handler reçoit `*Service`, accès DB direct via `s.DB.Select/Get/Exec`.
> Pas de repository. Un fichier par route.

| ID | Tâche | Agent | Dépend de | Statut |
|----|-------|-------|-----------|--------|
| M-012 | `src/api/messages/` — `send.go` (POST /messages : valider, `SelectProvider()`, `INSERT messaging.messages`, publier event AMQP), `get.go` (GET /messages/:id), `list.go` (GET /messages). Tests httptest. | go-engineer | M-008, M-009, M-010 | ✅ |
| M-013 | `src/api/routing/` — `select.go` (POST /routing/select : lire `routing.provider_scores`, appeler `CombineScore` si `highest_delivery`, retourner sélection), `estimate.go` (POST /routing/estimate : lire pricing, calculer coût). Tests httptest. | go-engineer | M-010 | ✅ |
| M-014 | `src/api/contacts/` — `get_score.go` (GET /contacts/:phone/score : lire `contact_intel.contact_channel_scores` + `contacts`, retourner score Laplace ou prior 50 si inconnu, jamais 404), `record_outcome.go` (POST /contacts/outcomes : UPSERT `ON CONFLICT (phone,channel)` + compteurs `contacts` en une tx). Tests httptest. | go-engineer | M-009, M-010 | ✅ |
| M-015 | `src/api/campaigns/` — `create.go`, `schedule.go`, `cancel.go` (machines à états `Campaign.Transition()`), `import.go` (INSERT recipients `ON CONFLICT DO NOTHING`), `get.go`, `list.go`. Estimation coût via `s.DB` (routing.provider_pricing) + `SelectProvider()`. Tests httptest. | go-engineer | M-009, M-010, M-013 | ✅ |
| M-016 | `src/api/analytics/` — `get_kpis.go` (GET /analytics/kpis : SELECT depuis `analytics.kpi_daily` avec filtres workspace/période/canal/pays), `get_timeseries.go` (GET /analytics/timeseries). Tests httptest. | go-engineer | M-011 | ✅ |
| M-017 | `src/api/wallet/` — `get.go` (GET /wallet/:workspaceId : SELECT balance depuis `wallet.wallets`), `transactions.go` (GET transactions, migré depuis `src/wallet/`), `deduct.go` (POST deduct : CHECK balance >= amount en tx), `topup.go` (POST topup). Tests httptest. | go-engineer | M-011 | ✅ |
| M-018 | `src/api/webhooks/` — `om.go` (POST /webhooks/om : callback Orange Money → UPDATE `wallet.transactions` status), `mtn.go` (POST /webhooks/mtn : callback MTN MoMo), `telegram.go` (POST /webhooks/telegram : DLR Telegram → publier event AMQP `message.delivered/failed`). Tests httptest. | go-engineer | M-012 | ✅ |
| M-019 | QA Phase 2 : `go build/vet/test ./src/api/...`, tous les handlers testés via httptest, 0 repository, 0 port input/output, accès DB direct confirmé par revue, `go.mod` stabilisé, 0 régression monorepo. | qa-engineer | M-012..M-018 | ✅ |

> **Verdict Phase 2 : PASS** (2026-07-26). 22 endpoints HTTP livrés dans le service unifié `src/api/`, testés et revus.
>
> **Écart structurel entériné** : la structure est **plate** (`messages_send.go` en `package api`) et non en sous-dossiers (`src/api/messages/send.go`) comme décrit dans le plan initial. Raison : en Go un sous-dossier = un package distinct, ce qui interdirait les méthodes sur `api.Service` et créerait un cycle `api` ↔ `messages`. La règle transverse n°1 est respectée au sens strict — **22 routes ↔ 22 fichiers, un handler par fichier** — via la convention de nommage `<domaine>_<action>.go`. Écart entériné, la Phase 3 ne doit pas le re-questionner.
>
> **Migrations additives hors plan initial (écart corrigé, non prévu)** : la QA a établi que M-018 était **structurellement irréalisable** sur le schéma existant. Deux migrations additives ont donc été ajoutées (db-engineer), non prévues au plan : `migrations/0018_messaging_dlr_correlation.sql` (`messaging.messages` += `provider_id text`, `external_id text`, `cost bigint` + index partiels `(external_id)` et `(provider_id, external_id) WHERE external_id IS NOT NULL`) et `migrations/0019_wallet_reconciliation.sql` (`wallet.wallet_transactions` += `status text NOT NULL DEFAULT 'completed'`, `reference_id text` + index partiel). Les deux sont **strictement additives** (`ADD COLUMN IF NOT EXISTS` / `CREATE INDEX IF NOT EXISTS`), aucune migration `0001..0017` n'est modifiée, `atlas migrate hash`/`validate` exit 0. Index volontairement **non-UNIQUE et partiels** (aucune garantie d'unicité inter-providers ; un webhook ne doit jamais échouer sur contrainte). Ces migrations soldent D-M10 et D-M11.
>
> **Contenu livré** — **M-012** messages (`messages_send.go` : validation → `SelectProvider` → INSERT `messaging.messages` → event AMQP ; `messages_get.go`, `messages_list.go`, `helpers.go` transverse). **M-013** routing (`routing_select.go` lit `routing.provider_scores` + `CombineScore` sur `highest_delivery` ; `routing_estimate.go` lit `routing.provider_pricing`). **M-014** contacts (`contacts_get_score.go` : `Laplace` ou prior 50, jamais 404 ; `contacts_record_outcome.go` : upsert transactionnel `ON CONFLICT (phone,channel)` + compteurs + historique en une `*gosql.Tx`). **M-015** campaigns (6 endpoints, machine à états `Campaign.Transition()`, mapping `ErrMissingMessageBody`→422 / `ErrInvalidTransition`→409, import `ON CONFLICT DO NOTHING`). **M-016** analytics (`analytics.kpi_daily` / `message_daily`, filtres workspace/période/canal/pays, requêtes dynamiques à placeholders numérotés — aucune interpolation de valeur, pas d'injection possible). **M-017** wallet (`wallet_deduct.go` : `SELECT … FOR UPDATE` + contrôle de solde + ledger en tx, 402 si insuffisant ; `wallet_topup.go` UPSERT + `reference_id` optionnel). **M-018** webhooks (Telegram : UPDATE `messaging.messages` par `external_id` + publication AMQP `message.delivered`/`message.failed` ; OM et MTN : HMAC-SHA256 en temps constant + `UPDATE wallet.wallet_transactions SET status WHERE reference_id`, fonctions pures `mapOMStatus`/`mapMTNStatus`, 200 systématique pour éviter les replays opérateur).
>
> **Correctifs appliqués avant clôture** (revue architecture, 2 bloquants + 6 écarts + nitpicks) : **B1** — le service s'auto-appelait en HTTP sur `http://localhost:8080/contacts/…` (host et port en dur) pour lire un score situé dans le même binaire → remplacé par `(s *Service) loadContactScore` in-process + fonction pure `enrichScoresWithContact` ; fallback gracieux D28 préservé (score indisponible ⇒ dégradation vers `provider_scores`, jamais d'échec du routage). **B2** — secrets HMAC `"change-me-in-production"` en dur et identiques OM/MTN sur un flux de paiement → sortis vers zconfig (`Service.OMWebhookSecret`/`MTNWebhookSecret`), politique explicite : secret configuré ⇒ signature obligatoire (401), secret vide ⇒ accepté + log WARN. **C1** interfaces de persistance ad hoc → `*gosql.Tx` concret. **C2/C3** code mort (`nopResponseWriter`, `buildContactIntelURL`) supprimé. **C4** `recoverMiddleware` global (panic → 500 JSON). **C5/D-M07** arrêt gracieux `http.Server` + `Shutdown(ctx)` + logging `golog` uniforme. **D-M13** contrat `external_id` Telegram incohérent (le stub produisait `tg<nanos>`, le webhook corrélait sur un entier décimal → toute corrélation DLR retournait 0 ligne silencieusement) → format aligné + contrat documenté des deux côtés + tests unitaires de `resolveTelegramCorrelation` ajoutés (règle 9). **D-M15** garde d'idempotence `AND status NOT IN ('completed','failed')` sur la réconciliation wallet (un callback tardif ne peut plus écraser un statut final). Harnais de test `reflect`+`unsafe` remplacé par `gosql.NewFromSQLX` exporté. `gosql.ExecRows` ajoutée à la lib partagée.
>
> **QA M-019 : PASS 10/10** (2 passes ; la 1re avait rendu FAIL 9/10 sur le seul critère 9 — webhooks OM/MTN sans persistance). build/vet/test `./src/api/...` et `./src/...` exit 0 ; 0 régression monorepo (21 packages `ok`, les 8 anciens services Go intacts) ; 22/22 routes couvertes httptest ; 0 repository / 0 port / 0 use case / une seule `interface` dans tout `src/api` (`Provider`, justifiée) ; 100 % des tables préfixées par leur schéma (D-M01) ; `go mod tidy` sans diff ; `gofmt` vide ; migrations 0018/0019 confrontées ligne à ligne au code (aucune colonne orpheline, aucune requête invalide). **architect-reviewer : CONFORME AVEC RÉSERVES 8,5/10** — aucune violation bloquante restante ; réserves converties en dettes Phase 3 ci-dessous. **Non commité** au moment de la revue ; commit de clôture effectué ensuite. Dettes ouvertes pour la Phase 3 : D-M12, D-M14, D-M16..D-M19 (et D-M13 partielle : unicité `(chat_id, message_id)`).

---

## Phase 3 — Workers `src/core-processor/` et `src/intelligence-processor/`

> Pattern winmarket `transaction-processor` : struct Service + zconfig, consumer AMQP,
> un fichier par type d'événement, pas d'endpoint HTTP (sauf /health optionnel).

| ID | Tâche | Agent | Dépend de | Statut |
|----|-------|-------|-----------|--------|
| M-020 | `src/core-processor/` — `service.go` (Service struct zconfig), `consumer.go` (connect AMQP, dispatch par routing key), `on_message_delivered.go` (UPDATE `messaging.messages` status=delivered, incrémente `wallet` si remboursement), `on_message_failed.go` (UPDATE status=failed, emit wallet.refund si applicable), `main.go`. Tests avec consumer mock. | go-engineer | M-004, M-005, M-012 | ✅ |
| M-021 | `src/intelligence-processor/` — `service.go`, `consumer.go`, `on_message_delivered.go` (UPSERT `contact_intel.contact_channel_scores` + MAJ compteurs `contacts` + UPSERT `analytics.message_daily` + MAJ compteurs `campaign.campaign_runs` en **une seule tx** si message_id connu), `on_message_failed.go` (même logique, statut failed), `on_campaign_run.go` (appel HTTP src/api pour envoyer les messages d'un run, rate limit token bucket), `campaign_scheduler.go` (ticker : SELECT campaigns WHERE status='scheduled' AND scheduled_at<=now(), UPDATE atomique anti-double-run, emit event), `analytics_refresh.go` (ticker REFRESH MATERIALIZED VIEW analytics.kpi_daily CONCURRENTLY), `main.go`. Tests avec mocks. **+ `on_message_sent.go` ajouté au périmètre (décision PM, voir journal).** | go-engineer | M-004, M-005, M-015, M-016 | ✅ |
| M-022 | QA Phase 3 : build/vet/test des deux workers, consumers testés (événements nominaux + erreurs + double-traitement), schedulers testés (anti-double-run), 0 appel SQL cross-schéma direct (D11), 0 régression monorepo. | qa-engineer | M-020, M-021 | ✅ |

> **Note sur le critère « 0 appel SQL cross-schéma direct (D11) »** : ce critère est **obsolète sous l'architecture unifiée** et a été réinterprété. D11 interdisait à un service d'interroger le schéma d'un *autre service*. Or la cible de cette migration est explicitement que les workers accèdent à plusieurs schémas (le plan lui-même prescrit pour `core-processor` « UPDATE `messaging.messages` … incrémente `wallet` », et pour `intelligence-processor` `contact_intel` + `analytics` + `campaign` **en une seule transaction**). Le critère est donc appliqué comme **D-M01** : 100 % des tables préfixées par leur schéma, aucune table nue. Vérifié : conforme.

---

> **Verdict Phase 3 : PASS** (2026-07-26). Deux workers Go livrés, revus **deux fois** par l'architecture, et corrigés sur 2 bloquants fonctionnels réels. Commité sur la branche **`migration/phase-3`** (et non sur `main`, contrairement aux Phases 0/1/2 — décision utilisateur).
>
> **Contenu livré** — **M-020** `src/core-processor/` : consumer AMQP (Ack/Nack explicites, `recover` + `debug.Stack()`), `on_message_delivered.go`, `on_message_failed.go` (remboursement **en transaction**, montant relu **en base** via `RETURNING`, jamais depuis le payload AMQP ; `kind='refund'` anticipé par 0002), composition root, `pkg`. **M-021** `src/intelligence-processor/` : 3 consumers `message.sent`/`delivered`/`failed` (contact_intel + analytics + campagne **en une seule transaction**), `on_campaign_run.go` (HTTP vers `src/api`, token bucket, réservation anti-double-envoi), `campaign_scheduler.go` (UPDATE atomique anti-double-run **en une seule requête**), `analytics_refresh.go` (D-M03 : refresh CONCURRENTLY avec repli non-concurrent, un échec ne tue jamais le worker). **M-022** QA.
>
> **Décisions du PM prises en ouverture de phase pour éviter la découverte tardive de M-018** (où une migration manquante avait coûté une passe de QA) : **(A)** `on_message_sent.go` **ajouté au périmètre** — lacune du plan : sans lui personne n'incrémente `analytics.message_daily.sent`, et tous les KPIs de la MV `kpi_daily` valent 0 (`delivery_rate = delivered/sent`). Répartition anti-double-comptage figée : `sent`/`cost` ← sent uniquement, `delivered`/`latency` ← delivered uniquement, `failed` ← failed uniquement. **(B)** `country` (`NOT NULL` **et dans la PK** de `analytics.message_daily`, absent de `messaging.messages`) dérivé par une **fonction pure** `CountryFromRecipient` plutôt que par une migration → **D-M23**. **(C)** exclusivité du remboursement à `core-processor` (deux workers qui créditent = double remboursement), garde-fou vérifié par un test qui scanne **toutes** les requêtes émises. **Aucune migration n'a été créée en Phase 3** (`migrations/` intact, dernier = 0019).
>
> **Deux dettes bloquantes tranchées avant M-020** : **D-M21** (découverte par le PM, non tracée au plan — le **vrai** bloquant, bien plus que D-M13) : l'événement `message.delivered`/`message.failed` publiait un `message_id` tantôt UUID Fleece, tantôt `external_id` Telegram, tantôt vide → aucun consumer ne pouvait corréler. Contrat figé : `message_id` = **toujours** l'UUID Fleece via `RETURNING id`, rien de publié si rien n'est corrélé. **D-M13** : voir la table des dettes — **mon arbitrage initial était faux et a été réfuté par la revue**.
>
> **1re revue d'architecture : NON CONFORME (7/10) — 2 bloquants fonctionnels**, tous deux issus de décisions du PM. **B1 (D-M26)** : le webhook écrivait le statut terminal **avant** de publier ⇒ la garde d'idempotence de `core-processor` renvoyait `rows == 0` **à 100 % des cas nominaux** ⇒ **le worker ne faisait rien** et n'aurait **jamais remboursé**. Ma « redondance idempotente acceptée » était en réalité une **neutralisation**. **B2 (D-M13)** : l'hypothèse `recipient == chat_id` est fausse (`SelectProvider` est agnostique du format du destinataire ; `@username` explicitement autorisé) ⇒ corrélation à 0 ligne **silencieuse** et **régression vs Phase 2**. Plus des écarts sérieux : crash-loop de requeue sur `channel` NULL, double envoi de campagne à impact monétaire, **topologie RabbitMQ déclarée nulle part** (exchange/queues inexistants ⇒ publications perdues sans le moindre log).
>
> **Round de correctifs complet** (arbitrage utilisateur) : B1 → **un seul écrivain** du statut, verrouillé par un test qui **échoue** si un `UPDATE` revient dans le webhook ; gardes alignées sur le graphe réel de `Message.Transition()`. B2 → **fix opportuniste** (stricte puis repli = requête Phase 2, couverture intégralement restaurée) + log ERROR `dlr_recipient_mismatch` ; vrai fix différé en **D-M27**. E2 → `sql.NullString` + repli `"unknown"` + `Qos`/prefetch. E6 → réservation `pending → sending` **avant** le POST. E9 → sortie en code non nul. **D-M29** → `src/go/amqp/topology.go`, déclaration idempotente dans les 3 binaires, bindings dérivés de `BoundRoutingKeys()` (source unique), `Publish` en `Persistent` + confirms retournant une **erreur**. **D-M17** → logger nil-safe dans les providers.
>
> **2e revue d'architecture : CONFORME AVEC RÉSERVES (8,5/10)** — les 2 bloquants **réellement levés et vérifiés** (pas seulement documentés), **aucun nouveau bloquant**, aucune abstraction injustifiée ajoutée. 4 nouveaux écarts tracés (D-M34, D-M35, D-M36, D-M37) dont 3 **introduits par les correctifs eux-mêmes** — coût assumé et documenté. **QA M-022 : PASS 17/18** (le 18e, E6, OK sur son objectif — le risque monétaire de double envoi est éliminé et prouvé par test HTTP réel — avec un résidu honnêtement documenté dans le code → **D-M31**).
>
> **Chiffres** : `go build`/`go vet ./src/...` exit 0 ; `go test ./src/... -count=1` → **0 FAIL, 24 packages ok**, **330 tests PASS** sur le périmètre workers+api+amqp ; `gofmt` vide ; `go mod tidy` sans diff ; 0 import de `fleece/src/api` ni des 8 anciens services ; 100 % des tables préfixées ; `migrations/` intact.
>
> **Découverte majeure hors périmètre → D-M36** : `POST /messages` **ne débite jamais le wallet** (aucune référence à `wallet` dans `messages_send.go`, aucun appelant de `/wallet/deduct` nulle part), alors que `core-processor` crédite au remboursement. Dès qu'un DLR de provider payant sera branché, **chaque échec créerait de la monnaie**. Inatteignable aujourd'hui (seul Telegram publie des DLR, `cost=0`), donc non bloquant — mais **à trancher avant D-M08**.

---

## Phase 4 — Suppression des anciens services Go

> Ne supprimer qu'après que src/api + workers passent QA complète.

| ID | Tâche | Agent | Dépend de | Statut |
|----|-------|-------|-----------|--------|
| M-023 | Supprimer `src/messaging/`, `src/routing/`, `src/provider/`, `src/wallet/`, `src/webhook/`, `src/contact-intelligence/`, `src/campaign/` (+ branche `wip/T-018-campaign`), `src/analytics/` (si existant). Vérifier que `go build ./src/...` passe encore. | go-engineer | M-019, M-022 | 🔴 |
| M-024 | Mettre à jour `deploy/k8s/` : remplacer les 8 Deployments/Services/HPAs par 3 (api, core-processor, intelligence-processor). Mettre à jour les ConfigMaps URLs internes (toutes les URLs services Go → `http://api:<port>`). Mettre à jour `.github/workflows/ci.yml`. | devops-engineer | M-023 | ✅ |

> **M-024 Done** (2026-07-27). Exécutée **avant M-023** (elle n'en dépend pas : c'est de la configuration, pas de la suppression ; aucun répertoire `src/*` n'a été touché).
>
> **K8s** : `messaging|provider|routing|wallet|webhook.yaml` supprimés → `api.yaml` (Deployment 3 replicas + Service ClusterIP 8080 + HPA CPU 70 %, 3-15), `core-processor.yaml` et `intelligence-processor.yaml` (**Deployment seul** — les workers n'exposent aucun port, donc ni Service ni sonde HTTP ; `restartPolicy: Always` suffit puisque E9 les fait sortir en code non nul). **`intelligence-processor` est volontairement à `replicas: 1` + `strategy: Recreate`** : `analytics_refresh` (`REFRESH MATERIALIZED VIEW`) n'est pas idempotent en exécution concurrente, et la garde d'idempotence des agrégats est locale (D-M25) — N réplicas amplifieraient le double-comptage. `core-processor` à 3 réplicas fixes (scalable en théorie, mais aucune métrique de profondeur de file n'est câblée ; un HPA CPU serait un proxy trompeur pour un backlog AMQP → recommandation KEDA `ScaledObject` sur `messages_ready`).
>
> **`POSTGRES_SEARCH_PATH` supprimé** pour `src/api` : le service couvre les 8 schémas via une seule connexion et préfixe explicitement chaque table (D-M01) ; une valeur unique n'aurait aucun sens. Vérifié : `cmd/api/main.go` ne lit jamais cette variable.
>
> **ConfigMap/Secret** : URLs consolidées en **`API_URL: http://api:8080`** (`AUTH_API_URL` conservée), cohérence vérifiée avec `src/graphql-api/infrastructure/config.ts` (`process.env["API_URL"]`) — coordination M-024/M-025 validée des deux côtés. `TELEGRAM_BOT_TOKEN`, `OM_WEBHOOK_SECRET`, `MTN_WEBHOOK_SECRET` déplacés vers `secret.example.yaml` (**D-M05 soldée**).
>
> **D-M09 soldée** — le bug était réel et silencieux : `go build -o x ./src/api` **exit 0 en produisant une archive `ar`**, pas un binaire. Fix dans `mk/go.mk` : `gobuildpath = $(if $(wildcard ./src/${pkg}/cmd/${pkg}),./src/${pkg}/cmd/${pkg},./src/${pkg})` — détection automatique, aucune variable par package, et la règle redeviendra triviale quand M-023 supprimera les anciens services. `docker/go.dockerfile` inchangé (il appelle `make build`, donc hérite du fix).
>
> **Validation plus poussée que prévu** : Docker était en fait disponible → les 4 images (`api`, `core-processor`, `intelligence-processor`, `messaging`) ont été **réellement construites et exécutées**. Les deux workers sortent proprement en **exit 1** sur env vide ; `api` sort en **exit 0** → confirme **D-M39** (non corrigée, hors périmètre DevOps).
>
> **D-M20 (partiellement soldée)** : `golangci-lint`/depguard non installables offline → **substitut explicite** en CI (`go list -deps` + grep sur les 8 anciens services + `src/api`), testé en positif **et** en négatif. Documenté comme substitut, pas comme remplacement définitif.
>
> **D-M35 (mitigée, non corrigée)** : initContainer `wait-for-amqp-topology` dans `api.yaml` — `api` ne démarre pas tant que les queues des workers ne sont pas visibles via l'API management RabbitMQ. C'est une **mitigation de déploiement** ; le vrai correctif reste côté code (`mandatory=true` + `NotifyReturn`, ou job de bootstrap dédié).
>
> **SKIP documentés** : `kubectl apply --dry-run=client` (kubectl v1.32 exige un appel réseau à l'API server pour la découverte de ressources ; aucun cluster local) → compensé par parsing YAML complet des 12 fichiers + vérification manuelle `containerPort` ↔ `PORT` ↔ `Service.port` ↔ URLs DNS. `atlas migrate lint` (D-M04).
| M-025 | Mettre à jour `src/graphql-api` et `src/rest-api` : les clients REST internes pointent désormais tous vers `src/api` (une seule URL `API_URL`). Supprimer les URLs séparées (MESSAGING_API_URL, ROUTING_API_URL, etc.). | ts-engineer | M-023 | ✅ |

> **M-025 Done** (2026-07-27). Exécutée **avant M-023** (elle n'en dépend pas réellement : c'est un changement de configuration, pas de suppression). `WALLET_API_URL`/`MESSAGING_API_URL`/`WEBHOOK_API_URL` → **`API_URL`** unique (défaut `http://localhost:8080`) ; `AUTH_API_URL` préservée (`auth-api` est TS, hors migration). 0 occurrence résiduelle dans `src/*.ts`. `tsc --noEmit` **0 erreur**, build esbuild OK, `go build ./src/...` inchangé. `src/rest-api` : rien à changer (non implémenté, D-M06).
>
> **La confrontation chemins TS ↔ routes `src/api/service.go` a payé** — elle a révélé des erreurs qu'un simple renommage d'URL aurait laissées passer : `GET /wallets/{id}/balance` → `GET /wallet/{id}` (pluriel erroné), `?workspaceId=` → `?workspace_id=`, et surtout le **trou fonctionnel D-M40** (CRUD webhooks sortants absent de `src/api`). Divergences de DTO corrigées : mapping snake_case réel des réponses wallet/messages, `message_id` nullable, pagination BFF sur tableau brut. Nouvelles dettes : **D-M40** (bloquante Phase 5), **D-M41** (scoping workspace), **D-M42** (pagination).

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
| D-M07 | ~~Composition root `src/api/cmd/api/main.go` : pas d'arrêt gracieux, logging mixte.~~ **SOLDÉE en Phase 2** : `http.Server` + `srv.Shutdown(ctx)` (timeout de drain 10 s, `ErrServerClosed` distingué via `errors.Is`), logging 100 % `golog`. | Clos |
| D-M08 | `providers/telegram.go` : `MapTelegramResponseStatus` (fonction pure testée) et les constantes de capabilities sont définies mais non appelées par `Send()` (stub). L'intégration HTTP réelle reste à faire. **Reportée en Phase 3.** | Phase 3 |
| D-M09 | ~~Build du binaire `src/api` : cible `./src/api/cmd/api` (et non `./src/api`).~~ **SOLDÉE en M-024.** Le défaut était **silencieux** : `go build -o x ./src/api` **exit 0** en écrivant une archive `ar` (package `api`), pas un exécutable — `make build pkg=api` semblait donc réussir sans jamais produire de binaire. Fix `mk/go.mk` : `gobuildpath = $(if $(wildcard ./src/${pkg}/cmd/${pkg}),./src/${pkg}/cmd/${pkg},./src/${pkg})`. Les 3 binaires + `messaging` (non-régression) validés par `make build` **et** par build/exécution d'images Docker réelles. | Clos |
| D-M10 | ~~`messaging.messages` sans `provider_id`/`external_id`/`cost` → corrélation DLR impossible.~~ **SOLDÉE en Phase 2** par `migrations/0018_messaging_dlr_correlation.sql`, câblée dans `insertMessage` et `webhooks_telegram.go`. | Clos |
| D-M11 | ~~`wallet.wallet_transactions` sans `status`/`reference_id` → webhooks OM/MTN incapables de réconcilier.~~ **SOLDÉE en Phase 2** par `migrations/0019_wallet_reconciliation.sql`, câblée dans les webhooks OM/MTN et `wallet_topup.go`. | Clos |
| D-M12 | **Couverture de test limitée par le pattern « `s.DB` nil »** : `newTestService()` laisse `DB`/`AMQP` à nil, donc la majorité des tests httptest n'exercent que validation, parsing et fonctions pures — pas le SQL réel. Un harnais de faux driver réutilisable existe désormais (`src/api/qa_m019_dbpath_test.go` via `gosql.NewFromSQLX`) et couvre les chemins DB des webhooks. À généraliser aux handlers messages/campaigns/wallet, ou à remplacer par des tests d'intégration contre un Postgres éphémère en CI. | Phase 3 / M-026 |
| D-M13 | **Corrélation DLR Telegram — ROUVERTE puis re-soldée autrement en Phase 3.** ⚠️ **Le raisonnement initial du PM était FAUX et est conservé ici comme garde-fou : ne pas le refaire.** J'avais tranché « `chat_id` est déjà stocké, donc `(chat_id, message_id)` ≡ `(recipient, external_id)` » au motif que `providers/telegram.go` documente `to` comme étant le chat_id et que `insertMessage` persiste ce `to` dans `messaging.messages.recipient`. **L'architect-reviewer a réfuté cette prémisse avec deux chemins réels** : (a) `SelectProvider` (`src/api/routing.go`) est **agnostique du format du destinataire** — un `POST /messages {"recipient":"+237690000000"}` peut être routé vers `telegram-bot`, et c'est **le cas normal des campagnes** (`on_campaign_run.go` poste des numéros E.164 importés) ; (b) `providers/telegram.go` autorise explicitement `@username`. Dans les deux cas `recipient ≠ chat.id` et le prédicat dur `AND recipient = $4` renvoyait **0 ligne silencieusement** — une **régression** vs Phase 2. **Correctif retenu (arbitrage utilisateur : fix opportuniste minimal)** : corrélation stricte `(external_id, provider_id, recipient=chat_id)` **puis repli** `(external_id, provider_id)` (= exactement la requête Phase 2, couverture intégralement restaurée), avec log **ERROR** `dlr_recipient_mismatch` quand le repli aboutit là où la stricte échoue. Le durcissement anti-collision inter-chats reste actif quand `recipient` *est* réellement le chat_id. Vrai fix → **D-M27**. | Clos (voir D-M27) |
| D-M14 | **Sémantique du top-up asynchrone** : `wallet_topup.go` crédite le solde immédiatement et laisse `status` au défaut `'completed'`, même lorsqu'un `reference_id` est fourni — alors que `0019` prévoit `'pending'` avant callback. Un callback `FAILED` passera la ligne à `'failed'` sur de l'argent déjà crédité. Décision produit + paiement requise (D8, V2). | Phase 3 |
| D-M15 | ~~Réconciliation wallet non idempotente.~~ **SOLDÉE en Phase 2** : garde `AND status NOT IN ('completed','failed')`. **Reste ouvert (mineur)** : pas de scoping par opérateur — une collision de `reference_id` entre OM et MTN réconcilierait la mauvaise ligne. | Phase 3 |
| D-M16 | `gosql.Tx` n'a pas l'équivalent de `DB.ExecRows` (nombre de lignes affectées) → asymétrie qui empêche le pattern de concurrence optimiste `UPDATE … WHERE balance >= $x` + contrôle des lignes dans `wallet_deduct.go` (qui doit passer par `Get` puis `Exec`). | Phase 3 |
| D-M17 | `src/api/providers/*.go` utilisent la stdlib `log` (`log.Printf`) au lieu de `fleece/src/go/log` — viole la règle transverse n°5. À corriger en même temps que le remplacement des stubs (D-M08). | Phase 3 / D-M08 |
| D-M18 | **Partiellement soldée en Phase 3.** Volet stack trace traité : les workers loggent `runtime/debug.Stack()` dans leur `recover()` par message, et `recoverMiddleware` (`src/api/service.go`) a été aligné. **Reste ouvert (mineur)** : `recoverMiddleware` est toujours réalloué à chaque requête au lieu d'être monté une fois dans `Init()`. | Phase 4 (cosmétique) |
| D-M19 | Nitpicks résiduels : `writeJSON` (`helpers.go`) avale l'erreur d'encodage sous un commentaire trompeur ; `reconcileWalletTransactionStatus` est une fonction libre prenant `*Service` en paramètre positionnel au lieu d'une méthode ; `enrichScoresWithContact` mute le slice en place tout en le retournant (double sémantique). | Phase 3 (cosmétique) |
| D-M20 | Aucune config `depguard` / `.golangci.yml` au dépôt : les règles de frontière (0 import cross-service, cycle `providers`→`api` sain) ne sont vérifiées que **manuellement** par l'architect-reviewer, alors que CLAUDE.md et ARCHITECTURE.md §7 affirment qu'elles sont outillées en CI. Dette pré-existante, à outiller. | M-024 (DevOps) |
| D-M21 | ~~**Contrat de l'événement `message.delivered`/`message.failed` ambigu**~~ — **dette découverte par le PM en ouverture de Phase 3, SOLDÉE en M-020.** Le seul producteur (`webhooks_telegram.go`, `publishTelegramDLR`) publiait `{"message_id": corr.Value}` où `corr.Value` valait tantôt le **message_id Telegram** (= `external_id`, entier décimal, chemin nominal), tantôt l'**UUID Fleece** (chemin de compatibilité `fleece_message_id`), tantôt la **chaîne vide** (aucune corrélation). Un consumer ne pouvait pas savoir ce qu'il recevait → **M-020 était réellement bloquée** (bien plus que par D-M13). Correctif : `message_id` vaut **toujours** l'UUID Fleece, obtenu par `UPDATE … RETURNING id` (une seule requête, pas de fenêtre de course) ; l'événement porte en plus `external_id`/`provider_id`/`source` ; **aucun événement n'est publié** si aucune ligne n'est corrélée (un événement non corrélable n'est traitable par aucun consumer et n'a aucune valeur d'audit). Contrat documenté côté producteur et côté consumers. | Clos |
| D-M23 | **`analytics.message_daily.country` dérivé par heuristique.** La colonne est `text NOT NULL` **et fait partie de la PK** `(day, workspace_id, country, channel)` (0009), or `messaging.messages` **n'a aucune colonne `country`**. Décision PM en ouverture de M-021 (pour éviter la découverte tardive qui a coûté une passe de QA en M-018) : **pas de migration**, une fonction pure `CountryFromRecipient(recipient) string` dérive le pays du préfixe E.164 (marchés PRD : Afrique francophone + Europe), longest-prefix match, fallback `"unknown"` (jamais `""`/NULL). Limite assumée : un destinataire Telegram est un chat_id numérique nu → toujours `"unknown"`. La source de vérité serait une colonne `country` sur `messaging.messages` (migration additive triviale) alimentée à l'envoi. | Phase 4 / V2 |
| D-M26 | ~~**`core-processor` était un no-op : le webhook désarmait sa garde d'idempotence.**~~ **SOLDÉE en Phase 3** (bloquant B1 de la revue d'architecture). `webhooks_telegram.go` écrivait le statut terminal **sans garde**, *puis* publiait ; à réception, la garde `status NOT IN (…)` des handlers renvoyait donc `rows == 0` **à 100 % des cas nominaux** → transition ignorée, **remboursement jamais exécuté**, aucun log d'anomalie. Invisible tant que Telegram est gratuit (`cost=0`), mais tout remboursement d'échec aurait été silencieusement perdu avec un provider payant. Correctif : **un seul écrivain** — le webhook résout en `SELECT` seul et publie ; `core-processor` est l'unique écrivain de `messaging.messages.status`. Verrouillé par `assertNoUpdateEmitted` (`qa_m019_dbpath_test.go`) qui **fait échouer** tout retour d'un `UPDATE` dans le webhook. Gardes alignées sur le graphe réel de `Message.Transition()` : `delivered` ⇐ `status='sent'` (seule arête entrante), `failed` ⇐ `status NOT IN ('delivered','failed','rejected')`. | Clos |
| D-M27 | **Le vrai `chat_id` Telegram n'est pas persisté.** Suite de D-M13 : la corrélation DLR repose sur un repli best-effort parce que `messaging.messages.recipient` n'est pas garanti égal au `chat.id`. Le fix honnête est de capturer le `result.chat.id` **renvoyé par `sendMessage`** dans `ProviderResult` (nouveau champ) et de le persister dans une colonne dédiée (migration additive), ce qui découplerait la clé technique de corrélation de la colonne métier `recipient`. Suppose de remplacer le stub Telegram → **lié à D-M08**. En attendant, surveiller le log ERROR `dlr_recipient_mismatch`. | Phase 4 / V2 |
| D-M28 | **Pas de DLQ, ni backoff, ni compteur `x-death`.** Toute erreur non classée « permanente » (JSON illisible, `message_id` non-UUID, routing key inconnue) est requeue-ée. Une erreur Postgres **déterministe** (violation de contrainte, cast invalide) boucle donc indéfiniment. Mitigation posée en Phase 3 : `Qos`/prefetch (`goamqp.DefaultPrefetch = 10`) borne le débit, et le crash-loop concret identifié (`channel` NULL scanné dans un `string`) est corrigé. Reste à faire : `x-dead-letter-exchange` + TTL de retry, ou bascule en erreur permanente au-delà de N tentatives. ⚠️ Ajouter les args DLQ à `DeclareQueue` **cassera les queues existantes** (`PRECONDITION_FAILED`) — à prévoir comme migration opérationnelle. | Phase 4 |
| D-M29 | ~~**Topologie RabbitMQ déclarée nulle part + publications non fiables.**~~ **SOLDÉE en Phase 3** (écart E3). Aucun `ExchangeDeclare`/`QueueDeclare`/`QueueBind` n'existait dans tout le dépôt, et `Publish` ouvrait/fermait un canal **sans publisher confirm ni `DeliveryMode: Persistent`** : exchange inexistant ⇒ le broker fermait le canal en 404 de façon asynchrone et `Publish` renvoyait `nil` ⇒ **tous les événements perdus sans une ligne de log**. Correctif : `src/go/amqp/topology.go` (`DeclareExchange`/`DeclareQueue`/`BindQueue`/`EnsureQueueBound`), déclaration idempotente au démarrage des 3 binaires (flags factorisés ⇒ aucun `PRECONDITION_FAILED` possible), bindings dérivés de `BoundRoutingKeys()` (source unique = `handlersByRoutingKey`), `Publish` en `Persistent` + confirms retournant une **erreur** si non confirmé. Résidus → D-M34, D-M35. | Clos |
| D-M31 | **Ligne de campagne piégée en `'sending'`.** Le correctif E6 (réservation `pending → sending` **avant** le `POST /messages`) élimine le risque de **double envoi** — objectif principal, prouvé par test HTTP réel (réservation à 0 ligne ⇒ 0 requête HTTP). Résidu : une ligne peut rester bloquée en `'sending'` sans issue si le worker meurt entre la réservation et le revert, **ou sur arrêt gracieux** (SIGTERM pendant le `POST` ⇒ `revertRecipientToPending` reçoit le **même `ctx` déjà annulé** ⇒ échoue). La reprise `WHERE status='pending'` ne la récupère jamais. Cas **routine à chaque rolling deploy**. Correctifs : revert sur `context.WithoutCancel(ctx)` + timeout court (ferme ~90 % de la fenêtre), et balayage périodique des `'sending'` anciens (à fusionner avec D-M30). | Phase 4 |
| D-M34 | **`Publish` peut bloquer sans borne dans un chemin de requête HTTP** (introduit par D-M29). `confirmation.WaitContext(ctx)` n'a d'autre sortie que `ctx`, et `src/api/cmd/api/main.go` monte un `http.Server` **sans `ReadTimeout`/`WriteTimeout`/`IdleTimeout`** ni `TimeoutHandler` — le contexte de requête n'est annulé que si le **client** coupe. Si RabbitMQ passe en alarme mémoire/disque (il bloque alors les publishers, TCP vivant, aucun ack), **tous les `POST /messages` pendent indéfiniment** ; avant ce round ils retournaient immédiatement. Interaction avec D-M31 : le timeout client de 10 s de `sendCampaignMessage` déclencherait un revert et **renverrait un message déjà parti**. Correctifs : borner l'attente dans `Publish` (`context.WithTimeout`, ~5 s), poser les timeouts `http.Server`, et publier les DLR post-corrélation sur un contexte **détaché** (`context.WithoutCancel`) pour qu'une coupure Telegram ne jette pas un DLR pourtant corrélé. | Phase 4 (haute) |
| D-M35 | **Messages non routables silencieusement détruits + doc `conn.go` inexacte.** Avec `mandatory=false`, RabbitMQ **ACK** les messages non routables : `ErrPublishNotConfirmed` ne couvre donc **pas** le cas « exchange sans queue liée », contrairement à ce qu'affirme le commentaire de `conn.go`. Or `src/api/cmd/api/main.go` ne déclare **que l'exchange**, jamais les queues ni les bindings : si `src/api` démarre avant le premier lancement de `core-processor`, tout DLR est détruit avec un `Publish` retournant `nil` — **sans** log `dlr_publish_lost`. Depuis D-M26 c'est la **seule** voie d'écriture du statut : la perte serait totale et muette. Correctifs : `mandatory=true` + `NotifyReturn`, ou déclarer queues et bindings depuis un job de bootstrap unique ; corriger la doc. | Phase 4 (haute) |
| D-M36 | **Débit wallet absent de `POST /messages` ⇒ le remboursement créerait de la monnaie.** Révélé par la levée de D-M26. `src/api/messages_send.go` ne contient **aucune** référence à `wallet` ; aucun appelant de `POST /wallet/{id}/deduct` n'existe (ni Go, ni `graphql-api`, ni `rest-api`), et aucun trigger SQL. Or `core-processor/on_message_failed.go` crédite `balance = balance + cost` à partir de `messaging.messages.cost`, et `SMSTwilio.EstimateCost` retourne déjà **25**. Dès qu'un DLR de provider payant sera branché, **chaque échec crédite un wallet jamais débité**. Aujourd'hui inatteignable (seul Telegram publie des DLR, `cost=0`) donc non bloquant, mais **à trancher avant D-M08** : décider où le débit a lieu (le plus cohérent : dans `HandleSendMessage`, dans la même transaction que l'INSERT). Corollaire : la justification de D-M31 dans `on_campaign_run.go` (« chaque POST /messages déclenche un débit wallet réel ») est **factuellement fausse** — à corriger. | Phase 4 (haute) |
| D-M37 | **Perte silencieuse d'un DLR sur message `pending`** (introduit par l'alignement des gardes sur la machine à états). `mapProviderStatus` (`messages_send.go`) retourne `"pending"` **par défaut** pour tout statut provider inconnu ; le vrai Twilio renvoie `queued`/`accepted`. Dès le remplacement des stubs (D-M08), un message partira en `status='pending'` **avec** un `external_id`, et son DLR `delivered` sera rejeté par la garde `WHERE status='sent'`. Le rejet est loggé en **INFO sans le statut réel** (`on_message_delivered.go`) : impossible de distinguer en production une idempotence bénigne d'une perte réelle. Correctif : `RETURNING`/second `SELECT status` dans la branche `rows==0` et **WARN** si le statut courant est non terminal. | Phase 4 |
| D-M38 | **Divergence d'agrégat assumée** : `intelligence-processor` comptabilise un `delivered` que `core-processor` refuse (message `pending`, cf. D-M37) ⇒ `analytics.message_daily.delivered` peut dépasser le nombre de lignes réellement en statut `delivered` dans `messaging.messages`. | Phase 4 (mineure) |
| D-M25 | **Limite résiduelle d'idempotence de `intelligence-processor`.** Contrairement à `core-processor` (dont l'`UPDATE … WHERE status NOT IN (…)` est naturellement idempotent), ce worker écrit des compteurs `+1` (analytics, campagne) et un score : une **redelivery AMQP** peut double-compter. Le worker ne peut pas utiliser le statut de `messaging.messages` comme garde, car `core-processor` fait le même UPDATE sur le même message — le premier des deux gagne et le second ne compterait jamais. Résolution partielle retenue : garde réelle sur `campaign_recipients.status` (table qui lui appartient) ; la limite subsiste sur les agrégats analytics/contact. Solution propre : table de marque d'idempotence portant `message_id` + `event_type` avec `INSERT … ON CONFLICT DO NOTHING` en tête de transaction (migration additive). Analyse détaillée en tête de `src/intelligence-processor/delivery_outcome.go`. **⚠️ Énoncé amendé après revue d'architecture** : la source **dominante** de duplication n'est pas la redelivery AMQP (fenêtre étroite) mais le **producteur** — (a) le webhook Telegram n'a aucune garde de statut et Telegram **rejoue ses updates jusqu'à 24 h**, chaque rejeu republiant un événement ; (b) `mapTelegramStatusToRoutingKey` mappe **`"delivered"` ET `"read"`** sur la même routing key, donc un message livré *puis lu* produit **deux** `message.delivered` légitimes (`delivery_rate` > 100 %). Conséquence : le facteur atténuant « fenêtre étroite » ne tient pas, et une table `processed_events(message_id, event_type)` **ne suffirait pas** pour le cas `delivered`+`read` (même `event_type`) — il faudrait dédupliquer sur l'`update_id` Telegram, présent dans le payload mais jamais propagé dans l'événement AMQP. Impact non monétaire (exclusivité wallet respectée). | Phase 4 / V2 |
| D-M30 | **Cycle de vie de campagne jamais fermé.** Aucun code n'écrit `campaign.campaigns.status = 'completed'`, ni `campaign_runs.finished_at`, ni `delivered_count`/`failed_count`. Une campagne lancée reste `running` indéfiniment, et la transition `running → completed` de `Campaign.Transition()` (abstraction volontairement conservée) n'est exercée par **personne**. Aggravant : `campaign_scheduler.go` passe *toutes* les campagnes dues à `running` (pas de `LIMIT`) **puis** traite les runs un par un — si `publishCampaignRun` échoue ou si le worker s'arrête entre l'UPDATE et l'INSERT, la campagne reste `running` **définitivement** (plus aucun tick ne la reprend, `WHERE status='scheduled'`), sans commande de reprise : résolution manuelle en base. Correctif : tick de reprise (`status='running' AND NOT EXISTS (run non terminé)`) ou patron **outbox** (republier depuis l'état DB au lieu d'une publication one-shot). | Phase 4 |
| D-M32 | **~150 lignes dupliquées entre `src/api/contacts_record_outcome.go` et `src/intelligence-processor/contact_score.go`** : `Laplace`, `clamp`, `reconstructSuccess` (inversion approchée de D26, ±1 pt), `upsertChannelScore` **et son SQL**, `upsertContact*` **et leurs UPSERT avec formule `delivery_score` recalculée en SQL`**. Le refus d'importer `fleece/src/api` depuis un worker est **correct** (couplage de déploiement inacceptable), mais « donc on duplique » est le mauvais second terme. **Divergence déjà constatée** : le worker **n'insère pas** dans `contact_intel.contact_channel_history` — un outcome issu d'un DLR ne laisse aucune trace d'historique, contrairement au même outcome poussé via `POST /contacts/outcomes`. Risque élevé : D26 est une dette ouverte dont la résolution devra toucher deux copies sans filet. Correctif : extraire `src/go/scoring/` (~120 lignes, zéro I/O) — `Laplace`, `clamp`, `ReconstructSuccess` + les **constantes de requête SQL** d'upsert (les fonctions restent chez l'appelant, elles prennent `*gosql.Tx`). Ne **pas** extraire pour `Laplace` seul : c'est le SQL dupliqué qui justifie la lib. | Phase 4 |
| D-M33 | **`campaign.campaign_recipients.message_id` n'est pas indexé.** `campaign_correlation.go` exécute `UPDATE … WHERE message_id = $2` **dans la transaction** de chaque `message.delivered`/`failed`, or les seuls index de la table (0016) sont `(campaign_id, recipient)` et `(campaign_id, status)` → **seq scan complet à chaque DLR**, verrous longs, dégradation quadratique avec le volume. Migration additive triviale : `CREATE INDEX … ON campaign.campaign_recipients (message_id) WHERE message_id IS NOT NULL`. | Phase 4 (db-engineer) |
| D-M40 | **🔴 TROU FONCTIONNEL DE LA MIGRATION — le CRUD des webhooks sortants a disparu.** Découvert en M-025 par confrontation des clients TS aux routes réellement enregistrées dans `src/api/service.go`. L'ancien service `src/webhook` (T-005, Done/PASS) exposait `POST /endpoints`, `GET /endpoints`, `DELETE /endpoints/{id}` — l'enregistrement des endpoints webhook **sortants** d'un workspace (URL + événements abonnés + secret HMAC). **`src/api` n'expose que les callbacks *entrants*** (`/webhooks/om|mtn|telegram`) : les 22 endpoints migrés en Phase 2 **ont omis ce domaine**. Le schéma `webhook.webhook_endpoints` existe toujours (0006/0010) mais n'a plus aucune surface HTTP. Conséquence : `src/graphql-api/adapters/clients/webhook.client.ts` appelle `/webhook-endpoints…` → **404 garanti**, et l'écran dashboard **DASH-04 (Webhooks)** serait non fonctionnel. Invisible aujourd'hui (clients TS = stubs offline). **Bloquant avant Phase 5** : la QA de régression T-001..T-019 doit inclure T-005. Décision requise : porter ces endpoints dans `src/api` (+ le dispatcher HMAC et le scheduler de retry, également absents), ou acter la perte de la fonctionnalité. | **Bloquant Phase 5** |
| D-M41 | **`GET /messages/{id}` ne filtre pas par workspace** (`src/api/messages_get.go`) : un identifiant connu permet de lire un message appartenant à **un autre workspace**. Le client TS passait déjà un `workspaceId` inopérant. Défaut hérité de l'ancien service, mais aggravé par l'unification (un seul service, tous les workspaces). À corriger avec un scoping obligatoire. | Phase 4 (sécurité) |
| D-M42 | **Pas de vraie pagination serveur sur `GET /messages`** : `messages_list.go` renvoie un **tableau brut** plafonné à `maxLimit=200`, sans curseur ni total. La pagination cursor est simulée côté BFF (M-025), donc aveugle au-delà de 200 éléments. Idem, `messaging.messages` n'a pas de colonne `updated_at` (0003) : `MessageDTO.updatedAt` est approximé par `created_at`. | Phase 4 / V2 |
| D-M39 | **`src/api/cmd/api/main.go` sort en code 0 sur erreur fatale.** E9 a été appliqué aux deux workers (`main() { os.Exit(run()) }`) mais **pas** à `src/api`, où le nouveau chemin fatal `DeclareExchange` (et les chemins Postgres/AMQP préexistants) font un simple `return` depuis `main()` ⇒ **exit 0** ⇒ K8s ne redémarre pas un pod pourtant non fonctionnel. Aligner sur le même pattern. | Phase 4 |

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
| 2026-07-14 | M-007 squelette `src/api/` | Done | `src/api/pkg` (type=go) + `service.go` (package api, Service+Init+GET /health) + `cmd/api/main.go` (package main, DI manuelle). build/vet/gofmt OK, go.mod inchangé. Écart : main en `cmd/api/` (1 package/répertoire) → D-M09. |
| 2026-07-14 | M-008 interface `Provider` + impls | Done | `provider.go` (Provider+ProviderResult) + `providers/{sms,whatsapp,telegram,registry,errors}.go` (assertions api.Provider, stubs+TODO(production), clés registry sms-twilio/whatsapp-meta/telegram-bot). Cycle sain, câblage main.go. build ./src/... OK. |
| 2026-07-14 | M-009 machines à états | Done | `message.go` (Message.Transition, 6 états) + `campaign.go` (Campaign.Transition + Schedule garde, 7 états) + tests table-driven (valides+invalides+terminaux). Structs purs, ErrInvalidTransition partagé. |
| 2026-07-14 | M-010 fonctions pures | Done | `routing.go` (SelectProvider 3 stratégies, CombineScore=clamp((ps+cs+1)/2), clamp) + `scorer.go` (Laplace prior 50) + tests (Laplace 50/66/91/8, CombineScore 90/40→65). scorer.go = 0 import. |
| 2026-07-14 | M-011 QA + revue archi | PASS | QA 10/10 (build/vet/test src/api + monorepo exit 0, 0 régression, transitions valides+invalides, cas canoniques Laplace/CombineScore). architect-reviewer CONFORME 7/7 (interface au niveau api, 0 import cross-service, fonctions pures sans I/O, cycle sain). Nitpick commentaire service.go soldé PM. Nitpicks Phase 2 → D-M07/D-M08/D-M09. |
| 2026-07-14 | **Phase 1** | **PASS** | Coquille `src/api/` + abstractions conservées (Provider, machines à états, fonctions pures) livrées et testées. Non commité (coordinateur). Phase 2 (endpoints M-012..M-019) débloquée. |
| 2026-07-25 | M-012 messages | Done | `messages_send.go` (validation → `SelectProvider` → INSERT `messaging.messages` → event AMQP), `messages_get.go`, `messages_list.go`, `helpers.go` (helpers transverses des 22 handlers, renommé depuis `messages_helpers.go`). Tests httptest. |
| 2026-07-25 | M-013 routing | Done | `routing_select.go` (lit `routing.provider_scores`, `CombineScore` sur `highest_delivery`), `routing_estimate.go` (lit `routing.provider_pricing`). Distinction `sql.ErrNoRows` vs erreur DB réelle (une panne ne peut plus devenir un coût 0 silencieux). |
| 2026-07-25 | M-014 contacts | Done | `contacts_get_score.go` (`Laplace` ou prior 50, jamais 404), `contacts_record_outcome.go` (upsert `ON CONFLICT (phone,channel)` + compteurs + historique dans une seule `*gosql.Tx`). |
| 2026-07-25 | M-015 campaigns | Done | 6 endpoints ; machine à états `Campaign.Transition()` réellement appelée (`ErrMissingMessageBody`→422, `ErrInvalidTransition`→409) ; import recipients `ON CONFLICT DO NOTHING`. Aucune logique métier dupliquée dans les handlers. |
| 2026-07-25 | M-016 analytics | Done | `analytics_get_kpis.go` / `analytics_get_timeseries.go` sur `analytics.kpi_daily` + `message_daily`. Requêtes dynamiques à placeholders numérotés uniquement — aucune interpolation de valeur utilisateur. |
| 2026-07-25 | M-017 wallet | Done | `wallet_get.go`, `wallet_transactions.go`, `wallet_deduct.go` (`SELECT … FOR UPDATE` + contrôle de solde + ledger en tx, 402 si insuffisant), `wallet_topup.go` (UPSERT + `reference_id` optionnel). |
| 2026-07-26 | M-018 webhooks | Done | Telegram : UPDATE `messaging.messages` par `external_id` + publication AMQP `message.delivered`/`message.failed`. OM/MTN : HMAC-SHA256 en temps constant + `UPDATE wallet.wallet_transactions SET status WHERE reference_id` (fonctions pures `mapOMStatus`/`mapMTNStatus`), 200 systématique anti-replay. **A nécessité les migrations 0018/0019** (voir verdict Phase 2). |
| 2026-07-26 | Migrations 0018/0019 | Done | Hors plan initial. `0018_messaging_dlr_correlation.sql` (`messaging.messages` += provider_id/external_id/cost + 2 index partiels) et `0019_wallet_reconciliation.sql` (`wallet.wallet_transactions` += status/reference_id + index partiel). Strictement additives, 0001..0017 intactes, `atlas hash`/`validate` exit 0. Soldent D-M10 et D-M11. |
| 2026-07-26 | Correctifs archi Phase 2 | Done | 2 bloquants soldés : **B1** auto-appel HTTP `localhost:8080` → `loadContactScore` in-process (fallback D28 préservé) ; **B2** secrets HMAC en dur → zconfig. Plus C1 (`*gosql.Tx` concret), C2/C3 (code mort), C4 (`recoverMiddleware`), C5/D-M07 (arrêt gracieux). |
| 2026-07-26 | M-019 QA Phase 2 (1re passe) | FAIL | 9/10. Seul écart : critère 9 — webhooks OM/MTN ne persistaient rien, `wallet.wallet_transactions` n'ayant ni `status` ni `reference_id`. Diagnostic exact (pas un bug de code) → migration additive décidée par le PM. |
| 2026-07-26 | D-M13 corrélation Telegram | Done | Bug fonctionnel réel trouvé par l'architect-reviewer et **manqué par la QA** : le stub provider produisait `tg<nanos>` là où le webhook corrélait sur un entier décimal → **tout DLR retournait 0 ligne silencieusement**. Format aligné, contrat documenté des deux côtés, tests unitaires de `resolveTelegramCorrelation` ajoutés (règle 9 : la fonction n'en avait aucun). |
| 2026-07-26 | D-M15 idempotence wallet | Done | Garde `AND status NOT IN ('completed','failed')` : un callback opérateur tardif ou rejoué ne peut plus écraser un statut final. Harnais de test `reflect`+`unsafe` remplacé par `gosql.NewFromSQLX` exporté. |
| 2026-07-26 | M-019 QA Phase 2 (2e passe) | PASS | 10/10. build/vet/test `./src/api/...` et `./src/...` exit 0, 0 régression (21 packages ok), 22/22 routes httptest, 0 repository/port/use case, 100 % des tables préfixées, `go mod tidy` sans diff, gofmt vide, migrations confrontées au code. |
| 2026-07-26 | Revue archi Phase 2 | CONFORME AVEC RÉSERVES | 8,5/10. Aucune violation bloquante restante. Réserves converties en dettes D-M12, D-M14, D-M16..D-M20 (+ D-M13 partielle : unicité `(chat_id, message_id)`). |
| 2026-07-26 | **Phase 2** | **PASS** | 22 endpoints HTTP livrés dans `src/api/`. Écart structurel plat entériné (22 routes ↔ 22 fichiers). Phase 3 (workers M-020..M-022) débloquée — **traiter D-M13 (clé `(chat_id, message_id)`) avant M-020**. |
| 2026-07-26 | D-M13 arbitrage PM | Done | **Tranchée avant M-020 sans migration ni db-engineer.** `chat_id` était déjà stocké : `providers/telegram.go` documente `to` comme étant le chat_id, et `insertMessage` le persiste dans `messaging.messages.recipient` → `(chat_id, message_id)` ≡ `(recipient, external_id)`, colonnes existantes (0003/0018). Évite une 3e migration hors plan. |
| 2026-07-26 | D-M21 (découverte PM) | Done | **Vrai bloquant de M-020, non tracé au plan.** L'événement `message.delivered`/`message.failed` publiait un `message_id` tantôt UUID Fleece, tantôt `external_id` Telegram, tantôt vide → aucun consumer ne pouvait corréler. Contrat figé : `message_id` = **toujours** l'UUID Fleece via `UPDATE … RETURNING id` ; rien n'est publié si aucune ligne n'est corrélée. |
| 2026-07-26 | M-020 core-processor | Done | `src/core-processor/` : `service.go`, `consumer.go` (Ack/Nack explicites, `recover`+`debug.Stack()`), `on_message_delivered.go`, `on_message_failed.go`, `cmd/core-processor/main.go`, `pkg`=type=go. Remboursement wallet **en transaction** et **uniquement si `rows==1`** (idempotence), montant relu **en base** (`RETURNING workspace_id, cost`) et jamais depuis le payload AMQP. `kind='refund'` (0002, anticipé par la migration). **Décision PM** : le worker écrit le remboursement en base et ne publie **pas** `wallet.refund` (un double chemin = risque de double crédit). Garde SQL `status NOT IN (…)` plutôt que `api.Message.Transition()` pour éviter d'embarquer tout le service HTTP dans le binaire. Solde D-M13, D-M21, et le volet stack trace de D-M18. |
| 2026-07-26 | M-021 intelligence-processor | Done | `src/intelligence-processor/` (15 fichiers de prod + 15 de test, **113 tests**). Consumers `message.sent`/`delivered`/`failed` en **une seule transaction** (contact_intel + analytics + campagne), `on_campaign_run.go` (HTTP vers `src/api`, token bucket), `campaign_scheduler.go` (UPDATE atomique anti-double-run en **une seule requête**), `analytics_refresh.go` (D-M03 : 1er refresh non-concurrent puis CONCURRENTLY, un échec ne tue jamais le worker). **Décision PM A** : `on_message_sent.go` **ajouté au périmètre** (lacune du plan) — sans lui, personne n'incrémente `analytics.message_daily.sent` et tous les KPIs de la MV valent 0. Répartition anti-double-comptage : `sent`/`cost` ← sent uniquement, `delivered`/`latency` ← delivered uniquement, `failed` ← failed uniquement. **Décision PM B** → D-M23 (`country` par fonction pure, pas de migration). **Décision PM C** : aucune écriture `wallet.*` ici (exclusivité core-processor), garde-fou testé. Limite d'idempotence assumée et documentée → **D-M25**. |
| 2026-07-26 | Interruption de session | Note | L'agent M-021 a été coupé par une limite de session après avoir livré tout le code de production (build/vet/gofmt verts) mais **avant d'écrire les tests** (`[no tests to run]`). Reprise ciblée sur la seule suite de tests, sans réécriture du code de production. |
| 2026-07-26 | Revue archi Phase 3 (1re passe) | **NON CONFORME** | 7/10. **2 bloquants fonctionnels, tous deux issus de décisions du PM.** **B1** : le webhook écrivait le statut terminal avant de publier ⇒ garde d'idempotence de `core-processor` à `rows == 0` sur **100 %** des cas nominaux ⇒ worker inopérant, remboursement jamais exécuté. **B2** : `recipient == chat_id` réfuté (`SelectProvider` agnostique du format ; `@username` autorisé) ⇒ corrélation 0 ligne silencieuse + **régression vs Phase 2**. Écarts : crash-loop `channel` NULL, double envoi de campagne (monétaire), topologie RabbitMQ inexistante, worker zombie. |
| 2026-07-26 | Round de correctifs | Done | B1 (un seul écrivain + gardes alignées sur `Message.Transition()`), B2 (repli + log ERROR `dlr_recipient_mismatch`), E2 (`sql.NullString` + `Qos`), E6 (réservation `sending`), E9 (exit non nul), D-M29 (`topology.go` + confirms + `Persistent`), D-M17 (logger nil-safe). 7 tests cassés par E6 réparés. Aucune migration créée. |
| 2026-07-26 | Revue archi Phase 3 (2e passe) | **CONFORME AVEC RÉSERVES** | 8,5/10. 2 bloquants **levés et vérifiés mécaniquement** (grep dépôt entier : seuls `core-processor/on_message_*.go` écrivent `messaging.messages.status`), verrouillés par `assertNoUpdateEmitted`. **Aucun nouveau bloquant**, aucune sur-ingénierie. 4 nouveaux écarts (D-M34/35/36/37), dont 3 introduits par les correctifs. A exigé la correction de la traçabilité : D-M13 était encore « Clos » avec la prémisse réfutée. |
| 2026-07-26 | M-022 QA Phase 3 | **PASS** | 17/18 stricts. build/vet exit 0 ; `go test ./src/...` **0 FAIL, 24 packages ok** ; **330 tests PASS** (workers+api+amqp) ; gofmt vide ; `go mod tidy` sans diff ; 0 import interdit (`go list -deps`) ; 100 % des tables préfixées ; `migrations/` intact. Critère 7 (E6) : OK sur son objectif (risque monétaire éliminé, prouvé par test HTTP réel `requestsReceived==0`), résidu `'sending'` documenté → D-M31. |
| 2026-07-26 | **Phase 3** | **PASS** | 2 workers livrés. Branche **`migration/phase-3`**, commit `a22e7b4` (56 fichiers, +8367 lignes). |
| 2026-07-27 | M-025 clients REST TS | Done | `graphql-api` → **`API_URL`** unique (défaut `http://localhost:8080`) ; `AUTH_API_URL` préservée ; 0 occurrence résiduelle dans `src/*.ts`. `tsc --noEmit` 0 erreur, build esbuild OK. **La confrontation systématique chemins TS ↔ routes `service.go` a payé** : `/wallets/` pluriel erroné, `?workspaceId=` camelCase, mapping snake_case des DTO — et surtout **D-M40**. `src/rest-api` : rien à faire (D-M06). |
| 2026-07-27 | M-024 DevOps | Done | K8s 5 manifestes Go → 3 (`api` + 2 workers sans Service ni sonde) ; `intelligence-processor` **replicas=1 + Recreate** (refresh MV non idempotent, D-M25). `POSTGRES_SEARCH_PATH` supprimé (multi-schéma). ConfigMap → `API_URL`, secrets HMAC + token Telegram vers Secret (**D-M05 soldée**). **D-M09 soldée** (`mk/go.mk`, bug silencieux : archive `ar` au lieu d'un binaire). **D-M20 partiellement soldée** (substitut `go list -deps` faute de golangci-lint offline). **D-M35 mitigée** (initContainer d'attente de topologie). Docker disponible → 4 images réellement construites et exécutées. |
| 2026-07-27 | D-M40 (découverte M-025) | 🔴 Ouvert | **Trou fonctionnel de la migration** : le CRUD des webhooks **sortants** (`POST/GET/DELETE /endpoints` de l'ancien `src/webhook`, T-005 Done/PASS) **n'a jamais été migré** en Phase 2. `src/api` n'expose que les callbacks entrants. Le schéma `webhook.webhook_endpoints` (0006/0010) n'a plus aucune surface HTTP ⇒ `webhook.client.ts` → **404**, écran DASH-04 mort. **Bloquant Phase 5** (M-026 doit rejouer T-005) et **remet en cause le périmètre de M-023** : supprimer `src/webhook` ferait perdre le seul code de référence (register endpoint, dispatcher HMAC, scheduler de retry). |
| 2026-07-27 | **Phase 4 (partielle)** | **M-024 + M-025 Done** | **M-023 NON LANCÉE** — suppression de 226 fichiers Go sur 8 services + branche `wip/T-018-campaign` (3684 lignes, poussée sur `origin`) : **en attente du feu vert explicite de l'utilisateur**. Recommandation du PM : **traiter D-M40 avant M-023**. Phase 5 (M-026) bloquée par M-023 **et** par D-M40. |
