# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Project overview

**Fleece** is an omnichannel communication platform (API-first): a single API that routes messages over the best channel (SMS, WhatsApp, Telegram, …) at the best cost, with intelligent routing + fallback and a prepaid wallet. Target markets: francophone Africa + Europe.

Design documents live in `.ia/` (PRD, TDD, ARCHITECTURE, user-story, MEMORY). Read `.ia/MEMORY.md` first when picking up a session — it summarises every technical decision and its rationale.

---

## Build system

The monorepo uses a custom `Makefile` + `mk/<type>.mk` system. Every package declares its type in a `src/<pkg>/pkg` descriptor file (`type=go|node|react|docker|graphql`). The Makefile reads that file and includes the matching `mk/<type>.mk`.

```sh
make build pkg=<name>          # compile / bundle
make test  pkg=<name>          # run tests
make image pkg=<name>          # build Docker image (ARG PKG)
make fmt   pkg=<name>          # format code
```

`pkg` is always required. `version` is derived from a `<pkg>-*` git tag, falling back to the short commit SHA.

### Go services

```sh
go build ./src/<svc>           # direct Go build (all services share module "fleece")
go test  ./src/<svc>/...       # run all tests for a service
go test  ./src/<svc>/internal/domain/...  # run tests for one layer
go vet   ./src/...             # vet the whole monorepo
```

Go module is **single** (`module fleece`, `go.mod` at repo root). Internal imports follow `fleece/src/<svc>/internal/...`.

### TypeScript services (`src/auth-api`, `src/graphql-api`, `src/ts/*`)

```sh
npm install                    # install all workspaces (run from repo root)
npm exec -- tsc -p ./src/<pkg>/tsconfig.json --noEmit   # type-check
make build pkg=auth-api        # type-check + esbuild bundle → bin/auth-api/index.js
make test  pkg=auth-api        # jest
make fmt   pkg=auth-api        # prettier --write
```

Workspaces are declared in the root `package.json` (`src/ts/*`, `src/auth-api`, `src/graphql-api`).

### Database migrations (Atlas)

```sh
atlas migrate hash --dir file://migrations                                   # update checksum after adding a file
atlas migrate lint --dir file://migrations --dev-url "docker://postgres/16/dev" --latest 1
atlas migrate apply --dir file://migrations --url "$DATABASE_URL"            # applied as K8s init job
```

`DATABASE_URL` env var must be set for apply. Atlas config is in `atlas.hcl`.

---

## Repository layout

```
src/<pkg>/         one folder = one deployable or lib; type declared in src/<pkg>/pkg
migrations/        ALL SQL migrations (single folder, Atlas)
mk/                build rules per type: go.mk  node.mk  esbuild.mk  graphql.mk  docker.mk
docker/            go.dockerfile  node.dockerfile  (ARG PKG — one Dockerfile per language)
src/bastion/       tooling container (psql, atlas); its own Dockerfile
src/go/app/        shared Go lib: Version/Name (injected at build) + Bootstrap()
src/ts/            shared TS libs: logger, config, form, gql, mail (npm workspaces)
src/graphql/       GraphQL schema + codegen
.ia/               all design docs (PRD, TDD, ARCHITECTURE, MEMORY, user-story, PROJECT_TRACKER)
```

**Do not** introduce `services/`, `libs/`, or `apps/` directories — the existing `src/<pkg>` convention is intentional.

---

## Architecture: Clean Architecture, strictly enforced

All services follow a 4-layer model. **Dependencies point inward only.**

```
4. infrastructure/   ← Postgres, RabbitMQ, Redis, HTTP server, Better Auth, config, composition root
3. adapters/         ← HTTP handlers, GraphQL resolvers, Postgres repositories, REST clients, RabbitMQ publishers
2. application/      ← use cases + ports/input + ports/output (interfaces only, no frameworks)
1. domain/           ← pure entities, value objects, state machines, business errors (zero external imports)
```

**Go services** (`src/messaging`, `src/routing`, `src/provider`, `src/wallet`, `src/webhook`, `src/campaign`, `src/contact-intelligence`, `src/analytics`):
- Internal packages live under `src/<svc>/internal/` (Go's `internal/` prevents cross-service imports).
- Composition root is `src/<svc>/main.go` — manual DI only.
- Reference implementation: `src/messaging/` (entity + state machine + ports + use case all filled in).

**TypeScript services**:
- `src/auth-api` — Identity Service (Better Auth confined to `adapters/auth/`).
- `src/graphql-api` — GraphQL Gateway / BFF (no business rules; Application + Adapters only).
- Layers sit directly under the package folder (no nested `src/`); entrypoint is `index.ts`.

**Key rules:**
- `domain/` and `application/` must not import from `adapters/` or `infrastructure/`. Enforced by **depguard** (Go) and **dependency-cruiser** (TS) in CI.
- No service imports another service's domain. Cross-service calls go through `adapters/clients/` (REST) or RabbitMQ events.
- Better Auth, Drizzle, RabbitMQ, Postgres, GraphQL are layer-3/4 details — never leak into domain or application.

---

## Database

Single PostgreSQL instance; **one schema per service** (`identity`, `wallet`, `messaging`, `routing`, `provider`, `webhook`, `campaign`, `contact_intel`, `analytics`). Each service connects with `search_path` set to its own schema. No cross-schema queries.

**Atlas is the single source of truth for migrations.** Drizzle is used as a query builder only — never run Drizzle migrations. All migrations live in `migrations/` (numbered globally: `0001_identity.sql` … `0009_analytics.sql`). One file = one schema. Never mix two schemas in one migration file.

---

## Inter-service communication

- **Sync:** REST (internal HTTP) — each service exposes an internal REST API.
- **Async:** RabbitMQ — events (`message.delivered`, `wallet.refunded`, …) are output ports in `application/ports/output/`; RabbitMQ is the adapter in layer 3/4.

---

## Shell note

The user's shell is **fish**. For scripts with loops, heredocs, or word-splitting, use `bash -c '…'` or a temporary bash file — `for x in $var` does not word-split in fish.

---

## Agent team

Specialised agents are defined in `.ia/.claude/agents/`. For feature work, always go through `fleece-pm` — it owns `.ia/PROJECT_TRACKER.md` and dispatches to the appropriate engineer agents.

| Agent | Scope |
|-------|-------|
| `fleece-pm` | Orchestrator; owns PROJECT_TRACKER |
| `fleece-go-engineer` | Go services (messaging, routing, provider, wallet, webhook, …) |
| `fleece-ts-engineer` | auth-api + graphql-api |
| `fleece-frontend-engineer` | src/platform-app (Next.js + shadcn) |
| `fleece-db-engineer` | migrations/ + PostgreSQL schema |
| `fleece-devops-engineer` | Makefile/mk/, docker/, deploy/k8s/, CI |
| `fleece-qa-engineer` | Tests and acceptance criteria |
| `fleece-architect-reviewer` | Clean Architecture guard (read-only) |
