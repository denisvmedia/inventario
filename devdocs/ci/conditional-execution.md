# CI conditional execution

This document explains how Inventario's CI workflows decide which jobs run on
a pull request and which are skipped. The rules and rationale live here; the
machine-readable form lives in `.github/filters.yml`.

## Why this exists

Most of our PRs touch a single ecosystem. A frontend-only PR doesn't need to
re-run the Postgres bootstrap suite; a backend-only PR doesn't need to
rebuild the React bundle, run Lighthouse, or spin up Firefox and Webkit
Playwright lanes; a README fix doesn't need any of it. Running the full
matrix every time costs developer wait, CI minutes, and ghcr storage churn
without adding signal.

Pushes to `master` and pushes of `v*` tags continue to run the full pipeline
unconditionally — they are the integration point where maximum confidence
matters more than wall-time.

## How it works

1. `.github/filters.yml` defines named filters (`go`, `frontend`, `e2e`,
   `image_inputs`, etc.). Each filter is a list of path globs.
2. Each PR-event workflow has a `changes` job that runs `dorny/paths-filter@v3`
   against that file and exposes the result as job outputs.
3. Every existing job in the workflow gains a `needs: changes` dependency and
   an `if:` predicate of the form:

   ```yaml
   if: >-
     github.event_name != 'pull_request' ||
     needs.changes.outputs.<ecosystem> == 'true' ||
     needs.changes.outputs.ci == 'true'
   ```

   The first clause keeps `push` (to `master`) and `push` of tags running
   unconditionally. The third clause forces the full pipeline back on whenever
   the PR touches `.github/workflows/`, `.github/actions/`, or this filter
   file itself — when CI is being edited we want every check to confirm the
   edit didn't break it.
4. A job that evaluates to `false` in its `if:` is reported as `success`
   (skipped) by GitHub. It does not block merges.

## Filter quick-reference

| Filter             | Triggers when …                                                                                                                                          |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `go`               | Any path under `go/**`, plus `cmd/**`, `.golangci.yml`, `Makefile`, `.goreleaser.yaml`                                                                   |
| `go_swagger`       | Any path under `go/**` or `Makefile`. Intentionally as broad as `go` plus `Makefile` — `swag init` walks transitively imported packages, so narrowing risks silently missing drift cases. |
| `frontend`         | Any `frontend/**`, `Makefile`                                                                                                                            |
| `frontend_codegen` | The codegen script, `frontend/package*.json`, `frontend/src/types/api.d.ts`, plus any Go file or `Makefile` (Go drives `swagger.json`, the codegen input) |
| `frontend_i18n`    | `frontend/src/**`, `frontend/i18next.config.ts`, `frontend/scripts/i18n-check.mjs`, `frontend/package*.json`                                            |
| `e2e`              | `e2e/**` or `docker-compose.e2e.yaml`                                                                                                                    |
| `image_inputs`     | Anything that makes a fresh Docker image necessary or that a downstream consumer (e2e-tests, kind-smoke-test) needs an image for: `Dockerfile`, `.dockerignore`, `docker-compose*.yaml`, `go/**`, `frontend/**`, `scripts/**`, `init-scripts/**`, `Makefile`, `.goreleaser.yaml`, `e2e/**`, `k8s/dev/**` |
| `ci`               | Anything under `.github/workflows/`, `.github/actions/`, or `.github/filters.yml`                                                                        |

Markdown-only diffs at the repository root (e.g. `README.md`) match none of
these filters, so all PR-gated jobs short-circuit. A markdown change *inside*
a tracked subtree (`go/README.md`, `frontend/CONTRIBUTING.md`) does match the
ecosystem filter and triggers that ecosystem's checks — a deliberate tradeoff
to avoid the fragility of composing negation patterns under `dorny/paths-filter`'s
default semantics.

## Decision table — representative PR scenarios

| PR diff                              | Jobs that run                                                                                                                                                                              |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Root markdown only (`README.md`)     | None of the 13 PR-event-gated workflows; existing narrowly-filtered ones (helm-lint, cli-integration-test, smoke tests, release dry-run, renovate-config) also skip                         |
| Backend only (`go/**`)               | `go-test`, `go-test-postgres`, `go-lint`, `go-swagger-docs`, `frontend-codegen` (Go drives swagger.json), `frontend-embed-smoke-test`, `docker.yml`, `e2e-tests-linux[chromium]`; Firefox + macOS-webkit + the other frontend-* workflows skip |
| Frontend code only (e.g. `frontend/src/App.tsx`) | `frontend-test`, `frontend-lint`, `frontend-i18n`, `frontend-lhci`, `frontend-size`, `frontend-embed-smoke-test`, `docker.yml`, full `e2e-tests` matrix. `frontend-codegen` skips because no codegen driver changed |
| Frontend codegen driver only (`frontend/scripts/codegen.mjs`) | `frontend-codegen`, plus (via `frontend/**`) `frontend-test`, `frontend-lint`, `frontend-lhci`, `frontend-size`, `frontend-embed-smoke-test`, `docker.yml`, and the full `e2e-tests` matrix. `frontend-i18n` skips because no i18n input changed |
| E2E only (`e2e/**`)                  | `docker.yml` (`e2e/**` is in `image_inputs` so a fresh image is built), full `e2e-tests` matrix                                                                                            |
| Helm only (`helm/**`)                | `helm-lint`                                                                                                                                                                                |
| Dockerfile / compose / scripts / init-scripts | `docker.yml`, `docker-compose-smoke-test`, `kind-smoke-test`, e2e-tests-linux[chromium] (webkit + firefox skip unless the change is also frontend/e2e)                          |
| `k8s/dev/**` only                    | `docker.yml`, `kind-smoke-test`, `e2e-tests-linux[chromium]`                                                                                                                              |
| `renovate.json` only                 | `renovate-config-validation`                                                                                                                                                              |
| Any change to `.github/workflows/**` | Full pipeline (the `ci` filter forces it)                                                                                                                                                  |
| `master` push or `v*` tag            | Full pipeline (PR-only gating, always-on integration)                                                                                                                                      |

## e2e browser-matrix scoping

`e2e-tests.yml` runs three Playwright browsers in the worst case: Chromium
and Firefox on Linux, Webkit on macOS. Each browser is its own ~30-minute
critical-path job. The `changes` job emits two extra outputs to scope the
matrix:

- `linux_browsers` — a JSON array. `["chromium"]` for backend-only PRs;
  `["chromium","firefox"]` for anything that could plausibly affect rendering
  (frontend / e2e / image_inputs ∩ frontend / ci / push).
- `run_webkit` — boolean; same logic as the firefox toggle.

| PR diff                                 | Linux runs              | macOS webkit runs |
| --------------------------------------- | ----------------------- | ----------------- |
| Backend (`go`) only                     | chromium                | no                |
| Frontend / e2e / image_inputs           | chromium + firefox      | yes               |
| `ci` / push to master / `v*` tag        | chromium + firefox      | yes               |

## Workflows not modified

| Workflow                          | Reason                                                                       |
| --------------------------------- | ---------------------------------------------------------------------------- |
| `cli-integration-test.yml`        | Already has `paths: [go/**, .github/workflows/cli-integration-test.yml]`     |
| `docker-compose-smoke-test.yml`   | Already has a conservative path filter                                       |
| `kind-smoke-test.yml`             | Already has a conservative path filter                                       |
| `helm-lint.yml`                   | Already gated on `helm/**`                                                   |
| `renovate-config-validation.yml`  | Already gated on `renovate.json`                                             |
| `release.yml`                     | Already gated on a narrow PR path filter (PR dry-run); tag push unaffected   |
| `dependabot-automerge.yml`        | Only fires on PRs by `dependabot[bot]` / `renovate[bot]`                     |
| `copilot-setup-steps.yml`         | `workflow_dispatch` only                                                     |
| `_wait-for-docker-image.yml`      | `workflow_call` only (reusable)                                              |

## Adding a new conditional check

1. Add the filter to `.github/filters.yml` if no existing filter matches.
2. In the workflow, add a `changes` job using the established pattern (see
   `go-test.yml` for the canonical minimal example).
3. Add `needs: changes` and the `if:` predicate to each real job.
4. Always include the `ci` clause in the OR so workflow-config changes force
   the check back on.
5. Don't add workflow-level `paths:` filters — they don't compose well with
   `paths-filter`'s base-detection logic and bypass the `ci` escape hatch.

## What we did not do (deliberately)

- **No structural refactor** of the frontend workflows. `npm ci` and
  `npm run build` are still repeated across four workflows; deduplication
  is tracked separately.
- **No new aggregator/required-check job.** None of these check names are
  currently required in branch protection, so a skipped job's implicit
  `success` is sufficient.
- **No `merge_group` trigger.** Merge-queue compatibility is a separate
  feature; this PR keeps the existing trigger surface intact.
