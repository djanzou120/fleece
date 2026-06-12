# Fleece — Project Tracker (mémoire de l'agent PM)

> **Propriétaire : `fleece-pm`.** Source de vérité du suivi projet : tâches, assignations,
> tests d'acceptance, blocages. Mis à jour à chaque intervention/changement d'état.
> Voir aussi `.ia/MEMORY.md` (décisions techniques) et `.ia/user-story.md` (critères d'acceptance).
>
> Dernière mise à jour : **2026-06-13**.

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
| T-001 | Adapters concrets messaging (Postgres, RabbitMQ, HTTP, clients) | `src/messaging` | go-engineer | 🟢 | Backlog | — | Domaine + use case déjà faits ; manque infra/adapters. |
| T-002 | Implémenter Wallet (débit/refund/ledger) | `src/wallet` | go-engineer | 🟢 | Backlog | — | Schéma `wallet` prêt (0002). |
| T-003 | Implémenter Routing (stratégies + fallback) | `src/routing` | go-engineer | 🟢 | Backlog | — | Stratégies lowest_cost/highest_delivery/fastest/custom. |
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

---

## Blocages / Risques
- **Réseau non garanti** : `npm install` (build TS), commandes Atlas/Docker peuvent échouer hors ligne → acceptance partielle à signaler.
- **`mk/react.mk` + `docker/react.dockerfile` manquants** : bloquent le build du dashboard (T-008/T-009).
- **Base unifiée vs 99.9 %** : point de couplage assumé (cf. MEMORY D11) ; scindable par schéma plus tard.

---

## Changelog
- **2026-06-13** — Création du tracker + de l'équipe d'agents. Baseline scaffold consignée. Backlog MVP T-001..T-010 défini.
