# Inventario — Production Release & Deployment Runbook

This is a **runbook**: a top-to-bottom checklist you can execute yourself to ship
Inventario to production on Kubernetes. It has two parts:

- **Part A — Cut the release**: turn a green `master` commit into an immutable,
  published container image.
- **Part B — Deploy to Kubernetes**: stand the release up on a cluster.

It is written to be **distribution-agnostic**. The default worked example targets
**k3s**, and every step that differs between distributions calls out **k3s / GKE /
DigitalOcean DOKS** explicitly. Nothing here is specific to the maintainer's private
ArgoCD/vcluster infrastructure under `infra/` — that is a separate, personal setup.

> For a single-box / systemd install (no Kubernetes) see [`DEPLOYMENT.md`](DEPLOYMENT.md).
> For local evaluation see [`QUICKSTART.md`](QUICKSTART.md) and [`DOCKER.md`](DOCKER.md).

## Conventions

Placeholders you substitute (no angle brackets in the final command):

| Placeholder | Meaning | Example |
| --- | --- | --- |
| `<VERSION>` | release tag you are cutting/deploying | `v0.1.0` |
| `<DOMAIN>` | public hostname | `inventario.example.com` |
| `<NAMESPACE>` | target namespace | `inventario` |
| `<R2_ACCOUNT_ID>` | Cloudflare account id | `a1b2c3…` |
| `<R2_BUCKET>` | R2 bucket name | `inventario-prod` |

Each `- [ ]` is a step to tick off. ⚠️ marks a footgun.

## Architecture at a glance

Inventario is a single Go binary that embeds the React frontend and serves the API +
SPA on **port 3333**. In Kubernetes the Helm chart (`helm/inventario/`) runs it either
**combined** (`run all`, the default — API + all background workers in one Deployment)
or **split** (`run apiserver` + per-group worker Deployments). This runbook uses
combined mode; see `helm/inventario/README.md` → "Split deployment mode" to scale tiers
independently.

Runtime dependencies:

| Dependency | Required? | Notes |
| --- | --- | --- |
| **PostgreSQL** | **Yes** | 12+. `memory://` is dev-only (data lost on restart). |
| **Object storage** | Recommended | `file://` (single replica + PVC) **or** S3/R2/GCS/Azure (any replica count). |
| **Redis** | Recommended | Token blacklist, auth/global rate limit, CSRF, email queue. In-memory fallback warns and is single-instance only. |
| **SMTP / email provider** | For real email | `stub` (default) only logs. Registration, password reset, invites, magic-link need a real provider. |
| **AI vision provider** | Optional | `none` (default), `mock`, `anthropic`, `openai`. Fails-loud if a real provider is selected without its API key. |
| **Prometheus + Grafana** | **Go-live gate** | See Part B §B10 and blocker [#2034](https://github.com/denisvmedia/inventario/issues/2034). |

The chart's source of truth for every value and secret key is
[`helm/inventario/values.yaml`](helm/inventario/values.yaml) and
[`helm/inventario/README.md`](helm/inventario/README.md); this runbook references them
rather than duplicating the full surface.

## Pre-flight decisions

Fill these in before you start; they parameterize the rest of the runbook.

| Decision | This runbook's worked example |
| --- | --- |
| Release version | `v0.1.0` (first tag) |
| Kubernetes distro | k3s (GKE / DOKS notes inline) |
| PostgreSQL | **External** — managed or in-cluster operator (your choice; see §B3) |
| Object storage | **Cloudflare R2** (S3-compatible) |
| Email provider | **Mailtrap** (SMTP) |
| AI vision | **Anthropic** |
| Redis | **In-cluster** (chart's bundled Redis) |
| Public scan endpoint | Off (enable deliberately — see §B7) |

---

## Part A — Cut the release

Pushing a `vX.Y.Z` tag is what publishes the production image. Until you do this,
`ghcr.io/denisvmedia/inventario:<VERSION>` and `:latest` **do not exist** (only
`edge` / `master` / `sha-<commit>` from master pushes), and a default `helm install`
would `ImagePullBackOff` (see [#2035](https://github.com/denisvmedia/inventario/issues/2035)).

- [ ] **Confirm `master` CI is green.** ⚠️ Tagging does **not** wait for CI — verify
  first. The gates: `go-test`, `go-test-postgres`, `go-lint`, `go-swagger-docs`,
  `frontend-lint`, `frontend-test`, `frontend-codegen`, `frontend-i18n`, `docker`,
  `e2e-tests`, `helm-lint`, and `release` (goreleaser dry-run).
- [ ] **Pick the version.** First release is `v0.1.0` (pre-1.0 = no SemVer API-stability
  promise yet). Use annotated tags.
- [ ] **Delete the stale `v0.0.1` *draft* GitHub Release** if present (it is an untagged
  June-2025 leftover and only causes confusion).
- [ ] **Tag the exact `master` commit and push:**

  ```bash
  git checkout master && git pull --ff-only
  git tag -a v0.1.0 -m "Inventario v0.1.0"
  git push origin v0.1.0
  ```

- [ ] **Let CI build the release.** The tag push fires two workflows:
  - `release.yml` → goreleaser publishes the GitHub Release (multi-platform binaries,
    archives, checksums, changelog).
  - `docker.yml` → native amd64 **and** arm64 builds merged into a multi-arch manifest,
    pushed to GHCR as `:v0.1.0`, `:v0.1`, `:v0`, and `:latest`.
- [ ] **Verify the image landed (multi-arch):**

  ```bash
  docker buildx imagetools inspect ghcr.io/denisvmedia/inventario:v0.1.0
  ```

  You should see both `linux/amd64` and `linux/arm64`.
- [ ] **Verify the GitHub Release** is published with binaries + `checksums.txt`.
- [ ] **Record the immutable tag** you will deploy: `ghcr.io/denisvmedia/inventario:v0.1.0`.
  ⚠️ Always deploy a concrete `:vX.Y.Z`, never `:latest` (the chart `appVersion` is
  `latest`, so you must pass `--set image.tag=v0.1.0` — [#2035](https://github.com/denisvmedia/inventario/issues/2035)).

---

## Part B — Deploy to Kubernetes

### B1. Cluster prerequisites

You need a cluster, an ingress controller, a default StorageClass, and (for HTTPS)
cert-manager.

| Concern | k3s | GKE | DigitalOcean DOKS |
| --- | --- | --- | --- |
| Ingress controller | Traefik built-in → `ingressClassName: traefik` (or install ingress-nginx) | GCE ingress (`gce`) or install ingress-nginx | install ingress-nginx (1-click) |
| External IP / LB | ServiceLB (Klipper) → node IP, or MetalLB | cloud LB auto-provisioned | DO LB auto-provisioned |
| Default StorageClass | `local-path` (RWO, node-bound) | `standard-rwo` / `premium-rwo` | `do-block-storage` |
| TLS | cert-manager + Let's Encrypt `ClusterIssuer` (HTTP-01) | same | same |

- [ ] Confirm `kubectl get nodes` is healthy and `kubectl get storageclass` shows a
  default.
- [ ] **Install cert-manager** (universal) and a `ClusterIssuer`:

  ```bash
  helm repo add jetstack https://charts.jetstack.io && helm repo update
  helm upgrade --install cert-manager jetstack/cert-manager \
    --namespace cert-manager --create-namespace --set crds.enabled=true
  ```

  ```yaml
  # clusterissuer.yaml — apply with: kubectl apply -f clusterissuer.yaml
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: letsencrypt-prod
  spec:
    acme:
      server: https://acme-v02.api.letsencrypt.org/directory
      email: you@example.com
      privateKeySecretRef:
        name: letsencrypt-prod
      solvers:
        - http01:
            ingress:
              ingressClassName: traefik   # GKE: gce or nginx · DOKS: nginx
  ```

- [ ] **Point DNS** for `<DOMAIN>` (A/AAAA) at the ingress external IP
  (`kubectl get svc -A | grep -i ingress`; on k3s ServiceLB this is a node IP).
- [ ] **Get the chart.** It is not published to a chart repo — use it from a git
  checkout at the release tag:

  ```bash
  git clone https://github.com/denisvmedia/inventario.git
  cd inventario && git checkout v0.1.0   # chart pinned to the same release
  ```

### B2. Generate and store secrets

- [ ] Generate two **stable** signing secrets (and optionally a backup signing key):

  ```bash
  openssl rand -hex 32   # → JWT secret
  openssl rand -hex 32   # → file signing key
  ```

  ⚠️ **Keep these constant forever.** Rotating the JWT secret invalidates **all
  sessions and every MFA enrollment** (MFA secrets are HKDF-derived from it; see
  `DEPLOYMENT.md` → MFA). A rotating file signing key breaks every outstanding signed
  download URL. If either is left empty, the app auto-generates an ephemeral one at
  boot and logs it — never rely on that in production.
- [ ] Decide how secrets reach the cluster. This runbook uses an **existing Secret**
  (`secrets.existingSecret`) you create out-of-band — clean for GitOps and for a
  secrets manager (sops / sealed-secrets / external-secrets). The full key list is in
  `helm/inventario/README.md` → "Secret handling". The concrete command is in
  [Appendix B](#appendix-b-create-the-runtime-secret).

### B3. PostgreSQL (external, required)

⚠️ The chart's `demo.postgresql` is **not** production-grade (trivial password, no
persistence/HA) — never use it in production.

- [ ] Provision PostgreSQL 12+. Options (your call — TBD):
  - **Managed**: Neon, Supabase, Google Cloud SQL (GKE), DO Managed PostgreSQL (DOKS).
  - **In-cluster operator**: [CloudNativePG](https://cloudnative-pg.io/) gives a proper
    HA Postgres with backups on a PVC — a good fit for self-hosted k3s.
- [ ] Create the database and an **app** user, plus a **migrator** user — or provide a
  superuser DSN and let the chart's bootstrap step create the migrator role for you
  (`setupJob.bootstrap.superuserDsn`). Use `sslmode=require`.
- [ ] Record two DSNs for the Secret:
  - `INVENTARIO_DB_DSN` — app user: `postgres://inventario:…@<host>:5432/inventario?sslmode=require`
  - `MIGRATOR_DB_DSN` — migrator user (falls back to the app DSN if omitted).

### B4. Object storage — Cloudflare R2

Using external object storage means uploads survive pod restarts and you can run more
than one replica (set `persistence.enabled=false`).

- [ ] In the Cloudflare dashboard: create an **R2 bucket** `<R2_BUCKET>` and an **R2 API
  token** (Object Read & Write). Note the **access key id**, **secret access key**, and
  your **S3 endpoint** `https://<R2_ACCOUNT_ID>.r2.cloudflarestorage.com`.
- [ ] Upload location (goes in `app.uploadLocation`; mirrors the proven `k8s/dev` MinIO
  form, which `gocloud.dev/s3blob` v0.45 parses for `endpoint`/`region`/`prefix`):

  ```text
  s3://<R2_BUCKET>?prefix=uploads/&region=auto&endpoint=https://<R2_ACCOUNT_ID>.r2.cloudflarestorage.com
  ```

- [ ] Put the R2 credentials in the Secret as `AWS_ACCESS_KEY_ID` /
  `AWS_SECRET_ACCESS_KEY`.
- [ ] ⚠️ **R2 checksum caveat.** `aws-sdk-go-v2` (v1.42.0) defaults to sending request
  checksums (CRC32) that R2 has historically rejected (uploads fail with HTTP 501 /
  signature errors). If you hit that, set these two env vars — the chart loads the whole
  Secret as env, so just add them as keys in the runtime Secret (or via a ConfigMap in
  `extraEnvFrom`):

  ```text
  AWS_REQUEST_CHECKSUM_CALCULATION=when_required
  AWS_RESPONSE_CHECKSUM_VALIDATION=when_required
  ```

  Try without them first; add if uploads fail.

### B5. Email — Mailtrap (SMTP)

- [ ] In Mailtrap, use a **Sending** stream (not the Sandbox) and copy its SMTP
  credentials. Set:
  - `email.provider=smtp`, `email.smtp.host=live.smtp.mailtrap.io`, `email.smtp.port=587`,
    `email.smtp.useTls=true`, `email.smtp.username=<user>`.
  - `email.from=<verified sender>` (must be a non-empty address on a domain you verified
    in Mailtrap).
  - `app.publicUrl=https://<DOMAIN>` so links inside emails resolve.
  - SMTP password → Secret key `INVENTARIO_RUN_SMTP_PASSWORD`.
- [ ] (Optional) `email.replyTo`. Leave `email.logUrls=false` in production (tokens in
  logs).

### B6. Redis — in-cluster

- [ ] Quick path: set `demo.redis.enabled=true`. The chart wires **all** Redis-backed
  features (token blacklist, auth + global rate limit, CSRF, email queue) to the bundled
  Redis automatically and clears the in-memory "not suitable for multi-instance"
  warnings.
- [ ] ⚠️ The bundled Redis is `emptyDir`, single-instance, no auth — on restart it loses
  the token blacklist, queued emails, and rate-limit counters. Acceptable for a homelab.
  For durability, deploy a standalone Redis (Bitnami chart or a StatefulSet + PVC),
  leave `demo.redis.enabled=false`, and set the five `secrets.*RedisUrl` values
  (`redis://:password@host:6379/0` …) instead.

### B7. AI vision — Anthropic

- [ ] Set `aivision.provider=anthropic` and put the key in the Secret as
  `INVENTARIO_RUN_AI_VISION_ANTHROPIC_API_KEY`. Model defaults to `claude-sonnet-4-6`
  (`aivision.anthropicModel`). ⚠️ The apiserver **fails to boot** if the provider is
  `anthropic` but the key is empty.
- [ ] Decide on the **public** scan endpoint. `aivision.publicScanEnabled=false`
  (default) keeps the landing-page "Add your first item" scan behind auth. Set it `true`
  only if you want the unauthenticated CTA — every call then spends Anthropic tokens with
  no login wall (bounded by per-IP + daily caps and `maxPhotos`/`maxPhotoBytes`).

### B8. Ingress + TLS

- [ ] Enable ingress in your values: `ingress.enabled=true`, `className` per distro,
  host `<DOMAIN>`, and a cert-manager-issued TLS secret:

  ```yaml
  ingress:
    enabled: true
    className: traefik   # GKE: gce/nginx · DOKS: nginx
    annotations:
      cert-manager.io/cluster-issuer: letsencrypt-prod
    hosts:
      - host: inventario.example.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - secretName: inventario-tls
        hosts:
          - inventario.example.com
  ```

- [ ] ⚠️ Ensure the controller forwards `X-Forwarded-Proto: https` (Traefik, ingress-nginx,
  GCE, and DO all do by default). Auth cookies are marked `Secure` based on `r.TLS` **or**
  that header — without it, cookies over a TLS-terminating proxy would not get `Secure`.
- [ ] nginx only: add `nginx.ingress.kubernetes.io/proxy-body-size: "100m"` so large file
  uploads aren't rejected.

### B9. Install and verify

- [ ] Assemble your `values-prod.yaml` ([Appendix A](#appendix-a-worked-values-prodyaml-k3s))
  and create the runtime Secret ([Appendix B](#appendix-b-create-the-runtime-secret)).
- [ ] **Dry-run render** and eyeball it (no secrets are printed — they come from the
  existing Secret):

  ```bash
  helm template inventario ./helm/inventario -n <NAMESPACE> \
    -f values-prod.yaml --set image.tag=v0.1.0 | less
  ```

- [ ] **Install / upgrade:**

  ```bash
  helm upgrade --install inventario ./helm/inventario \
    -n <NAMESPACE> --create-namespace \
    -f values-prod.yaml --set image.tag=v0.1.0
  ```

  The setup hook runs `bootstrap → migrate → init-data` (idempotent). It creates the
  default tenant + admin from `setupJob.initData.*`.
  > **ArgoCD users:** set `setupJob.argocdMode=true` instead, which moves migrations into
  > an init container coupled to the app image revision (sync-wave layout — see the chart
  > README → "ArgoCD-managed migrations").
- [ ] **Wait for rollout and health:**

  ```bash
  kubectl -n <NAMESPACE> rollout status deploy/inventario --timeout=10m
  kubectl -n <NAMESPACE> port-forward deploy/inventario 3333:3333 &
  curl -fsS localhost:3333/healthz   # process alive
  curl -fsS localhost:3333/readyz    # DB + Redis reachable
  ```

- [ ] **Log in** at `https://<DOMAIN>` with the `setupJob.initData.adminEmail` /
  admin password.
- [ ] **Create the back-office (platform-operator) account — manually in production.**
  Do **not** enable `backofficeUser` in prod (that writes a password into a Secret). Run
  the CLI inside the running pod, which already has the DB DSN in its env:

  ```bash
  kubectl -n <NAMESPACE> exec -it deploy/inventario -- \
    inventario backoffice bootstrap --email ops@example.com --name "Operations"
  kubectl -n <NAMESPACE> exec -it deploy/inventario -- \
    inventario backoffice mfa setup --email ops@example.com   # enroll TOTP
  ```

- [ ] **Confirm the unauthenticated seed endpoint stays off.** `POST /api/v1/seed` runs a
  privileged, RLS-bypassing seed (an anonymous caller could pollute your tenant and lock
  your main currency — #201). As of [#2039](https://github.com/denisvmedia/inventario/issues/2039)
  the route is **only mounted when `INVENTARIO_RUN_ENABLE_SEED_ENDPOINT=true`** (default
  `false`), so a production install that leaves the flag unset never exposes it — verify
  `curl -X POST https://<DOMAIN>/api/v1/seed` returns `404`.
- [ ] (Optional) **Defense-in-depth: also deny the path at the ingress.** No longer required
  now that the route is unmounted by default, but a belt-and-braces block keeps the path
  404 even if someone later flips the flag by mistake:

  ```yaml
  # ingress-nginx: add to the Inventario Ingress annotations
  nginx.ingress.kubernetes.io/server-snippet: |
    location = /api/v1/seed { return 404; }
  ```

  (Traefik: attach a middleware that blocks the `/api/v1/seed` path; GCE: an equivalent
  URL-map rule or an upstream WAF.)

### B10. Monitoring & alerting — required go-live gate

⚠️ **Do not call the deployment "done" until alerts fire to a real channel.** The app
ships `/metrics` and the building blocks exist (`deploy/monitoring/`), but production
monitoring is **not yet turnkey** — tracked as blocker
[#2034](https://github.com/denisvmedia/inventario/issues/2034). Until that lands, wire it
manually:

- [ ] **Deploy a monitoring stack** (Prometheus + Grafana + Alertmanager). The simplest
  universal path is `kube-prometheus-stack`:

  ```bash
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
  helm upgrade --install kps prometheus-community/kube-prometheus-stack \
    -n monitoring --create-namespace
  ```

  (GKE: Google Cloud Managed Service for Prometheus is an alternative. DOKS: the same
  Helm chart, or DO's monitoring add-on for node metrics.)
- [ ] **Enable scraping in the Inventario release** (add to `values-prod.yaml`,
  then `helm upgrade`):

  ```yaml
  metrics:
    serviceMonitor:
      enabled: true
      labels:
        release: kps          # so the Operator selects it
    podAnnotations:
      enabled: true           # needed to scrape worker pods in split mode
  ```

- [ ] **Import the dashboard**: `deploy/monitoring/grafana/dashboards/inventario-overview.json`
  ("Inventario / Overview") into Grafana.
- [ ] **Load the alert rules** as a `PrometheusRule` (port the recording rules + alerts
  from `deploy/monitoring/prometheus/rules/inventario.rules.yml` — `InventarioTargetDown`,
  `InventarioHighErrorRate` (5xx > 5%), `InventarioHighLatencyP95` (p95 > 1s)) and point
  Alertmanager at a real receiver (email/Slack/PagerDuty).
- [ ] ⚠️ **Protect `/metrics` with the bearer token and keep it off the public internet.**
  It exposes installation-wide aggregate gauges (tenant/user/commodity counts, storage
  bytes). Configure the token per **§B10a** below, scrape it only from the in-cluster
  Prometheus, and never route it through the public ingress.

### B10a. Metrics authentication — required security hardening (#2102)

⚠️ **`/metrics` (API `:3333` and each worker `:3334`) leaks installation-wide gauges**
(`inventario_tenants`, `inventario_users`, `inventario_commodities`,
`inventario_file_storage_bytes`). When no token is configured the endpoint is **open** and
the app logs a one-time startup warning. Gate it with a bearer token:

- [ ] **Generate a metrics token** (strong random, ≥ 32 bytes):

  ```bash
  openssl rand -hex 32   # → INVENTARIO_RUN_METRICS_TOKEN
  ```

- [ ] **Store it in the runtime Secret** (alongside `INVENTARIO_RUN_JWT_SECRET`,
  `INVENTARIO_RUN_FILE_SIGNING_KEY`, …) under the key `INVENTARIO_RUN_METRICS_TOKEN`
  (see Appendix B). The chart loads the whole Secret as env vars, so every workload
  (`run.all` / `run.apiserver` / workers) receives the token and enforces
  `Authorization: Bearer <token>` on `/metrics`, rejecting mismatches with HTTP 401.

- [ ] **Or set it inline** with `metrics.token` (mirrored into the chart-managed Secret as
  `INVENTARIO_RUN_METRICS_TOKEN`). Use the Secret key when `secrets.existingSecret` is set —
  `metrics.token` is ignored in that mode.

- [ ] **Wire the Prometheus Operator to send the token.** With
  `metrics.serviceMonitor.enabled=true` and a token configured (inline **or** the
  `INVENTARIO_RUN_METRICS_TOKEN` key in `secrets.existingSecret`), the chart automatically adds
  an `authorization` stanza to the ServiceMonitor pointing at the runtime Secret:

  ```yaml
  metrics:
    token: ""                  # leave empty when using secrets.existingSecret;
                               # supply INVENTARIO_RUN_METRICS_TOKEN in that Secret
    serviceMonitor:
      enabled: true
      labels:
        release: kps           # match your Prometheus Operator instance
  ```

  The Secret must live in the **same namespace** as the ServiceMonitor.

- [ ] **Vanilla (operator-less) Prometheus** discovers pods via
  `metrics.podAnnotations.enabled=true`, but pod annotations cannot carry a token — add the
  header in your scrape job's `scrape_config`:

  ```yaml
  authorization:
    type: Bearer
    credentials_file: /etc/prometheus/secrets/inventario-runtime/INVENTARIO_RUN_METRICS_TOKEN
  ```

- [ ] **Defense-in-depth: also block `/metrics` at the ingress controller.** The chart does
  NOT route it through its own ingress, but the default rule is `path: /` — block the path
  explicitly. ingress-nginx:
  `nginx.ingress.kubernetes.io/server-snippet: |` then `location = /metrics { return 401; }`.
  Traefik: attach a Middleware rejecting `/metrics`.

- [ ] **Verify** Prometheus scrapes successfully with the token — check the Prometheus UI
  `Targets` tab for the inventario ServiceMonitor endpoint showing **UP**, and confirm an
  unauthenticated `curl` to `/metrics` returns `401`.

### B10b. Disable the API docs UI in production (#2102)

- [ ] ⚠️ **Keep the interactive Swagger/OpenAPI docs off in production.** The chart defaults
  `app.enableApiDocs=false`, which sets `INVENTARIO_RUN_ENABLE_API_DOCS=false` and removes the
  `/swagger` UI so the full API surface is not advertised to anonymous callers. Leave it at
  the default for prod; only flip `app.enableApiDocs=true` on dev/preview overlays if you
  want the docs UI. Confirm `GET /swagger` returns 404 after deploy.

### B11. Backups & disaster recovery

Use independent layers — at least the first two are required:

- [ ] **PostgreSQL (logical)**: enable automated backups (managed snapshots, CloudNativePG
  scheduled backups, or a `pg_dump` CronJob). This is your primary data backup. Verify a
  restore actually works.
- [ ] **Object storage (R2)**: enable bucket versioning / lifecycle rules so uploaded files
  survive accidental deletion/overwrite.
- [ ] **Cluster-level (Velero)**: for whole-namespace disaster recovery — Kubernetes
  resource manifests **and** PersistentVolume data — install [Velero](https://velero.io)
  with an object-store backup location (S3, **Cloudflare R2**, GCS, or DO Spaces) and a
  scheduled backup of `<NAMESPACE>`. This is the layer that rebuilds the entire release
  (PVCs, Secrets, ConfigMaps, workloads) after a node or cluster loss, complementing the
  Postgres logical backup and R2 versioning above. ⚠️ R2: set `checksumAlgorithm: ""` on the
  `BackupStorageLocation` (Velero/kopia's S3 path otherwise trips R2's checksum handling).
  ⚠️ **Set a stable repository encryption key** (the kopia password, e.g.
  `velero install --secret-file ...` / a pre-created `velero-repo-credentials` Secret) and
  store it in your secrets manager. If you let Velero auto-generate it, the password lives
  only in an in-cluster Secret — a from-scratch restore on a fresh cluster then **cannot
  decrypt** the backups, defeating the purpose. Always test `velero restore` into a scratch
  namespace before relying on it.
  > Per-distro note: managed Postgres + managed object storage already cover most DR, so
  > Velero matters most when you self-host the data tier in-cluster (e.g. CloudNativePG +
  > MinIO on PVCs). On GKE/DOKS you can lean on Cloud SQL / DO Managed DB snapshots instead.
- [ ] **App-level backups (optional)**: the signed `.inb` backup/export feature (#534) lets
  an operator export the full dataset; store the Ed25519 backup signing key stably if you
  rely on it.

### B12. Post-go-live smoke test

- [ ] Register or log in; create an item.
- [ ] Upload a file/photo → confirm it lands in R2 (and renders back).
- [ ] Run an AI scan (Anthropic) on the add-item form.
- [ ] Trigger a password reset → confirm the email arrives via Mailtrap.
- [ ] Confirm the Grafana dashboard shows live traffic and a test alert routes to your
  channel.

---

## Appendix A — Worked `values-prod.yaml` (k3s)

External Postgres + Cloudflare R2 + Mailtrap + Anthropic + in-cluster Redis, combined
mode, no uploads PVC. Sensitive values come from the existing Secret in Appendix B; this
file holds only non-secret configuration.

```yaml
image:
  repository: ghcr.io/denisvmedia/inventario
  # tag is passed on the CLI: --set image.tag=v0.1.0  (do NOT rely on appVersion=latest)

run:
  all:
    enabled: true
    replicaCount: 1            # R2 storage allows >1 if you want HA

app:
  addr: ":3333"
  publicUrl: "https://inventario.example.com"
  uploadLocation: "s3://inventario-prod?prefix=uploads/&region=auto&endpoint=https://<R2_ACCOUNT_ID>.r2.cloudflarestorage.com"
  allowedOrigins: ""           # SPA is same-origin; leave empty to deny cross-origin
  enableApiDocs: false         # keep /swagger off in prod (chart default; shown for clarity)

email:
  provider: smtp
  from: "no-reply@example.com"
  smtp:
    host: live.smtp.mailtrap.io
    port: 587
    username: "<mailtrap-smtp-user>"
    useTls: true

aivision:
  provider: anthropic
  anthropicModel: claude-sonnet-4-6
  publicScanEnabled: false     # set true only to enable the unauthenticated landing CTA

demo:
  redis:
    enabled: true              # in-cluster Redis; wires all 5 Redis URLs

persistence:
  enabled: false               # uploads live in R2, not a PVC

ingress:
  enabled: true
  className: traefik
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: inventario.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: inventario-tls
      hosts:
        - inventario.example.com

secrets:
  existingSecret: inventario-runtime

setupJob:
  initData:
    defaultTenantName: "My Organization"
    defaultTenantSlug: "main"
    adminEmail: "admin@example.com"
    adminName: "Administrator"
    # adminPassword comes from the Secret key SETUP_ADMIN_PASSWORD

metrics:
  # token: ""                  # leave empty here; INVENTARIO_RUN_METRICS_TOKEN comes
                               # from the runtime Secret (Appendix B). See §B10a.
  serviceMonitor:
    enabled: true              # turn on once kube-prometheus-stack is installed
    labels:
      release: kps
  podAnnotations:
    enabled: true
```

## Appendix B — Create the runtime Secret

The chart loads this whole Secret as environment variables, so any key here becomes a
runtime env var. Generate the signing keys with `openssl rand -hex 32`.

```bash
kubectl create namespace inventario   # if not already created

kubectl -n inventario create secret generic inventario-runtime \
  --from-literal=INVENTARIO_DB_DSN='postgres://inventario:PASS@PGHOST:5432/inventario?sslmode=require' \
  --from-literal=MIGRATOR_DB_DSN='postgres://inventario_migrator:PASS@PGHOST:5432/inventario?sslmode=require' \
  --from-literal=INVENTARIO_RUN_JWT_SECRET='<openssl rand -hex 32>' \
  --from-literal=INVENTARIO_RUN_FILE_SIGNING_KEY='<openssl rand -hex 32>' \
  --from-literal=INVENTARIO_RUN_METRICS_TOKEN='<openssl rand -hex 32>' \
  --from-literal=SETUP_ADMIN_PASSWORD='<initial admin password>' \
  --from-literal=INVENTARIO_RUN_SMTP_PASSWORD='<mailtrap smtp password>' \
  --from-literal=INVENTARIO_RUN_AI_VISION_ANTHROPIC_API_KEY='<anthropic api key>' \
  --from-literal=AWS_ACCESS_KEY_ID='<r2 access key id>' \
  --from-literal=AWS_SECRET_ACCESS_KEY='<r2 secret access key>'
  # If R2 uploads fail with HTTP 501, add:
  #   --from-literal=AWS_REQUEST_CHECKSUM_CALCULATION='when_required' \
  #   --from-literal=AWS_RESPONSE_CHECKSUM_VALIDATION='when_required'
```

If your Postgres roles are created out-of-band, also add `SETUP_SUPERUSER_DSN` (or set
`setupJob.bootstrap.enabled=false`). For production, prefer a secrets manager
(sops / sealed-secrets / external-secrets) over a raw `kubectl create secret`.

## Appendix C — Upgrades

- [ ] Cut a new release (Part A) → new immutable tag `vX.Y.(Z+1)`.
- [ ] `helm upgrade --install inventario ./helm/inventario -n <NAMESPACE> -f values-prod.yaml --set image.tag=vX.Y.Z`.
- [ ] The setup hook re-runs migrations idempotently. ⚠️ **Migrations must be
  backward-compatible** (expand-contract): during the rolling update, old-image pods
  briefly serve traffic against the newly migrated schema. Spread destructive renames /
  drops across releases.
- [ ] Watch the rollout and the Grafana dashboard for error-rate / latency regressions.

## Appendix D — Rollback

- [ ] Fast path: `helm rollback inventario <PREVIOUS_REVISION> -n <NAMESPACE>`
  (`helm history inventario -n <NAMESPACE>` to find it), or re-pin the previous image tag
  and `helm upgrade`.
- [ ] ⚠️ Rolling the **image** back is safe; rolling a **schema** back is not automatic.
  If the bad release included a migration, restore from a Postgres backup or apply a
  forward fix — the embedded migrator does not auto-downgrade.

## Appendix E — Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Pods `ImagePullBackOff` on `:latest` | `image.tag` not set; `appVersion=latest` resolves to a non-existent image | `--set image.tag=vX.Y.Z` ([#2035](https://github.com/denisvmedia/inventario/issues/2035)) |
| File upload fails with HTTP 501 / checksum error | R2 rejecting `aws-sdk-go-v2` default request checksums | Add the two `AWS_*_CHECKSUM_*=when_required` keys (§B4) |
| Logged out immediately / cookies not `Secure` | ingress not forwarding `X-Forwarded-Proto: https` | enable the header on the controller (§B8) |
| Log warns "in-memory … not suitable for multi-instance" | no Redis configured | enable `demo.redis` or set the five `secrets.*RedisUrl` (§B6) |
| apiserver crashes on boot | `aivision.provider` real but API key empty | set the key, or `aivision.provider=none` (§B7) |
| Setup Job hangs / sync wedged | image tag missing from registry | fix the tag; `setupJob.activeDeadlineSeconds` caps the hang |
| Emails never arrive | `email.provider=stub` (default) | set a real provider + `email.from` (§B5) |

## Appendix F — Per-distro quick reference

| Concern | k3s | GKE | DigitalOcean DOKS |
| --- | --- | --- | --- |
| Ingress class | `traefik` (built-in) | `gce` or `nginx` | `nginx` |
| StorageClass (if using `file://` uploads) | `local-path` | `standard-rwo` | `do-block-storage` |
| Object storage equivalent | Cloudflare R2 / MinIO (`s3://…endpoint=…`) | GCS (`gs://bucket`) | Spaces (`s3://…endpoint=…`) |
| Managed Postgres option | external / CloudNativePG | Cloud SQL | DO Managed PostgreSQL |
| TLS | cert-manager (universal) | cert-manager or Google-managed certs | cert-manager |

Object storage other than R2 keeps the same shape: GCS uses `gs://<bucket>`, Azure
`azblob://<container>`, AWS S3 `s3://<bucket>?region=<region>`. Any S3-compatible service
(R2, Spaces, MinIO) uses the `s3://<bucket>?...&endpoint=<url>` form shown in §B4.
