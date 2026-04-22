# Inventario Helm Chart

This chart lives at `helm/inventario/` and deploys Inventario in two supported modes:

- **Production-style**: connect the app to external PostgreSQL, Redis, and object storage.
- **Demo mode**: optionally run in-cluster PostgreSQL, Redis, and MinIO for evaluation.

Each mode additionally supports two **run topologies**:

- **Combined** (default): one Deployment runs `inventario run all`, serving both the HTTP API and every background worker (`run.all.enabled=true`).
- **Split**: one Deployment per role (`run.apiserver` + zero or more `run.workers.<role>`) so each subsystem can be sized and scaled independently. See [Split deployment mode](#split-deployment-mode).

The chart source of truth is `helm/inventario/values.yaml` plus the templates in `helm/inventario/templates/`. `values.yaml` contains the complete default surface with inline comments; this README focuses on the install paths and the values you normally need to change.

## Prerequisites

- Helm 3.x
- Kubernetes API reachable from `kubectl`
- A namespace to install into (or use `--create-namespace`)

## Production-style install

Production-style installs are expected to use external services and real secrets.

Minimum required production inputs:

- `secrets.dbDsn` **or** `secrets.existingSecret`
- `secrets.jwtSecret` **or** `secrets.existingSecret`
- `secrets.fileSigningKey` **or** `secrets.existingSecret`
- `setupJob.initData.adminPassword` on first install **or** `SETUP_ADMIN_PASSWORD` in `secrets.existingSecret`
- `setupJob.bootstrap.superuserDsn` if the setup hook must create/update DB roles for you

Example:

```bash
helm upgrade --install inventario ./helm/inventario \
  --namespace inventario --create-namespace \
  --set-string secrets.dbDsn='postgres://inventario:password@postgres.example:5432/inventario?sslmode=require' \
  --set-string secrets.jwtSecret='replace-with-at-least-32-characters' \
  --set-string secrets.fileSigningKey='replace-with-at-least-32-characters' \
  --set-string setupJob.initData.adminPassword='replace-me' \
  --set-string app.publicUrl='https://inventario.example.com'
```

Production notes:

- Keep `replicaCount: 1` when using `file://` uploads. For multiple replicas, switch `app.uploadLocation` to shared object storage such as S3/MinIO/Azure/GCS.
- If your database users are managed outside the chart, set `setupJob.bootstrap.enabled=false`.
- The setup Job runs as a Helm hook. It is `pre-install,pre-upgrade` by default, and switches to `post-install,pre-upgrade` when `demo.postgresql.enabled=true` so the demo database exists before bootstrap/migrate runs.

## Demo install

Demo mode enables in-cluster PostgreSQL, Redis, and MinIO and automatically wires the app to them.

Example:

```bash
helm upgrade --install inventario ./helm/inventario \
  --namespace inventario-demo --create-namespace \
  --set demo.postgresql.enabled=true \
  --set demo.redis.enabled=true \
  --set demo.minio.enabled=true \
  --set-string secrets.jwtSecret='replace-with-at-least-32-characters' \
  --set-string secrets.fileSigningKey='replace-with-at-least-32-characters' \
  --set-string setupJob.initData.adminPassword='demo-admin-password'
```

Demo notes:

- Demo PostgreSQL, Redis, and MinIO are **not production-ready**.
- Demo persistence is disabled by default; data is lost on pod restart unless you enable the demo PVCs.
- Demo MinIO overrides `app.uploadLocation` to an in-cluster S3-compatible endpoint.
- Demo Redis wires all Redis-backed features to a single demo Redis instance.
- `setupJob.initData.seedDatabase=true` is available for a seeded demo install and is skipped automatically on upgrades to avoid duplicate demo data.

## Split deployment mode

Split mode renders one Deployment per role so the API server and each background worker can scale, schedule, and auto-scale independently. Enable it by turning off the combined Deployment and turning on the roles you need:

```yaml
run:
  all:
    enabled: false
  apiserver:
    enabled: true
  workers:
    thumbnails:
      enabled: true
    exports:
      enabled: true
    imports:
      enabled: true
    restores:
      enabled: true
    emails:
      enabled: true
    tokenCleanup:
      enabled: true
```

Each enabled worker role renders:

- a `Deployment` named `<release>-<chart>-worker-<cli-id>` running `run workers --workers-only=<cli-id> --probe-addr=:<probePort>`;
- a headless `Service` on the probe port so Prometheus (or a `ServiceMonitor`) can discover the pods and scrape `/metrics`.

`run.all.enabled` and any split role are **mutually exclusive**. The chart fails the render with a clear error if both are enabled, or if neither is.

Per-role values deep-merge over `run.workers.common`. Typical overrides are `replicaCount`, `resources`, `nodeSelector`, `tolerations`, and `autoscaling`.

Split mode notes:

- **Shared uploads**: when `app.uploadLocation` stays on `file://` and `persistence.enabled=true`, every role pod mounts the same uploads PVC. This requires an `accessMode` that allows multi-pod use (for example `ReadWriteMany`). For single-node clusters, ReadWriteOnce works only if every pod is scheduled on the same node. Object storage (`s3://`, `azblob://`, `gs://`) avoids the multi-pod PVC constraint entirely.
- **Restores scaling**: each `run.workers.restores` replica enforces its own `app.maxConcurrentRestores` limit, so cluster-wide concurrency equals `replicaCount × maxConcurrentRestores`. Database-level job locking prevents duplicate work, but keep `replicaCount=1` unless your DB can sustain the multiplied load.
- **CLI identifiers**: the Helm key `tokenCleanup` maps to the CLI flag value `token-cleanup`. All other role keys match the CLI flag value as-is.
- **Autoscaling**: set `<role>.autoscaling.enabled=true` to render a `HorizontalPodAutoscaler` targeting the matching Deployment. CPU utilization is the default metric; extend via `<role>.autoscaling.metrics` and `<role>.autoscaling.behavior`.

Example split install using external services:

```bash
helm upgrade --install inventario ./helm/inventario \
  --namespace inventario --create-namespace \
  --set-string secrets.dbDsn='postgres://inventario:password@postgres.example:5432/inventario?sslmode=require' \
  --set-string secrets.jwtSecret='replace-with-at-least-32-characters' \
  --set-string secrets.fileSigningKey='replace-with-at-least-32-characters' \
  --set-string setupJob.initData.adminPassword='replace-me' \
  --set run.all.enabled=false \
  --set run.apiserver.enabled=true \
  --set run.workers.thumbnails.enabled=true \
  --set run.workers.exports.enabled=true \
  --set run.workers.imports.enabled=true \
  --set run.workers.restores.enabled=true \
  --set run.workers.emails.enabled=true \
  --set run.workers.tokenCleanup.enabled=true
```

## Secret handling

By default the chart renders its own Secret (`templates/secret.yaml`) from `values.yaml` and `--set-string` values.

If you set `secrets.existingSecret`, the chart **does not render its own Secret** and instead points the Deployment and setup Job at your existing Secret name.

Expected secret keys when using `secrets.existingSecret`:

- `INVENTARIO_DB_DSN` (required unless `demo.postgresql.enabled=true`)
- `MIGRATOR_DB_DSN` (optional; falls back to `INVENTARIO_DB_DSN`)
- `INVENTARIO_RUN_JWT_SECRET` (required)
- `INVENTARIO_RUN_FILE_SIGNING_KEY` (required)
- `SETUP_ADMIN_PASSWORD` (required when `setupJob.enabled=true`)
- `SETUP_SUPERUSER_DSN` (optional)
- `INVENTARIO_RUN_TOKEN_BLACKLIST_REDIS_URL`, `INVENTARIO_RUN_AUTH_RATE_LIMIT_REDIS_URL`, `INVENTARIO_RUN_GLOBAL_RATE_LIMIT_REDIS_URL`, `INVENTARIO_RUN_CSRF_REDIS_URL`, `INVENTARIO_RUN_EMAIL_QUEUE_REDIS_URL` (optional)
- `INVENTARIO_RUN_SMTP_PASSWORD`, `INVENTARIO_RUN_SENDGRID_API_KEY`, `INVENTARIO_RUN_MANDRILL_API_KEY` (provider-specific)
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` (required for `s3://` upload targets)

Example existing Secret install:

```bash
kubectl create secret generic inventario-runtime \
  --from-literal=INVENTARIO_DB_DSN='postgres://inventario:password@postgres.example:5432/inventario?sslmode=require' \
  --from-literal=INVENTARIO_RUN_JWT_SECRET='replace-with-at-least-32-characters' \
  --from-literal=INVENTARIO_RUN_FILE_SIGNING_KEY='replace-with-at-least-32-characters' \
  --from-literal=SETUP_ADMIN_PASSWORD='replace-me'

helm upgrade --install inventario ./helm/inventario \
  --namespace inventario --create-namespace \
  --set-string secrets.existingSecret='inventario-runtime'
```

## Important values reference

For the complete default surface, see `helm/inventario/values.yaml`.

| Value | Default | When to change it |
| --- | --- | --- |
| `image.repository` | `ghcr.io/denisvmedia/inventario` | Use your own registry/image mirror. |
| `image.tag` | `""` | Pin a specific app version instead of `Chart.appVersion`. |
| `run.all.enabled` | `true` | Disable to switch from the combined Deployment to split mode. Mutually exclusive with `run.apiserver`/`run.workers.*`. |
| `run.all.replicaCount` | `1` | Increase only with shared object storage; do not scale `file://` uploads. |
| `run.all.autoscaling.enabled` | `false` | Enable to emit an HPA for the combined Deployment. |
| `run.apiserver.enabled` | `false` | Enable to render the standalone API-server Deployment in split mode. |
| `run.apiserver.replicaCount` | `1` | Scale the API tier independently of workers. |
| `run.apiserver.autoscaling.enabled` | `false` | Enable to emit an HPA for the API-server Deployment. |
| `run.workers.common.*` | see `values.yaml` | Shared defaults deep-merged into each per-role worker block. |
| `run.workers.<role>.enabled` | `false` | Turn on a worker Deployment for the given role (`thumbnails`, `exports`, `imports`, `restores`, `emails`, `tokenCleanup`). |
| `run.workers.<role>.replicaCount` | `1` | Scale the worker pool for the given role. Review caveats for `restores`. |
| `run.workers.<role>.autoscaling.enabled` | `false` | Enable to emit an HPA for the given worker role. |
| `service.type` | `ClusterIP` | Change for NodePort/LoadBalancer exposure. |
| `ingress.enabled` | `false` | Enable public ingress routing. |
| `ingress.hosts` / `ingress.tls` | example host / empty | Set your real hostname and TLS secret(s). |
| `app.publicUrl` | `""` | Set the public base URL for email links and external access. |
| `app.uploadLocation` | `file:///app/uploads?create_dir=1` | Use shared object storage for HA or external blob storage. |
| `persistence.enabled` | `true` | Disable only when you intentionally want ephemeral local uploads. |
| `persistence.size` | `10Gi` | Resize the uploads PVC for local file storage. |
| `email.provider` | `stub` | Switch to `smtp`, `sendgrid`, `ses`, `mandrill`, or `mailchimp` for real email. |
| `email.from` | `""` | Required for real email delivery. |
| `secrets.existingSecret` | `""` | Use an externally managed Secret instead of chart-managed secret data. |
| `secrets.dbDsn` | `""` | Required for production-style installs unless `secrets.existingSecret` is used. |
| `secrets.migratorDbDsn` | `""` | Set when schema migrations need a different DB user than the app runtime. |
| `secrets.jwtSecret` | `""` | Required unless supplied through `secrets.existingSecret`. |
| `secrets.fileSigningKey` | `""` | Required unless supplied through `secrets.existingSecret`. |
| `setupJob.bootstrap.enabled` | `true` | Disable if DB bootstrap/role management is handled outside Helm. |
| `setupJob.bootstrap.superuserDsn` | `""` | Provide when bootstrap needs elevated DB privileges. |
| `setupJob.initData.adminEmail` | `admin@example.com` | Set the initial admin login name. |
| `setupJob.initData.adminPassword` | `""` | Required on first install unless provided by `secrets.existingSecret`. |
| `setupJob.initData.seedDatabase` | `false` | Enable only for demo/sample data seeding. |
| `demo.postgresql.enabled` | `false` | Turn on in-cluster demo PostgreSQL. |
| `demo.redis.enabled` | `false` | Turn on in-cluster demo Redis. |
| `demo.minio.enabled` | `false` | Turn on in-cluster demo MinIO and S3-style uploads. |

## Upgrade notes

- Use `helm upgrade --install` for both first install and upgrades.
- The setup hook re-runs on upgrades to keep bootstrap/migrations safe and idempotent.
- Demo bucket creation is a `post-install,post-upgrade` hook.
- Demo database seeding is skipped on upgrades even if `setupJob.initData.seedDatabase=true`.

## Validation

The chart is covered by the `helm-lint.yml` CI workflow across combined, split, demo, and misconfiguration scenarios. The commands below reproduce those scenarios locally:

```bash
# Combined (run.all) render with explicit secrets.
helm template inventario helm/inventario/ \
  --set-string secrets.dbDsn='postgres://user:pass@pg-host:5432/inventario?sslmode=require' \
  --set-string secrets.jwtSecret='testtesttesttesttesttesttesttest' \
  --set-string secrets.fileSigningKey='testtesttesttesttesttesttesttest' \
  --set setupJob.initData.adminPassword=test

# Combined render with an existing Secret.
helm template inventario helm/inventario/ \
  --set-string secrets.existingSecret='inventario-runtime'

# Demo render.
helm template inventario helm/inventario/ \
  --set demo.postgresql.enabled=true \
  --set demo.redis.enabled=true \
  --set demo.minio.enabled=true \
  --set setupJob.initData.adminPassword=demo-secret

# Split mode render (apiserver + all workers).
helm template inventario helm/inventario/ \
  --set-string secrets.dbDsn='postgres://user:pass@pg-host:5432/inventario?sslmode=require' \
  --set-string secrets.jwtSecret='testtesttesttesttesttesttesttest' \
  --set-string secrets.fileSigningKey='testtesttesttesttesttesttesttest' \
  --set setupJob.initData.adminPassword=test \
  --set run.all.enabled=false \
  --set run.apiserver.enabled=true \
  --set run.workers.thumbnails.enabled=true \
  --set run.workers.exports.enabled=true \
  --set run.workers.imports.enabled=true \
  --set run.workers.restores.enabled=true \
  --set run.workers.emails.enabled=true \
  --set run.workers.tokenCleanup.enabled=true

# Mutual exclusion guard (expected to fail).
if helm template inventario helm/inventario/ \
  --set-string secrets.dbDsn='postgres://u:p@h/d' \
  --set-string secrets.jwtSecret='testtesttesttesttesttesttesttest' \
  --set-string secrets.fileSigningKey='testtesttesttesttesttesttesttest' \
  --set setupJob.initData.adminPassword=test \
  --set run.apiserver.enabled=true; then
  echo "expected combined+split render to fail" >&2
  exit 1
fi

# Missing-DSN guard (expected to fail).
if helm template inventario helm/inventario/; then
  echo "expected missing-DSN render to fail" >&2
  exit 1
fi
```