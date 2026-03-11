# Inventario Helm Chart

This chart lives at `helm/inventario/` and deploys Inventario in two supported modes:

- **Production-style**: connect the app to external PostgreSQL, Redis, and object storage.
- **Demo mode**: optionally run in-cluster PostgreSQL, Redis, and MinIO for evaluation.

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
| `replicaCount` | `1` | Increase only with shared object storage; do not scale `file://` uploads. |
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

Validated in this workspace on 2026-03-11 with the following commands:

```bash
helm lint helm/inventario/ \
  --set-string secrets.dbDsn='postgres://user:pass@pg-host:5432/inventario?sslmode=require' \
  --set-string secrets.jwtSecret='testtesttesttesttesttesttesttest' \
  --set-string secrets.fileSigningKey='testtesttesttesttesttesttesttest' \
  --set setupJob.initData.adminPassword=test

helm template inventario helm/inventario/ \
  --set-string secrets.dbDsn='postgres://user:pass@pg-host:5432/inventario?sslmode=require' \
  --set-string secrets.jwtSecret='testtesttesttesttesttesttesttest' \
  --set-string secrets.fileSigningKey='testtesttesttesttesttesttesttest' \
  --set setupJob.initData.adminPassword=test

helm template inventario helm/inventario/ \
  --set-string secrets.existingSecret='inventario-runtime'

helm lint helm/inventario/ \
  --set demo.postgresql.enabled=true \
  --set demo.redis.enabled=true \
  --set demo.minio.enabled=true \
  --set setupJob.initData.adminPassword=demo-secret

helm template inventario helm/inventario/ \
  --set demo.postgresql.enabled=true \
  --set demo.redis.enabled=true \
  --set demo.minio.enabled=true \
  --set setupJob.initData.adminPassword=demo-secret

if helm template inventario helm/inventario/; then
  echo "expected missing-DSN render to fail" >&2
  exit 1
fi
```

Observed results:

- Production-style `helm lint` passed.
- Production-style render passed and rendered only the app resources (`ServiceAccount`, app `Secret`, `ConfigMap`, uploads `PVC`, `Service`, `Deployment`, setup `Job`).
- Existing-secret render passed and omitted `templates/secret.yaml` as expected.
- Demo `helm lint` passed.
- Demo render passed and rendered the app plus demo PostgreSQL, Redis, MinIO, and the MinIO bucket Job.
- Missing-DSN render failed fast with `secrets.dbDsn must be set when demo.postgresql.enabled=false and secrets.existingSecret is empty`.