# Inventario monitoring stack (Prometheus + Grafana)

Opt-in observability stack for local/dev use, added in issue #843. The
application always exposes Prometheus metrics at `/metrics`; this directory adds
a Prometheus to scrape them and a Grafana with a pre-provisioned dashboard.

## Quick start

```bash
docker compose --profile monitoring up -d
```

| Service    | URL                     | Notes                                   |
| ---------- | ----------------------- | --------------------------------------- |
| App        | http://localhost:3333   | `/metrics` is the scrape target         |
| Prometheus | http://localhost:9090   | Status → Targets shows `inventario` UP  |
| Grafana    | http://localhost:3000   | dashboard **Inventario / Overview**     |

Grafana default login is `admin` / `admin` (override with `GRAFANA_ADMIN_USER` /
`GRAFANA_ADMIN_PASSWORD`). Anonymous **Viewer** access is enabled for dev
convenience — **never enable that on an internet-exposed deployment.**

The stack is gated behind the compose `monitoring` profile, so a plain
`docker compose up` and the e2e stack do not start it.

## Two clocks: scrape interval vs. business-collector tick

- **Prometheus `scrape_interval` = 15s** (`prometheus/prometheus.yml`): how often
  Prometheus reads `/metrics`. Counters/histograms (HTTP, DB, auth, email,
  rate-limit) change in real time, so 15s gives good `rate()` resolution.
- **Business-collector tick = 60s** (app config
  `INVENTARIO_RUN_BUSINESS_METRICS_INTERVAL`): how often the app recomputes the
  installation-wide business gauges (`inventario_users`, `inventario_commodities`,
  `inventario_file_storage_bytes`, …). They step every 60s and read flat in
  between — that is expected, not a gap.

## Files

```
deploy/monitoring/
├── prometheus/
│   ├── prometheus.yml            # scrape config (job: inventario → inventario:3333)
│   └── rules/inventario.rules.yml# recording rules + alerts (5xx ratio, p95, target down)
└── grafana/
    ├── provisioning/
    │   ├── datasources/prometheus.yml   # Prometheus datasource (uid inventario-prometheus)
    │   └── dashboards/inventario.yml    # provider: read-only file provisioning
    └── dashboards/
        └── inventario-overview.json     # the dashboard
```

## Metric families → dashboard panels

The dashboard PromQL is the contract with the app's metric names. If a metric is
renamed in `go/internal/metrics`, update the matching panel/rule here.

| Family                                            | Type            | Panel(s)                          |
| ------------------------------------------------- | --------------- | --------------------------------- |
| `inventario_http_requests_total`                  | counter         | request rate, 5xx ratio, classes  |
| `inventario_http_request_duration_seconds`        | histogram       | latency p50/p95/p99               |
| `inventario_http_requests_in_flight`              | gauge           | in-flight                         |
| `inventario_db_query_duration_seconds`            | histogram       | DB query latency p95 by op        |
| `inventario_db_queries_total`                     | counter         | DB queries by op & status         |
| `inventario_db_pool_connections` / `…_max_connections` | gauge      | DB pool connections               |
| `inventario_auth_login_attempts_total`            | counter         | login attempts by outcome         |
| `inventario_auth_tokens_issued_total`             | counter         | tokens issued by type             |
| `inventario_rate_limit_rejections_total`          | counter         | rate-limit rejections by scope    |
| `inventario_email_queue_depth`                    | gauge           | email queue depth                 |
| `inventario_emails_processed_total`               | counter         | emails processed by status        |
| `inventario_tenants/users/…/commodities/files`    | gauge           | business entity counts            |
| `inventario_file_storage_bytes`                   | gauge           | file storage by category          |
| `up{job="inventario"}`                            | synthetic       | scrape target up                  |

## Split deployments (`run apiserver` + `run workers`)

The default compose runs `run all` (one process), so every metric — HTTP, DB,
auth, email, and the business gauges — is exported on `:3333` and the dashboard
works out of the box.

In a **split** deployment the producers are split across processes:

- The **apiserver** process exports HTTP / auth / DB metrics on `:3333`.
- The **workers** process exports the email metrics and the business gauges
  (`inventario_users`, `inventario_commodities`, `inventario_file_storage_bytes`,
  `inventario_email_queue_depth`, …) on its probe port `:3334`.

The business/email gauges also exist (at `0`) in the apiserver process because
they are package-level, so a Prometheus that scrapes **only** the apiserver
target will show those panels flat at `0`. To get real values, scrape the
workers target too (uncomment the `inventario-workers` job in `prometheus.yml`,
or in k8s add the worker PodMonitor — see `helm/inventario/README.md`).

Because these gauges are installation-wide (every producer reports the same
total), the dashboard collapses them across targets with `max()` rather than
`sum()` — e.g. `max(inventario_users)`,
`max by (category) (inventario_file_storage_bytes)`,
`max(inventario_email_queue_depth)`. `max()` drops the apiserver `0` series and,
crucially, does not double-count when more than one `run all`/worker replica is
scraped. (Counter/histogram panels — request rate, latency — correctly keep
`sum(rate(...))`, since those aggregate per-process activity across the fleet.)

## Kubernetes

For cluster scraping see the chart's **Metrics & scraping** section in
`helm/inventario/README.md` (`metrics.podAnnotations` / `metrics.serviceMonitor`).
The app serves `/metrics` on the API port `3333` and, in split deployments, on
each worker's probe port `3334`.

## Security: keep `/metrics` off untrusted networks

`/metrics` is unauthenticated (as it was before #843) and now also publishes
installation-wide aggregate gauges (tenant/user/commodity counts, total storage
bytes). These are aggregates only — **no per-tenant data and no secrets** — but
they still reveal rough scale. Expose `/metrics` only to the monitoring network
(the in-cluster Prometheus via ServiceMonitor / `kubernetes_sd`, or a private
interface), never to the public internet.

## After editing Prometheus config

```bash
curl -X POST http://localhost:9090/-/reload   # --web.enable-lifecycle is set
```
