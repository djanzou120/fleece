# Fleece — Project Tracker (mémoire de l'agent PM)

> **Propriétaire : `fleece-pm`.** Source de vérité du suivi projet : tâches, assignations,
> tests d'acceptance, blocages. Mis à jour à chaque intervention/changement d'état.
> Voir aussi `.ia/MEMORY.md` (décisions techniques) et `.ia/user-story.md` (critères d'acceptance).
>
> Dernière mise à jour : **2026-06-13** (clôture T-003 Routing).

## Légende
- **Statut** : `Backlog` · `Ready` · `In Progress` · `In Review` · `Blocked` · `Done`
- **Acceptance** : `—` (non testé) · `PASS` · `FAIL`
- **Phase** : 🟢 MVP (P0) · 🟡 V1 (P1) · 🔵 V2 (P2)

## Équipe d'agents
| Agent | Domaine |
|-------|---------|
| `fleece-pm` | Orchestration, suivi, acceptance, ce tracker |
| `fleece-go-engineer` | Services Go (cœur métier) |
| `fleece-ts-engineer` | `auth-api` (Identity/Better Auth), `graphql-api` (BFF) |
| `fleece-frontend-engineer` | Dashboard `platform-app` (Next.js) |
| `fleece-db-engineer` | Schéma + migrations Atlas |
| `fleece-devops-engineer` | Makefile/Docker/K8s/CI/infra |
| `fleece-qa-engineer` | Tests & exécution des critères d'acceptance |
| `fleece-architect-reviewer` | Garde-fous Clean Architecture (lecture seule) |

---

## État global (baseline)
Le scaffold de l'architecture est en place et compile (`go build ./src/...` OK ; `make build pkg=messaging` OK).
Les couches Clean Architecture existent pour tous les services Go et TS, mais la plupart des adapters/use cases
sont des squelettes (`doc.go` / `TODO`). Le service **messaging** est rempli comme référence.

---

## Backlog / Tâches

| ID | Tâche | Service / zone | Agent | Phase | Statut | Acceptance | Notes |
|----|-------|----------------|-------|-------|--------|-----------|-------|
| T-001 | Adapters concrets messaging (Postgres, RabbitMQ, HTTP, clients) | `src/messaging` | go-engineer | 🟢 | Done | PASS | Adapters C3 + infra C4 + composition root livrés (stdlib-only, go.mod inchangé). 1 violation de dépendance (adapters→infra via interface Broker) détectée puis corrigée (Broker rapatrié en C3 `adapters/messaging`). TODO(amqp)/driver pq différés. Tests adapters absents (couverture à renforcer plus tard). |
| T-002 | Implémenter Wallet (débit/refund/ledger) | `src/wallet` | go-engineer | 🟢 | Done | PASS | C1 Money/Wallet/Transaction + C2 use cases debit/credit/refund/get_balance + C3 persistence/publisher/http + interface Broker en C3 (`adapters/messaging`) + C4 config/postgres/rabbitmq(NoopBroker)/httpserver + composition root. Stdlib-only, go.mod/go.sum inchangés. 19/19 tests verts (15 domaine + 4 use case debit). Frontières CONFORMES (0 violation C3→C4). Dette : pas de tests use case credit/refund/get_balance ni adapters http/persistence (à renforcer). |
| T-003 | Implémenter Routing (stratégies + fallback) | `src/routing` | go-engineer | 🟢 | Done | PASS | C1 domaine pur (Money, Channel, Strategy, ProviderPricing/Score, RoutingRule, RoutingDecision+ProviderRef, `SelectProvider`) + C2 ports input/output + usecases get_routing_decision/update_provider_score + C3 persistence (schéma `routing`, colonnes exactes 0004)/http (POST /route, POST /scores) + C4 config(port 8083)/postgres/httpserver + composition root. Pas de broker (Routing ne publie pas). Stdlib-only, go.mod inchangé/go.sum absent. 35/35 tests verts (13 domaine + 5 use case + 17 handler ajoutés par QA). Frontières CONFORMES (0 violation C3→C4, isolation schéma). Écarts schéma/spec documentés : devise via config DefaultCurrency (XAF) car absente de provider_pricing ; `Fastest`→fallback score (pas de latence en 0004) ; `Custom`→fallback HighestDelivery (pas de config JSON) ; EstimatedCost unitaire (Messaging multiplie). |
| T-004 | Implémenter Provider + adapters WhatsApp/SMS | `src/provider` | go-engineer | 🟢 | Backlog | — | Port `Provider` (TDD §7). |
| T-005 | Implémenter Webhook (signature HMAC + retries) | `src/webhook` | go-engineer | 🟢 | Backlog | — | Événements message.*/wallet.*. |
| T-006 | Identity : workspaces, users, API Keys, Better Auth | `src/auth-api` | ts-engineer | 🟢 | Backlog | — | Better Auth confiné en adapter. |
| T-007 | BFF GraphQL : schéma + résolveurs + clients REST | `src/graphql-api` | ts-engineer | 🟢 | Backlog | — | Agrège Identity/Wallet/Messaging. |
| T-008 | Dashboard : onboarding, API Keys, Wallet, Webhooks | `src/platform-app` | frontend-engineer | 🟢 | Backlog | — | Stories DASH-01..04. Manque `mk/react.mk`. |
| T-009 | Job d'init K8s `atlas migrate apply` + manifests services | `deploy/`, `docker/` | devops-engineer | 🟢 | Backlog | — | + `mk/react.mk`, `docker/react.dockerfile`. |
| T-010 | Suite de tests d'acceptance MVP | transverse | qa-engineer | 🟢 | Backlog | — | Basée sur user-story.md (API-01..04, DASH-01..04). |

> Les tâches 🟡/🔵 (campaign, contact-intelligence, analytics, Telegram, SSO, Messenger, RCS) restent en backlog
> jusqu'à clôture du MVP.

---

## Journal d'acceptance
| Date | Tâche | Verdict | Détails |
|------|-------|---------|---------|
| 2026-06-13 | Baseline scaffold | PASS (partiel) | `go vet`/`go build ./src/...` OK ; `make build pkg=messaging` OK. Couches TS/DB/Docker en place. Adapters concrets à venir. |
| 2026-06-13 | T-001 (1re passe) | FAIL | Build/vet/tests OK (QA). Architect-reviewer: 2 violations C3→C4 — `adapters/publisher` et `adapters/consumer` importaient `infrastructure/rabbitmq` (interface `Broker` mal placée en C4). Renvoyé au go-engineer. |
| 2026-06-13 | T-001 (après correction) | PASS | Interface `Broker` rapatriée en C3 (`internal/adapters/messaging/broker.go`), `NoopBroker` reste en C4 avec assertion infra→C3 (sens autorisé). `go vet ./src/messaging/...` OK, `go build ./src/...` OK, `go test ./src/messaging/...` 10/10 PASS. Aucun adapter n'importe l'infra (grep CLEAN). go.mod inchangé. Frontières CONFORMES. |
| 2026-06-13 | T-003 (1re passe) | PASS | go-engineer → qa → architect-reviewer, PASS du 1er coup (leçons T-001/T-002 sur le placement des interfaces appliquées d'emblée). QA : `go build ./src/...` OK, `go vet ./src/routing/...` OK, `go test ./src/routing/... -v` 35/35 PASS (le QA a ajouté `internal/adapters/http/handler_test.go`, 17 cas couvrant les 2 endpoints + mapping erreurs 400/422/500/204). go.mod inchangé, go.sum absent (stdlib-only), aucun import tiers actif. Isolation schéma `routing` stricte (3 tables, colonnes exactement conformes à 0004). Architect-reviewer : CONFORME — domaine pur (selector.go logique pure), application ne dépend que des ports, 0 violation adapters→infrastructure (grep vide), pas d'interface Broker (Routing ne publie pas), inversion via ports, aucun import cross-service. Écarts schéma/spec assumés et documentés (devise par config, Fastest/Custom en fallback faute de colonnes latence/config). |
| 2026-06-13 | T-002 (1re passe) | PASS | go-engineer → qa → architect-reviewer, PASS du 1er coup (leçon T-001 sur le Broker appliquée d'emblée). QA : `go build ./src/...` OK, `go vet ./src/wallet/...` OK, `go test ./src/wallet/... -v` 19/19 PASS, grep C3→C4 vide, go.mod/go.sum inchangés. Critères fonctionnels Debit/Credit/Refund/GetBalance + mapping HTTP (400/402/404) + Money + persistence (colonnes 0002) tous OK. Architect-reviewer : CONFORME (interface Broker en C3 `adapters/messaging`, NoopBroker en C4 avec assertion C4→C3, inversion via ports, isolation schéma `wallet`, stdlib-only). Écart non bloquant : tests use case credit/refund/get_balance et adapters absents. |

---

## Blocages / Risques
- **Réseau non garanti** : `npm install` (build TS), commandes Atlas/Docker peuvent échouer hors ligne → acceptance partielle à signaler.
- **`mk/react.mk` + `docker/react.dockerfile` manquants** : bloquent le build du dashboard (T-008/T-009).
- **Base unifiée vs 99.9 %** : point de couplage assumé (cf. MEMORY D11) ; scindable par schéma plus tard.

---

## Changelog
- **2026-06-13** — Création du tracker + de l'équipe d'agents. Baseline scaffold consignée. Backlog MVP T-001..T-010 défini.
- **2026-06-13** — T-001 **Done** (PASS). Adapters concrets messaging implémentés par go-engineer (C3 persistence/publisher/clients/http/consumer + C4 config/postgres/rabbitmq/httpserver + composition root). Cycle: go-engineer → qa (PASS build/vet/tests) → architect-reviewer (FAIL: 2 violations C3→C4) → go-engineer (fix interface Broker → C3) → architect-reviewer (CONFORME). Stdlib-only conservé (pas de go.sum), conforme à la contrainte offline du dépôt.
- **2026-06-13** — T-003 **Done** (PASS). Service Routing implémenté par go-engineer : domaine pur (Money, Channel, RoutingStrategy lowest_cost/highest_delivery/fastest/custom, ProviderPricing/Score, RoutingRule, RoutingDecision+ProviderRef, `SelectProvider` cœur métier), use cases get_routing_decision (rule absente → défaut highest_delivery) / update_provider_score (feedback DLR, Upsert ON CONFLICT (provider,channel)), adapters persistence (schéma `routing`, tables provider_pricing/provider_scores/routing_rules de 0004) + http (POST /route, POST /scores), config (port 8083, DefaultCurrency XAF) + postgres + httpserver, composition root sans broker. Cycle : go-engineer → qa (PASS, 35/35 tests, 17 tests handler ajoutés) → architect-reviewer (CONFORME du 1er coup). Stdlib-only, go.mod inchangé / pas de go.sum. **Écart documenté** : le schéma 0004 est plus simple que la spec domaine idéale (provider_scores sans country/delivery_rate/avg_latency ; routing_rules sans channel/config) → adapters alignés sur le schéma réel ; `Fastest` et `Custom` retombent sur le tri par score faute de colonnes latence/config (TODO en commentaire). Devise dérivée de la config car absente de provider_pricing. Service Wallet implémenté par go-engineer : domaine Money (value object centimes)/Wallet/WalletTransaction, use cases Debit/Credit/Refund/GetBalance, adapters persistence (schéma `wallet`, tables wallets + wallet_transactions ledger append-only)/publisher/http (GET /balance, POST /debit|credit|refund), interface Broker en C3, NoopBroker + infra en C4, composition root. Cycle: go-engineer → qa (PASS, 19/19 tests) → architect-reviewer (CONFORME du 1er coup). Stdlib-only, go.mod/go.sum inchangés. Dette de tests notée (use cases credit/refund/get_balance + adapters non couverts).
