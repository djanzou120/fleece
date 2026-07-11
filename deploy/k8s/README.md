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
    ├── messaging.yaml        # Go — port 8081
    ├── wallet.yaml           # Go — port 8082
    ├── routing.yaml          # Go — port 8083
    ├── provider.yaml         # Go — port 8084
    ├── webhook.yaml          # Go — port 8085
    ├── auth-api.yaml         # TypeScript — port 3001  (ClusterIP)
    ├── graphql-api.yaml      # TypeScript — port 4000  (LoadBalancer)
    └── platform-app.yaml     # Next.js   — port 3000  (LoadBalancer)
```

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

| Service       | Image tag                    | Dockerfile               |
|---------------|------------------------------|--------------------------|
| messaging     | `fleece/messaging:latest`    | `docker/go.dockerfile`   |
| wallet        | `fleece/wallet:latest`       | `docker/go.dockerfile`   |
| routing       | `fleece/routing:latest`      | `docker/go.dockerfile`   |
| provider      | `fleece/provider:latest`     | `docker/go.dockerfile`   |
| webhook       | `fleece/webhook:latest`      | `docker/go.dockerfile`   |
| auth-api      | `fleece/auth-api:latest`     | `docker/node.dockerfile` |
| graphql-api   | `fleece/graphql-api:latest`  | `docker/node.dockerfile` |
| platform-app  | `fleece/platform-app:latest` | `docker/react.dockerfile`|
| bastion       | `fleece/bastion:latest`      | `src/bastion/Dockerfile` |

Build all images:

```sh
for svc in messaging wallet routing provider webhook; do
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
