# Inventario monitoring stack (Prometheus + Grafana)

Opt-in observability stack for local/dev use, added in issue #843. The
application always exposes Prometheus metrics at `/metrics`; this directory adds
a Prometheus to scrape them, an Alertmanager to route what fires, and a Grafana
with a pre-provisioned dashboard.

## Quick start

```bash
docker compose --profile monitoring up -d
```

| Service      | URL                   | Notes                                    |
| ------------ | --------------------- | ---------------------------------------- |
| App          | http://localhost:3333 | `/metrics` is the scrape target          |
| Prometheus   | http://localhost:9090 | Status → Targets shows `inventario` UP   |
| Alertmanager | http://localhost:9093 | Status → Config; alerts land here        |
| Grafana      | http://localhost:3000 | dashboard **Inventario / Overview**      |

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
├── alertmanager/
│   └── alertmanager.yml          # routing + inhibition; receivers are yours to fill
└── grafana/
    └── provisioning/
        ├── datasources/prometheus.yml   # Prometheus datasource (uid inventario-prometheus)
        └── dashboards/inventario.yml    # provider: read-only file provisioning

helm/inventario/files/grafana-dashboards/
└── inventario-overview.json             # the dashboard (compose mounts it from here)
```

The dashboard JSON lives in the Helm chart, not here. The chart publishes it as a
Grafana-sidecar ConfigMap for Kubernetes installs (#2034), and this compose stack
bind-mounts the same file, so the two cannot drift. The dashboard is
environment-agnostic — it selects its datasource through a `${datasource}` variable
rather than hardcoding one.

The rules file is the reverse: it stays here because it is compose-specific. A
Prometheus scraping via `kubernetes_sd` has no static `job="inventario"` label, so the
chart's `PrometheusRule` carries the same alerts with cluster-shaped `up` selectors. A
`helm-lint` step compares the two alert name sets on every PR, so the expressions may
differ but the alert set cannot.

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

## Alerting

Prometheus evaluates the rules and hands whatever fires to Alertmanager at
`alertmanager:9093`. Without that hand-off the rules still evaluate and nothing
leaves the machine, which from the outside is indistinguishable from nothing
being wrong.

`alertmanager/alertmanager.yml` ships the routing and leaves the receivers
empty, because a chart cannot know where your pages should go and a default
that guesses is either wrong or ignored. Three names are referenced by the
routes — `default`, `backups`, `critical` — so keep them or rename them in both
places.

What the routing does:

- Anything matching `InventarioBackup.*` goes to `backups` with `group_wait: 0s`
  and a 12-hour `repeat_interval`. There is nothing to batch a backup alert
  with, and a backup that has not run is no more informative 30 seconds later.
- Everything else at `severity: critical` goes to `critical`.
- `InventarioBackupNeverRan` inhibits `InventarioBackupStale` within the same
  `namespace`. A backup that has never run makes "the newest dump is old"
  redundant — there is no newest dump — and both fire together on a fresh
  install whose backup config is broken. The `namespace` equality matters: a
  broken backup in one namespace must not silence a real one elsewhere.

To fill in a receiver, add a `*_configs` block under its name:

```yaml
receivers:
  - name: backups
    slack_configs:
      - api_url: https://hooks.slack.com/services/...
        channel: "#ops"
```

Check it before restarting, and confirm the routing still does what you expect:

```bash
docker compose --profile monitoring exec alertmanager \
  amtool check-config /etc/alertmanager/alertmanager.yml
docker compose --profile monitoring exec alertmanager \
  amtool config routes test --config.file=/etc/alertmanager/alertmanager.yml \
  alertname=InventarioBackupStale severity=warning
```

Then prove the path end to end rather than assuming it, by posting an alert
directly to Alertmanager and waiting for it to arrive wherever you sent it:

```bash
curl -s -XPOST http://localhost:9093/api/v2/alerts -H 'Content-Type: application/json' -d '[
  {"labels":{"alertname":"InventarioBackupNeverRan","namespace":"default","severity":"critical"},
   "annotations":{"summary":"delivery test, ignore"}}
]'
```

A rule that fires and a page that arrives are different claims. The first is
unit-tested (`scripts/test-alert-rules.sh`); only you can check the second,
because it ends at a receiver this repository does not own.

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

Alertmanager has no lifecycle endpoint enabled here, so restart it instead:

```bash
docker compose --profile monitoring restart alertmanager
```
