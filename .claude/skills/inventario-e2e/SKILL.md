---
name: inventario-e2e
description: Run, reproduce, and debug Playwright e2e tests for Inventario locally. Use this skill when the user asks to "run e2e tests", "reproduce a failing playwright test", "debug a flaky e2e", or when "PR has failing E2E checks" — anything where the answer involves `npx playwright test`, the `e2e/` directory, the docker-compose e2e stack, or Playwright artifacts (test-results, traces, reports). Skip for unit tests (`make test`, vitest), Go tests (`go test`), or anything that doesn't touch `e2e/`.
---

# Inventario e2e — local run, reproduce, debug

CI takes ~25 minutes. **Reproducing a failing e2e locally takes ~15 seconds** once the stack is up, so always reproduce locally before pushing a fix.

## Pick a run mode

There are three. Pick by what you already have running on `:3333` and `:5173`.

| Mode | Backend on `:3333` | Frontend on `:5173` | When to use |
| ---- | ------------------ | ------------------- | ----------- |
| **A. Dev mode** (`npm run stack` from `e2e/`) | `go run` started by setup script | Vite started by setup script | Clean machine, no other inventario running. Setup script handles everything. |
| **B. Pre-built mode** (`USE_PREBUILT=true`) | Docker container serves embedded SPA on `:3333` | none — Playwright hits `:3333` directly | Reproducing CI exactly, or testing the production-style embedded build. |
| **C. Worktree-against-host hybrid** | Already running natively (the user's main dev stack) | `npm run dev` started by hand from this worktree's `frontend/` | **Most common for Claude Code worktrees.** You're testing UI changes from a worktree without touching the running backend. |

### Mode A: dev mode (clean start)

```bash
cd e2e && npm run stack          # one terminal — starts go run + vite + seeds db
cd e2e && npx playwright test --project=chromium   # second terminal
```

`npm run stack` is `START_STACK=true tsx setup/run-stack.ts`. It runs `go run -tags with_frontend ./cmd/inventario/... run` in `go/` and `npm run dev` in `frontend/`. Auth + global rate limits are disabled by default (the script sets `INVENTARIO_RUN_AUTH_RATE_LIMIT_DISABLED=true` and `INVENTARIO_RUN_GLOBAL_RATE_LIMIT_DISABLED=true`).

### Mode B: pre-built (reproduces CI)

```bash
# project root
INVENTARIO_IMAGE=inventario-inventario:latest \
  docker compose -f docker-compose.yaml -f docker-compose.e2e.yaml \
  up -d --wait --no-build inventario

# e2e/
USE_PREBUILT=true npx playwright test --project=chromium tests/<spec>.spec.ts

# tear down when done
docker compose -f docker-compose.yaml -f docker-compose.e2e.yaml down -v
```

`docker-compose.e2e.yaml` disables the auth + global rate limits (without it, parallel `POST /register` calls trip 429 after the first few).

`USE_PREBUILT=true` flips `e2e/setup/urls.ts` so `BASE_URL` becomes `http://localhost:3333` (the embedded SPA) and the setup script just waits for the externally-managed stack instead of starting one.

### Mode C: hybrid (worktree against host backend)

The user already has a native backend on `:3333`. You're in a worktree and want to test this worktree's frontend changes against that backend.

```bash
# in worktree, terminal 1: start vite from THIS worktree's frontend
cd frontend && npm run dev    # Vite on :5173, proxies /api → :3333

# in worktree, terminal 2: run tests
cd e2e && npx playwright test --project=chromium tests/<spec>.spec.ts
```

Vite's `/api` proxy (configured in `frontend/vite.config.mjs`) sends every API call to `http://localhost:3333`. Playwright hits Vite at `:5173`, so it picks up the worktree's UI bundle while sharing the running backend. **Do not start `npm run stack`** — it would race the user's backend on `:3333`.

This is how PR #1353's CI debug saga finally reproduced: 9 failed CI cycles → 1 local run via this mode → fix found in 10 minutes.

## Common invocations

```bash
# single test, single browser (fastest iteration)
cd e2e && npx playwright test --project=chromium tests/file-uploads.spec.ts

# specific test by title
npx playwright test --project=chromium -g "should upload a file"

# headed (watch the browser)
npx playwright test --project=chromium --headed tests/<spec>.spec.ts

# debug (Playwright Inspector, step through)
npx playwright test --project=chromium --debug tests/<spec>.spec.ts

# UI mode (interactive picker + time-travel debugger)
npm run ui

# full suite, single browser
npx playwright test --project=chromium

# all three browsers (matches CI scope)
npx playwright test    # chromium, firefox, webkit
```

## Health check before launching Playwright

Run this. If anything fails, fix it before invoking Playwright — a stack that's not ready will surface as confusing test failures, not a clear "stack down" error.

```bash
# backend ready (returns 200 only when migrations + seed are done)
curl -sf http://localhost:3333/readyz && echo "backend ok" || echo "backend NOT ready"

# vite responding (skip in pre-built mode — there is no vite)
curl -sf http://localhost:5173/ > /dev/null && echo "vite ok" || echo "vite NOT ready"

# admin login works (proves seed ran + auth wired)
curl -sX POST -H 'Content-Type: application/json' \
  -d '{"email":"admin@test-org.com","password":"testpassword123"}' \
  http://localhost:3333/api/v1/auth/login | jq -r .access_token | head -c 20
echo
```

If `/readyz` 200 but login returns 401, the DB exists but wasn't seeded — re-run seeding (Mode A: re-`npm run stack`; Mode B: `docker compose down -v` and `up` again with `SEED_DATABASE=true`).

## Test credentials

From `e2e/tests/includes/auth.ts`:

- `admin@test-org.com` / `testpassword123` — tenant `test-org`. Created by `/api/v1/seed`.
- `user2@test-org.com` / `testpassword123` — used by `user-isolation.spec.ts`. **Not** auto-created in dev mode (the seed fast-path skips it when an admin email is supplied). For the docker-compose path, CI provisions it via `inventario users create`. To provision locally:

```bash
# dev mode (Mode A/C, native binary)
go/cmd/inventario/... users create --email=user2@test-org.com --password=testpassword123 --name="Test User 2" --tenant=test-org --no-interactive

# pre-built mode (Mode B, container)
docker compose -f docker-compose.yaml -f docker-compose.e2e.yaml \
  run --rm --no-deps inventario \
  users create --email=user2@test-org.com --password=testpassword123 \
    --name="Test User 2" --tenant=test-org --no-interactive
```

`user2` also needs a default group for some user-isolation tests; if the test fails on a `/no-group` redirect, see `.github/workflows/e2e-tests.yml` "Create default group for user2" for the exact `POST /api/v1/groups` call.

## When a port is already occupied

E2E tests need three ports — the right move depends on **which** is busy and **why**.

### `:3333` (backend) busy

Find what's holding it:

```bash
lsof -nP -iTCP:3333 -sTCP:LISTEN
```

- **It's the user's main inventario** (most common in Claude Code worktrees) → don't kill it. Use **Mode C** (hybrid). Run only `npm run dev` from this worktree's `frontend/`, skip `npm run stack`. Tests will exercise this worktree's UI against the user's backend.
- **It's a stale `go run`** from a prior session you started → kill it: `kill <pid>`. Then start cleanly. (`go run` spawns a child binary; killing the parent may leave the child — `pkill -f 'cmd/inventario'` is the blunt instrument.)
- **It's a stale docker-compose stack** → `docker compose -f docker-compose.yaml -f docker-compose.e2e.yaml down -v`. The `-v` removes the postgres volume; without it, you keep stale data and the next seed will be a no-op.
- **You actually want to override** → set `E2E_BACKEND_URL=http://localhost:<other-port>` and (if pre-built) `E2E_BASE_URL` to match. The setup script and tests both honor these.

### `:5173` (vite) busy

```bash
lsof -nP -iTCP:5173 -sTCP:LISTEN
```

Vite without `--strictPort` will **silently pick `:5174`** if `:5173` is taken — but Playwright's `BASE_URL` is hard-coded to `:5173`, so every test will load the wrong app and fail with confusing 404s or auth issues. Two fixes:

- **Kill the other vite** (usually a forgotten `npm run dev` in another worktree): `kill <pid>`. This is the right answer 90% of the time — there's only one `frontend/` UI to test.
- **Point Playwright at the new port**: `E2E_BASE_URL=http://localhost:5174 npx playwright test ...` (and start your vite explicitly there: `npm run dev -- --port 5174 --strictPort`).

Don't try to share one Vite across two worktrees — the bundle is built from one worktree's source, so the "other" worktree's UI changes won't show up.

### `:8025` (Mailpit) busy

Only matters for `mailpit-email.spec.ts`. If something else is on `:8025`, the spec's reachability probe will succeed but the API shape will be wrong → confusing failures. Kill it, or set `MAILPIT_URL=http://localhost:<other>` to point at the real Mailpit.

If you have no Mailpit at all (dev mode without docker), `mailpit-email.spec.ts` self-skips cleanly — that's expected, not a failure.

### Generic recipe

```bash
PORT=3333    # or 5173, 8025
lsof -nP -iTCP:$PORT -sTCP:LISTEN
# decide: is this the user's intended process? if yes → adapt (use a different mode / override URL).
# if no → kill <pid>.
```

**Never** `kill -9` blindly. Always identify the process first — killing the user's main backend mid-task is exactly the kind of "destructive shortcut" that loses work.

## Failure artifacts

Playwright writes to `e2e/test-results/<test-id>/` on failure:

- `test-failed-*.png` — read this first. Often shows the bug at a glance (wrong page, modal closed, empty list).
- `video.webm` — full run, ~10-30s. Open with any video player.
- `error-context.md` — DOM snapshot at failure point.
- `trace.zip` — step-by-step DOM + network + console. Open with:

```bash
cd e2e && npx playwright show-trace test-results/<dir>/trace.zip
```

The trace viewer is the best tool for "test passed locally, failed in CI" — load the CI trace and the local trace side by side.

The HTML report (after a run):

```bash
cd e2e && npm run report   # opens playwright-report/ in a browser
```

For CI failures, download the report from the failed run:

```bash
gh run download <run-id> --repo denisvmedia/inventario --name playwright-report-<browser>
# unzips into ./playwright-report — view via: cd e2e && npx playwright show-report ../playwright-report
```

## CI gotchas worth surfacing

These are the patterns that bite repeatedly. Memorize them.

### 1. Warmup-orphan rows

`.github/workflows/e2e-tests.yml` runs `playwright test --grep "fast-fail debug"` first as a warmup, **then** the full suite — both against the same backend DB. Tests that share `describe`-scoped constants (`testLocation`, `testArea`, `testCommodity`) with the warmup test create same-named rows twice. Name-based locators like `.location-card:has-text("X").first()` then bind to the stale warmup clone, not the test's own creation, causing `DELETE 422 ("contains areas")`.

The trap also bites because the locations list sorts DESC by `created_at`, so `.first()` picks the newest *orphan* and `.last()` picks the oldest — neither is reliably the row the current test just created.

**Fix patterns:** capture the entity ID from the create POST response and use `[data-location-id="<id>"]` (or equivalent attribute) to disambiguate; or give each `test()` its own `Date.now()`-suffixed name rather than sharing describe-level constants. PR #1275 is the canonical fix — `LocationListView.vue` got `:data-location-id="location.id"`, `createLocation` returns the ID from the POST response, and `deleteLocation` accepts it.

### 2. Rate-limit hangs

E2E is high-throughput and parallel. With production rate limits on, parallel `POST /register` and `POST /auth/login` will start returning 429 after a handful of tests, and the auth helper used to silently wait on a redirect that never came. Both `INVENTARIO_RUN_AUTH_RATE_LIMIT_DISABLED=true` **and** `INVENTARIO_RUN_GLOBAL_RATE_LIMIT_DISABLED=true` must be in effect.

These are set automatically by:
- `e2e/setup/setup-stack.ts` (dev mode)
- `docker-compose.e2e.yaml` (pre-built mode)
- `.github/workflows/e2e-tests.yml` env block (CI macOS lane)

If you're running the backend by hand with neither, **set them yourself**.

### 3. Mailpit only when compose stack is up

`mailpit-email.spec.ts` probes `MAILPIT_URL` (default `http://localhost:8025`) in `beforeAll`. Reachable → runs; unreachable → all tests `test.skip()` cleanly. Mailpit only comes up under docker-compose (its host port `8025:8025` mapping plus `inventario`'s transitive `depends_on` chain through `mailpit-sidecar`'s healthcheck); the dev-mode `go run` path uses a stub email provider, so the spec skips. That's expected.

Parallel-safety note for any new Mailpit test: do **not** call `DELETE /api/v1/messages` (it's a global wipe and races sibling workers). Use a per-test `freshEmail` recipient (`Date.now()` + random) and filter `GET /api/v1/messages` by `To`.

### 4. Webkit-on-macOS lane has no docker

The CI macOS lane downloads the darwin/arm64 binary, runs it directly without docker, and uses local Postgres. No Mailpit, no init-data container. Locally on macOS this matches Mode A or C; **don't** try to docker-compose webkit failures unless you also have docker on macOS.

### 5. CI flake patterns

Common surface symptoms and root causes:

- `DELETE … 422 contains areas` → warmup orphan; switch to ID-based locators.
- Test silently passing because of a conditional `test.skip()` on a fragile "is this UI present?" check → audit the skip; use selectors that match the actual rendered DOM (`#description`, `.p-select[id="type"]`, `data-testid` attrs).
- `waitForResponse` predicate timing out → wrong status filter; the request happened but didn't return what you expected. Loosen the predicate, log the actual status.
- "Test passes locally, fails in CI" → almost always one of: warmup orphan (rerun the failing test alone, then rerun after a `--grep "fast-fail"` to reproduce), CI's single-worker concurrency masking a parallel-safety bug, or a `data-` attribute that exists in dev but not in the production embedded build (rare; happened once with `data-testid` stripped by a misconfigured Vue plugin).

## Out of scope for this skill

- Authoring new tests / fixtures.
- Modifying `.github/workflows/e2e-tests.yml`.
- User-facing docs in `e2e/README.md` (that's for humans; this skill is for the agent).
