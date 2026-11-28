# GitHub API request budget in CI

Every job in this repository that calls the GitHub REST API with
`secrets.GITHUB_TOKEN` (`github.token`) draws on one shared limit: **1,000
requests per hour per repository**
([rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)).
It is a single counter for all workflows, all pull requests and master. When
it runs out, every job that calls the API fails with
`HTTP 403: API rate limit exceeded for installation` until the hour resets.
That happened on 2026-11-27 (#3025).

This document lists what spends that budget, what one push costs, the load
the CI is sized for, and how the image build hands off to the lanes that test
it.

## What does not count

These do not use the REST `core` limit, so they are left out below:

- `actions/checkout` (git over HTTPS), `actions/cache`, and
  `actions/upload-artifact` / `actions/download-artifact` within one run
  (they use the run's own runtime token and artifact service).
- Pulls and pushes to `ghcr.io` (registry protocol).
- `GET /rate_limit`.
- `setup-go`, `setup-node` and `setup-helm` when the version is already in the
  runner's tool cache, or is downloaded from a fixed URL.

## Inventory

Counts are per invocation. "Measured" values come from the job logs of the
runs for PR #3050, head `72fdeda`, on 2026-11-27 at 21:23 UTC: a Go-only change
that built an image, with a short runner queue (the manifest was published
about 6 minutes after the push). That run used the workflows as they were
after #2985 and before #3058. "Estimated" values are read from the
action's behavior and were not logged.

| Consumer | Where | Trigger | Endpoint family | Requests per invocation | Per push |
| --- | --- | --- | --- | --- | --- |
| Changed-file detection (`dorny/paths-filter`) | `changes` job of 22 workflows (two of them behind a workflow-level `paths:` filter) | every pull request push | none: a `git fetch` of one commit and a local diff | 0 | 0 (through `GET /repos/{r}/pulls/{n}/files` it was 1 per 100 changed files: 21 for the measured diff) |
| Image waiter (`_wait-for-docker-image.yml`) | `helm-kind-smoke-test.yml` | pull requests touching the chart, previews or CI; master | `GET .../actions/workflows/docker.yml/runs`, `GET .../actions/runs/{id}/jobs` | 1 per poll while the run is queued, 2 while it runs; one poll a minute | 2·D (measured: 6 polls, 12 requests) |
| PR-close drain | `docker.yml` `cleanup` | pull request closed | `GET .../actions/runs?head_sha=` | 1 per minute until no run is left, at most 30 | 1–30 per close (measured: 1 on #3050, 16 on #3058) |
| PR-close reclaim | `docker.yml` `cleanup` | pull request closed | PR commits, owner lookup, package versions, deletes | 2 + 1 per 100 versions published since the PR was opened, plus one page + 1 per deleted version | 129 for the listing alone before it was bounded (12,845 versions, #3058); bounded: about 1 per hour the PR was open, logged on every close |
| SARIF upload (`github/codeql-action/upload-sarif`) | `docker.yml` `scan-image` | every image build | code scanning upload and processing status | estimated 3–5 | 3–5 |
| `actions/dependency-review-action` | `dependency-review.yml` | every pull request push | dependency graph compare | estimated 1–2 | 1–2 |
| `dependabot/fetch-metadata`, `gh pr merge`, `gh pr comment` | `dependabot-automerge.yml` | bot pull requests only | pulls, comments | estimated 3–6 | bot PRs only |
| Required-check lookup | `.github/scripts/required-checks-configured.sh` | bot pull requests only | rules, GraphQL | 1–2 REST + 1 GraphQL | bot PRs only |
| Preview comment (`marocchino/sticky-pull-request-comment`) | `pr-preview-status.yml` | pull requests with the preview label | issue comments | estimated 2 | labeled PRs only |
| Migration plan comment | `migration-plan.yml` | schema-changing pull requests | issue comments | estimated 2 | schema PRs only |
| GoReleaser, chart push | `release.yml` | `v*` tags | releases, assets | tens | tags only |

Before #3058, two more consumers ran on every image-building push. Both
are gone:

| Removed consumer | Requests per push |
| --- | --- |
| Image waiters of `e2e-tests.yml` and `kind-smoke-test.yml` | 2·D each (measured: 12 and 8) |
| Cross-run download of the darwin binary in the webkit lane | estimated 2–3 |

GraphQL has its own points budget and the measured workflows spend almost
none of it. No job in the measured runs hit secondary rate limits.

## Cost of one push

D is the producer delay: minutes from the start of a waiter to the published
manifest. It is about 6 on an idle fleet and has reached 20 on a busy one.

| | Waiters | Fixed (changed files, SARIF, dependency review, artifacts) | One push, D = 6 | One push, D = 20 |
| --- | --- | --- | --- | --- |
| Before #2985 (15 s polls, about 4 requests each, 3 waiters) | 48·D | about 31 | about 320 | about 990 |
| After #2985 (60 s polls, 1–2 requests, 3 waiters) | 3·D to 6·D | about 31 | about 65 (measured 32 + 23 + estimates) | 90–150 |
| After #3058, chart pull request | 2·D | about 27 | about 39 | about 67 |
| After #3058, any other pull request | 0 | about 27 | about 27 | about 27 |

The row before #2985 is a model from the description in #2985 and the failed
job in #3025: that waiter was refused on its first request at 16:25:43 UTC
with request ID `7828:A01CB:8B0D48:1BA6A0D:6B09AF07`. The measured runs did
not log `x-ratelimit-*` headers, so the counts above come from log lines (one
per poll, one per changed-file fetch), not from the shared counter. No loop
was run against the live limit to obtain them.

## Budget

The load CI is sized for: 12 open pull requests, each pushed twice an hour,
every push building an image, a producer delay of 20 minutes, 2 of the 12
touching the chart, and 6 closes an hour.

| | Requests per hour at that load | Share of 1,000 |
| --- | --- | --- |
| Before #2985 | about 24,000 | spent within minutes |
| After #2985 | about 2,200–3,600 plus closes | 2 to 4 times over |
| Lanes behind `needs: merge` (#3058), close listing every version | about 810 + 6 × about 150 = about 1,700 | 1.7 times over |
| Close listing bounded to the pull request's lifetime | about 810 + 6 × 10–30 = 870–990 | 87–99 % |
| Changed-file detection through git | about 310 + 6 × 10–30 = 370–490 | 37–49 % |

The target is to stay under **700 requests an hour** at that load, which
leaves 300 for reruns, bot pull requests, previews and the tag release.
Readiness coordination costs nothing except for the Helm kind lane.

A close cost about 150 requests before its listing was bounded: 129 to page
through all 12,845 versions of the `inventario` package (measured on the close
of #3058), up to 30 for the drain, and the deletes. No job sweeps the package
on a schedule, so that count only grows. The reclaim step now stops paging at
the first page whose versions are all older than the pull request, less an
hour of slack: a pull request's images are built by its own pull request
runs, so none of them is older than the pull request. The listing then costs
one page per 100 versions published while the pull request was open, plus
one. The package holds about four versions per `docker.yml` run (12,845 over
about 3,460 runs), so at the sizing load it grows by about a page an hour,
and a close costs about one request per hour the pull request was open. The
step logs the pages it read on every close; the 10–30 per close above is an
estimate until those logs replace it.

Changed-file detection diffs with git and makes no request; through the
pull request files API it cost 21 requests a push, 504 an hour at that load.
See [Conditional CI execution](conditional-execution.md#how-it-works).

## How the image build hands off to its tests

`docker.yml` builds the image and runs the lanes that pull it:

```text
changes ─┬─ build (amd64, arm64) ── merge ─┬─ E2E Tests (chromium, firefox)
         │                                 ├─ E2E Tests (oauth)
         │                                 ├─ Kind / Kind smoke test
         │                                 ├─ scan-image
         │                                 └─ pin-master-image (master)
         └─ darwin-binary ─────────────────┴─ E2E Tests (webkit)
```

- The lanes `need` the manifest job, so they start as soon as the image is
  published. They never wait for the scan, the ArgoCD pin or cleanup, and
  waiting costs no API requests however long the queue is.
- `merge` outputs the image reference, `sha-<7>` of the commit under test. The
  tag is immutable per commit, so a lane cannot test an earlier commit's image
  through a moved `pr-N` or `edge` pointer. Digest pinning was not adopted:
  the kind lane loads a single platform out of the index, and a digest of the
  index does not resolve inside the cluster.
- The webkit lane needs `darwin-binary` in the same run and downloads the
  binary without a run ID or token.
- `E2E Tests (chromium)` and `E2E Tests (oauth)` run on every pull request and
  decide in their first step: run the suite, report green because the diff
  builds no image (path filter or fork), or fail because the build or the
  manifest did not succeed. `scripts/test-e2e-image-gate.sh` runs that step
  for each case in CI Lint.
- The e2e lanes are jobs of `docker.yml` rather than a called workflow. A
  called workflow prefixes its check names with the caller's job name, and
  `E2E Tests (chromium)` is a required check that keeps its exact name. The
  kind smoke test is not required, so it stays a called workflow and its check
  reads `Kind / Kind smoke test`.
- `helm-kind-smoke-test.yml` still waits through `_wait-for-docker-image.yml`.
  It runs only for chart, preview and CI changes.

### Cancellation and ordering

- Pull request runs cancel each other per pull request, lanes included, so a
  superseded commit leaves no lane waiting. Closing a pull request cancels the
  run still in progress; a merged change is tested again by the master push.
- A started master run is never cancelled (#3031). Because the lanes belong
  to the run, the next master push waits for the previous push's e2e lanes as
  well as its build. During a burst of merges, master images are therefore
  published less often: a commit whose run is still pending when a newer push
  arrives is never built, and the newer run tests it.
- Tag pushes build and publish the release image but run no test lane; the
  tagged commit was tested on master.

### Alternatives that were not taken

- **A `workflow_run` consumer.** It fires when the whole `docker.yml` run
  completes, which brings back the wait for the scan tail (#2554). Its check
  runs attach to the default branch rather than to the pull request, so a
  required check would not show up there, and it runs with the base
  repository's token on fork pull requests.
- **A single shared waiter.** One poller instead of three would divide the
  cost by three, but the cost would still grow with the queue.
- **A GitHub App installation token.** It would raise the limit to 5,000 an
  hour shared with other repositories of the installation, adds a secret to
  manage, is unavailable on fork pull requests, and starts workflows on
  writes. With polling removed the measured demand fits under the
  `GITHUB_TOKEN` limit, so the built-in token stays.

## Follow-ups

1. **Changed-file detection through git.** Done: the `changes` jobs diff
   with git against the merge commit's first parent, as described in
   [Conditional CI execution](conditional-execution.md#how-it-works).
2. **Move the Helm kind lane into `docker.yml`** the same way, which retires
   `_wait-for-docker-image.yml` and its verdict test.
3. **Narrow the close drain** to runs that can still pull the image. With the
   lanes inside `docker.yml`, only the Helm kind lane is left outside it.
4. **Sweep old package versions.** Nothing deletes a version that a closed
   pull request left behind (commits that were force-pushed away, a close
   whose drain gave up), and the reclaim step deletes only tagged versions,
   so the untagged per-architecture and attestation manifests of every build
   stay. A scheduled job with its own budget would keep the package small;
   until then the bounded listing keeps a close cheap whatever the package
   size.
5. **Honor the limit's reset.** The waiter retries a failed call every 60
   seconds; reading `x-ratelimit-reset` and `Retry-After` would stop it
   spending retries before the reset, and a 401 or 404 should fail at once.

## Troubleshooting

`API rate limit exceeded for installation` in a job means the repository's
hour is spent; the job is not broken. Re-run it after the reset (at most an
hour). A developer's own `gh` login has a separate limit and says nothing
about this one. To see the token's state from inside CI, a step can run
`gh api rate_limit` with `GH_TOKEN: ${{ github.token }}`, which does not count
against the limit.

A re-run uses the workflow files of the commit it ran on. A pull request
picks up workflow changes merged to master on its next push, because pull
request runs use the merge of the branch with its base; an empty commit is
enough.

If `E2E Tests (chromium)` fails in its first step with "The image build ended
as ...", the build or manifest job of the same run failed or was cancelled;
fix or re-run that, not the e2e lane.
