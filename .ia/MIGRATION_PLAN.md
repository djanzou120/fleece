# Plan de migration — Architecture simplifiée (inspiration winmarket)

> **Créé le :** 2026-07-14
> **Statut global :** ✅ Phase 0 Done/PASS · ✅ Phase 1 Done/PASS · ✅ Phase 2 Done/PASS — Phase 3 (workers, M-020..M-022) débloquée
> **Dernière mise à jour :** 2026-07-26
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
| D-M07 | ~~Composition root `src/api/cmd/api/main.go` : pas d'arrêt gracieux, logging mixte.~~ **SOLDÉE en Phase 2** : `http.Server` + `srv.Shutdown(ctx)` (timeout de drain 10 s, `ErrServerClosed` distingué via `errors.Is`), logging 100 % `golog`. | Clos |
| D-M08 | `providers/telegram.go` : `MapTelegramResponseStatus` (fonction pure testée) et les constantes de capabilities sont définies mais non appelées par `Send()` (stub). L'intégration HTTP réelle reste à faire. **Reportée en Phase 3.** | Phase 3 |
| D-M09 | Build du binaire `src/api` : cible `./src/api/cmd/api` (et non `./src/api`) — le `main.go` est en sous-dossier `cmd/api/` (contrainte 1 package/répertoire Go). Adapter la ligne Makefile/`mk/go.mk` + CI. | M-024 (DevOps) |
| D-M10 | ~~`messaging.messages` sans `provider_id`/`external_id`/`cost` → corrélation DLR impossible.~~ **SOLDÉE en Phase 2** par `migrations/0018_messaging_dlr_correlation.sql`, câblée dans `insertMessage` et `webhooks_telegram.go`. | Clos |
| D-M11 | ~~`wallet.wallet_transactions` sans `status`/`reference_id` → webhooks OM/MTN incapables de réconcilier.~~ **SOLDÉE en Phase 2** par `migrations/0019_wallet_reconciliation.sql`, câblée dans les webhooks OM/MTN et `wallet_topup.go`. | Clos |
| D-M12 | **Couverture de test limitée par le pattern « `s.DB` nil »** : `newTestService()` laisse `DB`/`AMQP` à nil, donc la majorité des tests httptest n'exercent que validation, parsing et fonctions pures — pas le SQL réel. Un harnais de faux driver réutilisable existe désormais (`src/api/qa_m019_dbpath_test.go` via `gosql.NewFromSQLX`) et couvre les chemins DB des webhooks. À généraliser aux handlers messages/campaigns/wallet, ou à remplacer par des tests d'intégration contre un Postgres éphémère en CI. | Phase 3 / M-026 |
| D-M13 | **Corrélation DLR Telegram — partiellement soldée.** Le désalignement de format `external_id` entre `providers/telegram.go` et `resolveTelegramCorrelation` (toute corrélation retournait 0 ligne silencieusement) est **corrigé** en Phase 2, contrat documenté des deux côtés + tests unitaires ajoutés. **Reste ouvert** : le `message_id` Telegram n'est unique que **par chat** — la clé de corrélation exacte est `(chat_id, message_id)`, et `chat_id` n'est ni capturé ni stocké. À trancher (décision de schéma) **avant M-020**. | Phase 3 / M-020 |
| D-M14 | **Sémantique du top-up asynchrone** : `wallet_topup.go` crédite le solde immédiatement et laisse `status` au défaut `'completed'`, même lorsqu'un `reference_id` est fourni — alors que `0019` prévoit `'pending'` avant callback. Un callback `FAILED` passera la ligne à `'failed'` sur de l'argent déjà crédité. Décision produit + paiement requise (D8, V2). | Phase 3 |
| D-M15 | ~~Réconciliation wallet non idempotente.~~ **SOLDÉE en Phase 2** : garde `AND status NOT IN ('completed','failed')`. **Reste ouvert (mineur)** : pas de scoping par opérateur — une collision de `reference_id` entre OM et MTN réconcilierait la mauvaise ligne. | Phase 3 |
| D-M16 | `gosql.Tx` n'a pas l'équivalent de `DB.ExecRows` (nombre de lignes affectées) → asymétrie qui empêche le pattern de concurrence optimiste `UPDATE … WHERE balance >= $x` + contrôle des lignes dans `wallet_deduct.go` (qui doit passer par `Get` puis `Exec`). | Phase 3 |
| D-M17 | `src/api/providers/*.go` utilisent la stdlib `log` (`log.Printf`) au lieu de `fleece/src/go/log` — viole la règle transverse n°5. À corriger en même temps que le remplacement des stubs (D-M08). | Phase 3 / D-M08 |
| D-M18 | `recoverMiddleware` (`service.go`) ne logge pas de stack trace (`debug.Stack()`) → panique difficilement débuggable en production ; il est aussi réalloué à chaque requête au lieu d'être monté une fois dans `Init()`. | Phase 3 |
| D-M19 | Nitpicks résiduels : `writeJSON` (`helpers.go`) avale l'erreur d'encodage sous un commentaire trompeur ; `reconcileWalletTransactionStatus` est une fonction libre prenant `*Service` en paramètre positionnel au lieu d'une méthode ; `enrichScoresWithContact` mute le slice en place tout en le retournant (double sémantique). | Phase 3 (cosmétique) |
| D-M20 | Aucune config `depguard` / `.golangci.yml` au dépôt : les règles de frontière (0 import cross-service, cycle `providers`→`api` sain) ne sont vérifiées que **manuellement** par l'architect-reviewer, alors que CLAUDE.md et ARCHITECTURE.md §7 affirment qu'elles sont outillées en CI. Dette pré-existante, à outiller. | M-024 (DevOps) |

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
