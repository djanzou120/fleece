# Fleece — Kubernetes manifests

Namespace: `fleece`
All manifests use `namespace: fleece` and must be applied after the namespace is created.

## Directory layout

```
deploy/k8s/
├── namespace.yaml           # Namespace fleece
├── config/
│   ├── configmap.yaml       # Non-sensitive configuration (URLs, ports, etc.)
│   └── secret.example.yaml  # Secret template — copy to secret.yaml, fill values, do NOT commit
├── infra/
│   ├── postgres.yaml        # PostgreSQL 16 Deployment + ClusterIP Service
│   └── rabbitmq.yaml        # RabbitMQ 3-management Deployment + ClusterIP Service
├── jobs/
│   └── atlas-migrate.yaml   # Atlas init Job (runs migrations before services start)
└── services/
    ├── api.yaml                     # Go — port 8080 (unified HTTP service, replaces messaging/wallet/routing/provider/webhook — M-024)
    ├── core-processor.yaml          # Go — worker, no HTTP port (AMQP consumer only)
    ├── intelligence-processor.yaml  # Go — worker, no HTTP port (AMQP consumer + 2 tickers, replicas fixed at 1 — see manifest)
    ├── auth-api.yaml                # TypeScript — port 3001  (ClusterIP)
    ├── graphql-api.yaml             # TypeScript — port 4000  (LoadBalancer)
    └── platform-app.yaml            # Next.js   — port 3000  (LoadBalancer)
```

> **M-024 (2026-07-27):** the 8 Go services (messaging, wallet, routing,
> provider, webhook, contact-intelligence, campaign, analytics) previously
> deployed as 5 Deployments (only 5 ever had a manifest — see
> `.ia/MIGRATION_PLAN.md` T-024) are replaced by 3 binaries: `api` (unified
> HTTP service), `core-processor` and `intelligence-processor` (workers, no
> HTTP port). See each manifest's header comment for env vars, probes, and
> the replicas/HPA rationale (the two workers are NOT scaled the same way —
> `core-processor` is fixed-replica pending a queue-depth autoscaler,
> `intelligence-processor` is pinned to 1 replica because of its two
> in-process tickers).

## Prerequisites

- A Kubernetes cluster (1.28+) with `kubectl` configured.
- Docker images built and available to the cluster (push to a registry or use `imagePullPolicy: IfNotPresent` with local images loaded via `kind load` / `minikube image load`).
- `make` installed to build images: `make image pkg=<svc>`.

## Apply order

### 1. Namespace

```sh
kubectl apply -f deploy/k8s/namespace.yaml
```

### 2. Secrets (fill in real values first)

```sh
cp deploy/k8s/config/secret.example.yaml deploy/k8s/config/secret.yaml
# Edit secret.yaml — replace all CHANGE_ME placeholders
kubectl apply -f deploy/k8s/config/secret.yaml -n fleece
```

### 3. ConfigMap

```sh
kubectl apply -f deploy/k8s/config/configmap.yaml
```

### 4. Infrastructure (Postgres + RabbitMQ)

```sh
kubectl apply -f deploy/k8s/infra/postgres.yaml
kubectl apply -f deploy/k8s/infra/rabbitmq.yaml
# Wait for postgres to be ready
kubectl rollout status deployment/postgres -n fleece
```

### 5. ConfigMaps for Atlas migrations

The migration files and `atlas.hcl` are not embedded in the bastion image.
Use the dedicated Makefile target to create or update both ConfigMaps from
the repository files before running the Job (idempotent — safe to re-run
after adding new migration files):

```sh
make k8s-configmaps
```

This is equivalent to (and replaces) the former manual commands:

```sh
# Equivalent — now automated by make k8s-configmaps
kubectl create configmap atlas-migrations \
  --from-file=migrations/ \
  -n fleece \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create configmap atlas-hcl \
  --from-file=atlas.hcl=atlas.hcl \
  -n fleece \
  --dry-run=client -o yaml | kubectl apply -f -
```

### 6. Atlas migration Job

```sh
kubectl apply -f deploy/k8s/jobs/atlas-migrate.yaml
# Wait for job completion
kubectl wait --for=condition=complete job/atlas-migrate -n fleece --timeout=120s
```

### 7. Application services

```sh
kubectl apply -f deploy/k8s/services/
```

## Image naming convention

| Service                | Image tag                             | Dockerfile               |
|------------------------|----------------------------------------|--------------------------|
| api                    | `fleece/api:latest`                    | `docker/go.dockerfile`   |
| core-processor         | `fleece/core-processor:latest`         | `docker/go.dockerfile`   |
| intelligence-processor | `fleece/intelligence-processor:latest` | `docker/go.dockerfile`   |
| auth-api               | `fleece/auth-api:latest`               | `docker/node.dockerfile` |
| graphql-api            | `fleece/graphql-api:latest`            | `docker/node.dockerfile` |
| platform-app           | `fleece/platform-app:latest`           | `docker/react.dockerfile`|
| bastion                | `fleece/bastion:latest`                | `src/bastion/Dockerfile` |

Build all images:

```sh
for svc in api core-processor intelligence-processor; do
  make image pkg=$svc
done
make image pkg=auth-api
make image pkg=graphql-api
make image pkg=platform-app
make image pkg=bastion
```

## Notes

- **Storage:** PostgreSQL and RabbitMQ use `emptyDir` volumes (data lost on pod restart). For production, replace with PersistentVolumeClaims (example commented in `infra/postgres.yaml`).
- **NEXT_PUBLIC_GRAPHQL_URL:** This is a browser-side variable baked into the Next.js bundle at build time. For production, rebuild `platform-app` with the real external hostname of `graphql-api`. The ConfigMap value (`http://graphql-api:4000/graphql`) works for SSR only.
- **TLS / HTTPS:** Not configured at the manifest level. Add a TLS-terminating Ingress controller (e.g., ingress-nginx + cert-manager) in front of `graphql-api` and `platform-app` LoadBalancer Services.
- **HPA:** All application services include a HorizontalPodAutoscaler. The Metrics Server must be installed in the cluster (`kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml`).
- **Secrets:** `secret.yaml` (filled) must never be committed to version control. Only `secret.example.yaml` is committed.
- **AMQP topology bootstrap order (D-M35):** `api.yaml`'s `wait-for-amqp-topology` initContainer blocks `api`'s pods from starting until BOTH the `core-processor` and `intelligence-processor` queues exist on the broker — i.e. until `core-processor.yaml` and `intelligence-processor.yaml` have been applied and their pods have run their composition root at least once. Apply (or at least roll out) the two workers before or together with `api`, never `api` alone on a fresh cluster — otherwise its pods stay `Running`/`NotReady` indefinitely on that initContainer (never crash-looping, just pending) until a worker pod comes up. This is a deployment-level mitigation for a real gap in the Go code (`src/api` only declares the exchange, never the queues/bindings it depends on — see the manifest's own comment and `.ia/MIGRATION_PLAN.md` D-M35); it does not replace the recommended code fix (`mandatory=true` + `NotifyReturn` on `Conn.Publish`, or a dedicated topology-bootstrap Job).
