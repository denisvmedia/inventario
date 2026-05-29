# Inventario Helm Chart

This chart lives at `helm/inventario/` and deploys Inventario in two supported modes:

- **Production-style**: connect the app to external PostgreSQL, Redis, and object storage.
- **Demo mode**: optionally run in-cluster PostgreSQL, Redis, and MinIO for evaluation.

Each mode additionally supports two **run topologies**:

- **Combined** (default): one Deployment runs `inventario run all`, serving both the HTTP API and every background worker (`run.all.enabled=true`).
- **Split**: one Deployment per worker group (`run.apiserver` + zero or more `run.workers.<group>`) so each subsystem can be sized and scaled independently. See [Split deployment mode](#split-deployment-mode).

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

Split mode renders one Deployment per worker group so the API server and each group can scale, schedule, and auto-scale independently. Worker groups consolidate individual families that share an operational profile:

| Group | Members | Scales on |
| --- | --- | --- |
| `archive` | exports, imports, restores | archive-queue depth |
| `emails` | email delivery lifecycle | SMTP / provider rate |
| `housekeeping` | refresh-token GC (and future periodic cleanup loops) | n/a — periodic |
| `media` | thumbnails (future resize / OCR) | media-queue depth / CPU |

Enable split mode by turning off the combined Deployment and turning on the groups you need:

```yaml
run:
  all:
    enabled: false
  apiserver:
    enabled: true
  workers:
    archive:
      enabled: true
    emails:
      enabled: true
    housekeeping:
      enabled: true
    media:
      enabled: true
```

Each enabled worker group renders:

- a `Deployment` named `<release>-<chart>-worker-<group>` running `run workers --workers-only=<group> --probe-addr=:<probePort>`;
- a headless `Service` on the probe port so Prometheus (or a `ServiceMonitor`) can discover the pods and scrape `/metrics`.

`run.all.enabled` and any split role are **mutually exclusive**. The chart fails the render with a clear error if both are enabled, or if neither is.

Per-group values deep-merge over `run.workers.common`. Typical overrides are `replicaCount`, `resources`, `nodeSelector`, `tolerations`, and `autoscaling`.

Split mode notes:

- **Shared uploads**: when `app.uploadLocation` stays on `file://` and `persistence.enabled=true`, every role pod mounts the same uploads PVC. This requires an `accessMode` that allows multi-pod use (for example `ReadWriteMany`). For single-node clusters, ReadWriteOnce works only if every pod is scheduled on the same node. Object storage (`s3://`, `azblob://`, `gs://`) avoids the multi-pod PVC constraint entirely.
- **Archive scaling**: each `run.workers.archive` replica enforces its own `app.maxConcurrentRestores` (and export/import) limits, so cluster-wide concurrency equals `replicaCount × maxConcurrent` per family. Database-level job locking prevents duplicate work, but keep `replicaCount=1` unless your DB can sustain the multiplied load.
- **CLI identifiers**: group keys match the `--workers-only` flag value exactly. Legacy per-family names (`exports`, `imports`, `restores`, `thumbnails`, `token-cleanup`) remain accepted on the CLI for one release with a deprecation warning; the Helm values surface only the group keys.
- **Autoscaling**: set `<group>.autoscaling.enabled=true` to render a `HorizontalPodAutoscaler` targeting the matching Deployment. CPU utilization is the default metric; extend via `<group>.autoscaling.metrics` and `<group>.autoscaling.behavior`.

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
  --set run.workers.archive.enabled=true \
  --set run.workers.emails.enabled=true \
  --set run.workers.housekeeping.enabled=true \
  --set run.workers.media.enabled=true
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

## ArgoCD-managed migrations

When the chart is deployed via ArgoCD (rather than `helm install` directly), Helm hooks don't map cleanly to ArgoCD's PreSync / Sync / PostSync phases — a `pre-install` hook fires before the Postgres demo Deployment exists, and a `post-install` hook fires only after the main app Deployment is Healthy (which can't happen if the schema isn't migrated yet). Setting `setupJob.argocdMode=true` switches the chart to an ArgoCD-native layout that supports in-place upgrades across new migrations.

### Sync-wave layout

| Wave | Resource | Step |
| --- | --- | --- |
| `-5` | `<release>-setup` Job | Bootstrap only — creates the `inventario_migrator` DB role. Carries `argocd.argoproj.io/sync-options: Force=true,Replace=true` so each sync delete-then-creates the immutable Job pod. |
| `0` (default) | App `Deployment` (combined and/or apiserver + workers) | Adds a `migrate` init container that runs `inventario db migrate up` against `MIGRATOR_DB_DSN`. Migrations are pinned to the same image revision that runs the new pod. |
| `+5` | `<release>-init-data` Job | Runs `inventario db migrate data` (idempotent default-tenant + admin) and, when `setupJob.initData.seedDatabase=true`, seeds demo data via a temporary in-Job `inventario run` server. |

This is approach **A** from #1884: schema upgrades are coupled to the Deployment revision, so an in-place ArgoCD sync never leaves old-image pods talking to a newer schema for longer than the rolling-update window.

### What this fixes

- **No standalone migration window.** In the previous argocdMode (single Job at wave 0 alongside everything else), a re-sync with new migrations would re-create the Job with the new image and apply the migration while old-image app pods were still serving. Now the migration runs as part of starting a new-image pod; the surge replica brings up the new schema, then the rolling-update terminates one old pod at a time.
- **No `Replace=true` label collisions for the migrate step.** ArgoCD's `Replace=true` performs a `kubectl replace` on the Job, which delete-then-creates the resource. Moving migrations off the Job removes the label-collision flake observed during local testing — the bootstrap Job that remains at wave -5 only ever runs short, idempotent role-creation.
- **Cleaner failure mode.** A bad migration now fails the new pod's init container, the surge replica never becomes Ready, the Deployment rollout stalls at `progressDeadlineSeconds`, and the old pods keep serving. ArgoCD reports `Degraded`; the operator can roll the image tag back without manual schema intervention.

### Caveats

- **Migrations must be backward-compatible.** During the rolling update there is a window where old-image pods serve traffic against the newly migrated schema. This is the same expand-contract rule every rolling-update operator follows; the chart cannot enforce it. Plan multi-step renames / column drops over multiple releases.
- **Idempotent per-pod cost.** Every new app pod re-runs `inventario db migrate up`. The ptah migrator returns immediately when `schema_migrations.version` already equals the embedded max, so the steady-state cost is a single round-trip to Postgres per pod start.
- **External Postgres still requires bootstrap.** With `demo.postgresql.enabled=false`, the bootstrap Job at wave -5 connects to the external Postgres using `setupJob.bootstrap.superuserDsn` (or the `SETUP_SUPERUSER_DSN` key in `secrets.existingSecret`). If your platform creates DB roles out-of-band, set `setupJob.bootstrap.enabled=false` — the chart will skip the bootstrap container; the operator is responsible for ensuring `inventario_migrator` exists before the Deployment starts.

### Enabling

```yaml
setupJob:
  argocdMode: true
  initData:
    adminPassword: "<set-via-overlay-or-existingSecret>"
```

The shared preview overlay `infra/helm-overlays/preview-base.values.yaml` already sets `setupJob.argocdMode=true` for both the per-PR preview Applications and the static master Application.

### When to leave it off

For direct `helm install` / `helm upgrade --install` workflows, keep `setupJob.argocdMode=false` (the default). The existing setup Job runs as a Helm hook in the usual way — bootstrap → migrate → init-data sequentially in a single pod — which is the simpler story when Helm is the deployment engine.

## Namespace quotas (opt-in)

Set `quota.enabled=true` to render a namespace-scoped `ResourceQuota` plus an opt-out `LimitRange` (#1866). The pair is off by default because direct `helm install` against a production namespace usually shouldn't drop chart-managed quotas in silently; turn it on for installs that share a namespace boundary with other tenants — the canonical case is the per-PR preview namespaces driven by `infra/argocd/applicationset-pr.yaml`.

When enabled, the chart emits:

- a `ResourceQuota` covering `requests.cpu`, `requests.memory`, `limits.cpu`, `limits.memory`, `requests.storage`, `persistentvolumeclaims`, and a hard `pods` ceiling, with values straight from `quota.hard.*`;
- a `LimitRange` of type `Container` carrying `defaultRequest` / `default` / `max`, so an ad-hoc pod (`kubectl debug`, an operator-applied tool) that omits resources still inherits a request/limit pair and isn't rejected by the `ResourceQuota`'s `limits.*` admission check. Disable with `quota.limitRange.enabled=false` if your cluster already has an equivalent LimitRange in place.

Both resources land at ArgoCD `sync-wave: "-10"`, before the bootstrap Job at wave `-5`, so the quota is enforced from the first scheduled pod. Helm's built-in kind-sort apply order places `ResourceQuota` / `LimitRange` before workloads, so the same template works for `helm install` without an explicit hook.

Chart defaults are sized for a single combined-mode install (apiserver pod only) with external Postgres/Redis/object-storage — `1500m / 2Gi` requests, `3 / 3Gi` limits, `10Gi` storage. Installs that enable `demo.*` sidecars or split mode should raise the caps. The shared overlay `infra/helm-overlays/preview-base.values.yaml` already overrides `quota.hard.*` to fit the preview demo bundle (postgres + redis + minio + apiserver + transient setup / init-data Jobs).

Operational notes:

- A pod that would push the namespace over a hard cap fails admission with a `Forbidden: exceeded quota` event — the failure is fast and the message names the offending dimension, satisfying the "fails fast with a clear message" acceptance criterion from #1866.
- The `pods` ceiling counts only pods in a non-terminal state (Pending / Running). Succeeded and Failed Job pods do not consume the quota — they are pruned in the background by `kube-controller-manager`'s PodGC once `terminated-pod-gc-threshold` is exceeded. The ceiling therefore caps *in-flight* pods (active workloads + transient Job pods overlapping a sync), not the historical record. See [kubernetes/kubernetes#51150](https://github.com/kubernetes/kubernetes/issues/51150) for the upstream design discussion. To defensively cap accumulated Job objects too, add an explicit `count/jobs.batch` line to `quota.hard`.
- When `persistence.enabled=true` and `persistence.size` ≥ `quota.hard.requests.storage`, the single uploads PVC consumes the entire storage quota and any additional PVC will be rejected. Raise `quota.hard.requests.storage` (or trim `persistence.size`) for installs that need both.
- Overriding `quota.hard.*` via `--set` requires escaping the dots: `--set 'quota.hard.requests\.cpu=2'`. Without the escape, Helm parses the path as a nested map (`{requests: {cpu: 2}}`) and the ResourceQuota renders an unrecognized resource that kubectl rejects. Prefer `--values` (or a values file) for non-trivial overrides.
- The pivot away from a queue bot (per the [2026-05-24 issue update](https://github.com/denisvmedia/inventario/issues/1866#issuecomment-4528708945)) makes the per-namespace quota itself the concurrency cap for PR previews: when the cluster runs out of capacity, the next ArgoCD `Application` stays `Pending` until an older preview is `/destroy`-ed.

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
| `run.workers.common.*` | see `values.yaml` | Shared defaults deep-merged into each per-group worker block. |
| `run.workers.<group>.enabled` | `false` | Turn on a worker Deployment for the given group (`archive`, `emails`, `housekeeping`, `media`). |
| `run.workers.<group>.replicaCount` | `1` | Scale the worker pool for the given group. Review caveats for `archive`. |
| `run.workers.<group>.autoscaling.enabled` | `false` | Enable to emit an HPA for the given worker group. |
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
| `setupJob.argocdMode` | `false` | Enable for ArgoCD-managed installs to use the sync-wave layout that supports in-place upgrades with new migrations. See [ArgoCD-managed migrations](#argocd-managed-migrations). |
| `setupJob.bootstrap.enabled` | `true` | Disable if DB bootstrap/role management is handled outside Helm. |
| `setupJob.bootstrap.superuserDsn` | `""` | Provide when bootstrap needs elevated DB privileges. |
| `setupJob.initData.adminEmail` | `admin@example.com` | Set the initial admin login name. |
| `setupJob.initData.adminPassword` | `""` | Required on first install unless provided by `secrets.existingSecret`. |
| `setupJob.initData.seedDatabase` | `false` | Enable only for demo/sample data seeding. |
| `demo.postgresql.enabled` | `false` | Turn on in-cluster demo PostgreSQL. |
| `demo.redis.enabled` | `false` | Turn on in-cluster demo Redis. |
| `demo.minio.enabled` | `false` | Turn on in-cluster demo MinIO and S3-style uploads. |
| `quota.enabled` | `false` | Render a namespace-scoped `ResourceQuota` + `LimitRange` (#1866). Enable on multi-tenant namespaces such as PR previews; raise `quota.hard.*` when enabling `demo.*` or split mode. See [Namespace quotas](#namespace-quotas-opt-in). |
| `quota.hard` | see `values.yaml` | Map passed straight into `ResourceQuota.spec.hard`. |
| `quota.limitRange.enabled` | `true` | Set to `false` to skip the chart's `LimitRange` (e.g. when a cluster-wide LimitRange is already in place). |

## Upgrade notes

- Use `helm upgrade --install` for both first install and upgrades.
- The setup hook re-runs on upgrades to keep bootstrap/migrations safe and idempotent.
- Demo bucket creation is a `post-install,post-upgrade` hook.
- Demo database seeding is skipped on upgrades even if `setupJob.initData.seedDatabase=true`.
- **Worker roles consolidated (breaking values change)**: `run.workers.{thumbnails,exports,imports,restores,emails,tokenCleanup}` have been replaced by the four groups `run.workers.{archive,emails,housekeeping,media}`. Rename values before upgrading:
  - `run.workers.thumbnails` → `run.workers.media`
  - `run.workers.{exports,imports,restores}` → single `run.workers.archive`
  - `run.workers.tokenCleanup` → `run.workers.housekeeping`
  - `run.workers.emails` unchanged

  The CLI `--workers-only` / `--workers-exclude` flags still accept the old family names (`exports`, `imports`, `restores`, `thumbnails`, `token-cleanup`) for one release and log a deprecation warning; Helm values surface only the group keys.

### Upgrading from chart 0.2.0 → 0.3.x

Chart 0.3.0 reshaped the run-topology surface from a flat set of pod-shape keys at the chart root into the `run.*` hierarchy that supports both combined (`run all`) and split (`run apiserver` + `run workers`) deployments. The following top-level keys were **removed**; if your existing `values.yaml` (or `--set` arguments) still sets them, the chart silently falls back to its new defaults and your overrides are lost. Move each one under the matching `run.*` block before upgrading.

| 0.2.0 (top-level) | 0.3.x (combined mode) | 0.3.x (split mode — API server) | 0.3.x (split mode — worker group) |
| --- | --- | --- | --- |
| `replicaCount` | `run.all.replicaCount` | `run.apiserver.replicaCount` | `run.workers.<group>.replicaCount` |
| `resources` | `run.all.resources` | `run.apiserver.resources` | `run.workers.common.resources` (or `run.workers.<group>.resources` to override) |
| `livenessProbe` | `run.all.livenessProbe` | `run.apiserver.livenessProbe` | `run.workers.common.livenessProbe` (worker probe port defaults to `probe`, not `3333`) |
| `readinessProbe` | `run.all.readinessProbe` | `run.apiserver.readinessProbe` | `run.workers.common.readinessProbe` |
| `podAnnotations` | `run.all.podAnnotations` | `run.apiserver.podAnnotations` | `run.workers.common.podAnnotations` (or per-group) |
| `nodeSelector` | `run.all.nodeSelector` | `run.apiserver.nodeSelector` | `run.workers.common.nodeSelector` (or per-group) |
| `tolerations` | `run.all.tolerations` | `run.apiserver.tolerations` | `run.workers.common.tolerations` (or per-group) |
| `affinity` | `run.all.affinity` | `run.apiserver.affinity` | `run.workers.common.affinity` (or per-group) |
| `priorityClassName` | `run.all.priorityClassName` | `run.apiserver.priorityClassName` | `run.workers.common.priorityClassName` (or per-group) |

Other surfaces are unchanged: `image.*`, `service.*`, `ingress.*`, `serviceAccount.*`, `app.*`, `features.*`, `email.*`, `secrets.*`, `setupJob.*`, `persistence.*`, `podSecurityContext`, `containerSecurityContext`, and `demo.*` keep the same shape and meaning as in 0.2.0.

If you previously enabled `autoscaling.*` at the chart root, that block was never wired in 0.2.0; 0.3.x introduces it as `run.<role>.autoscaling.*`.

A typical upgrade looks like:

```diff
-replicaCount: 2
-resources:
-  requests:
-    cpu: 200m
-    memory: 384Mi
-nodeSelector:
-  workload: inventario
+run:
+  all:
+    enabled: true
+    replicaCount: 2
+    resources:
+      requests:
+        cpu: 200m
+        memory: 384Mi
+    nodeSelector:
+      workload: inventario
```

Run `helm template ... --debug | grep -E 'replicas|resources|nodeSelector|tolerations|affinity'` after rewriting your values to confirm the new render still reflects your overrides.

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

# Split mode render (apiserver + all worker groups).
helm template inventario helm/inventario/ \
  --set-string secrets.dbDsn='postgres://user:pass@pg-host:5432/inventario?sslmode=require' \
  --set-string secrets.jwtSecret='testtesttesttesttesttesttesttest' \
  --set-string secrets.fileSigningKey='testtesttesttesttesttesttesttest' \
  --set setupJob.initData.adminPassword=test \
  --set run.all.enabled=false \
  --set run.apiserver.enabled=true \
  --set run.workers.archive.enabled=true \
  --set run.workers.emails.enabled=true \
  --set run.workers.housekeeping.enabled=true \
  --set run.workers.media.enabled=true

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
