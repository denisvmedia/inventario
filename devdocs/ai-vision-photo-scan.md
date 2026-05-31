# AI vision photo-scan

The Add-Item dialog can prefill the form from one or more product photos: the
user uploads photos, the backend asks a vision model to extract structured
fields (name, type, price, currency, serial number, URLs, purchase date,
comments), and the user reviews/accepts per-field before saving.

Tracked under #1720 (feature) and #1976 (deploy/config wiring).

This doc is the operator + developer guide for **turning the feature on**. The
application code (backend + frontend) already ships; only configuration selects
a provider and supplies a key.

## How it works (code map)

- Provider abstraction: `go/internal/aivision/` — `Provider` interface +
  `ScanRequest`/`ScanResult` types, a name→constructor registry
  (`registry.go`), and three implementations: `anthropic/` (Claude, tool-use
  forcing), `openai/` (GPT-4o, structured output), and `mock/` (deterministic
  canned result, no network).
- Service: `go/services/commodity_scan_service.go` — validation (photo count,
  per-photo bytes, MIME allowlist), per-user hourly rate limit, and an audit
  row on **every** outcome (`commodity_scan_audits` table).
- HTTP: `POST /g/{groupSlug}/commodities/scan` in
  `go/apiserver/commodity_scan.go` — multipart `photos` (+ optional `hint`),
  behind JWT + RLS + CSRF + group-role gate, with body/part size caps.
- Config: parsed in `go/cmd/inventario/run/bootstrap/config.go`, wired in
  `server_params.go` (`wireCommodityScan`).
- Frontend: `frontend/src/components/items/AiScanStep.tsx` +
  `frontend/src/features/commodities/scanApi.ts` / `scanHooks.ts`.

When the provider is `none` (the default), the route stays mounted but returns
`503` and the FE shows an "AI vision is unavailable" banner with a "Fill
manually" fallback.

## Configuration reference

All settings read from the `run` config section, i.e. the env var name is the
`INVENTARIO_RUN_` prefix + the name below.

| Env (under `INVENTARIO_RUN_`) | Default | Notes |
| --- | --- | --- |
| `AI_VISION_PROVIDER` | `none` | `none` \| `mock` \| `anthropic` \| `openai` |
| `AI_VISION_ANTHROPIC_API_KEY` | `""` | Required when provider=`anthropic` |
| `AI_VISION_ANTHROPIC_MODEL` | `claude-sonnet-4-6` | |
| `AI_VISION_ANTHROPIC_BASE_URL` | `""` | Empty = public `api.anthropic.com` |
| `AI_VISION_OPENAI_API_KEY` | `""` | Required when provider=`openai` |
| `AI_VISION_OPENAI_MODEL` | `gpt-4o` | |
| `AI_VISION_OPENAI_BASE_URL` | `""` | Empty = public `api.openai.com` |
| `AI_VISION_TIMEOUT` | `20s` | Per-call upstream deadline |
| `AI_VISION_MAX_PHOTOS` | `5` | Per scan request |
| `AI_VISION_MAX_PHOTO_BYTES` | `10485760` | 10 MiB per photo |
| `AI_VISION_RATE_LIMIT_PER_HOUR` | `30` | Per-user; `0` disables |

> ⚠️ **Fail-loud on empty key.** Selecting a real provider (`anthropic` /
> `openai`) with an empty API key makes the apiserver **fail to boot** — this is
> intentional (a misconfig should be loud, not a silent downgrade). `none` and
> `mock` never need a key.

## Local / dev quickstart

`.env` (see `.env.example`) drives `docker-compose.yaml`:

```bash
# Deterministic, no key, no network — best for UI work:
AI_VISION_PROVIDER=mock

# Real Claude extraction:
AI_VISION_PROVIDER=anthropic
AI_VISION_ANTHROPIC_API_KEY=sk-ant-...
```

Then `docker compose up -d`. The e2e stack pins `mock` (see
`docker-compose.e2e.yaml`); the `ai-scan.spec.ts` suite mainly intercepts the
network call, so it does not depend on a real provider.

## Production enablement (Helm + sops cluster)

Two halves — both required to actually serve scans:

1. **Provider selector** (non-secret, ConfigMap). Set `aivision.provider` in the
   chart values. Every preview-stack env pins `anthropic` in its ApplicationSet:
   `inv-vcl01-master` + `inv-vcl01-longevity`
   (`infra/argocd/applicationset-{master,longevity}.yaml`) and the per-PR
   previews (`applicationset-pr.yaml`). Generic `helm install` defaults to
   `none`.
2. **API key** (secret). Add `anthropic.api_key` to the sops bundle
   (`infra/vm/secrets/secrets.example.yaml` is the schema; see
   `infra/SECRETS.md` step 7). `apply-secrets.sh` delivers it two ways:
   - **master + longevity** (static namespaces): materialized into the
     `inventario-admin` Secret as `INVENTARIO_RUN_AI_VISION_ANTHROPIC_API_KEY`,
     loaded via `envFrom`.
   - **per-PR previews** (dynamic namespaces, #1976): put into a separate
     `inventario-ai-vision` Secret in `inventario-shared`, which
     **emberstack/reflector** (installed by `vm-install.sh`) auto-copies into
     each `inv-vcl01-pr{N}` namespace; the PR chart's `extraEnvFrom` loads it.

   For a chart-managed secret (no `existingSecret`), set
   `secrets.aiVisionAnthropicApiKey` instead.

> ⚠️ **Ordering / dependency.** Because all three preview envs pin
> `provider: anthropic`, the key must be in place **before** that config syncs,
> or those apiservers CrashLoop (fail-loud above) — including **every** PR
> preview. A fresh PR preview may CrashLoop briefly until reflector copies the
> Secret into its new namespace — the kubelet then restarts the pod, which
> re-reads the now-present Secret. Fill
> `anthropic.api_key` + run `apply-secrets.sh` first, or flip the
> ApplicationSet's `aivision.provider` to `none`/`mock` to defer.

### Verify

```bash
# the materialized Secret carries the key:
kubectl -n inv-vcl01-master get secret inventario-admin \
  -o jsonpath='{.data.INVENTARIO_RUN_AI_VISION_ANTHROPIC_API_KEY}' | base64 -d | head -c8

# a successful scan lands an audit row:
#   SELECT status, provider, model FROM commodity_scan_audits ORDER BY created_at DESC LIMIT 1;
#   -> ok | anthropic | claude-sonnet-4-6

# in a PR preview, confirm reflector copied the key into the per-PR namespace:
kubectl -n inv-vcl01-pr<N> get secret inventario-ai-vision
```

## Operability

- **Audit / rate limit**: every scan attempt writes a `commodity_scan_audits`
  row (status `ok` / `error` / `timeout` / `validation` / `rate_limited` /
  `disabled`). The per-user hourly limiter counts only rows that actually
  reached the provider. The table is RLS-isolated per tenant.
- **Cost control**: keep `AI_VISION_RATE_LIMIT_PER_HOUR` and
  `AI_VISION_MAX_PHOTOS`/`AI_VISION_MAX_PHOTO_BYTES` set in production. PR
  previews run the real provider too, so each preview scan bills the vendor —
  they're tailnet-gated and rate-limited, but flip `applicationset-pr.yaml`'s
  `aivision.provider` to `mock` if you'd rather previews not spend.
- **Preview key delivery (#1976)**: per-PR namespaces are dynamic, so the key
  reaches them via emberstack/reflector copying the `inventario-ai-vision`
  Secret (AI keys only — never the admin/JWT material) into each
  `inv-vcl01-pr{N}` namespace. The reflector is a hard dependency for preview
  scans: if it's down, new previews CrashLoop until the Secret is present.
- **Security**: the providers never log the API key or the auth header. The key
  lives only in the sops bundle and the cluster Secrets it's materialized into
  (`inventario-admin` + the reflected `inventario-ai-vision`) — never in git,
  the ConfigMap, or logs.
