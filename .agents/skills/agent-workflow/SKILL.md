---
name: agent-workflow
description: How to work in the Inventario repository on few tokens and little wall time. Load it at the start of any task in this repo that reads or changes code, tests, docs, CI, Helm or infra, or that reviews a change, before the first file read. It sets the reading rules, the scoped lint and test scripts to use while iterating, the full check chain to run once before a push, the local review rule for backend and infra changes, the push-and-stop rule, and how a coordinator runs several agents.
---

# Agent workflow

Agents in this repository spent most of their context on whole-file reads and most of their wall time on repeated full lint and test runs. These rules cut both. Repo rules stay in [AGENTS.md](../../../AGENTS.md); this skill only says how to work.

## Start

1. Work in a worktree under `.claude/worktrees/<name>/` branched off fresh `origin/master` (AGENTS.md, "Git Worktrees").
2. Run `.agents/skills/agent-workflow/scripts/worktree-setup.sh` from the root of the new worktree. It refuses on a nearly full disk, runs `npm ci` in `frontend/` when `node_modules` does not match the lock, and fills the Go module cache. Add `--e2e` when the task runs Playwright.
3. Read `devdocs/code-map.md` before exploring the tree. It names the canonical file for each pattern, the regeneration commands and the files to read only in ranges.

Do not repair the environment by hand. Rerun the setup script, and report what it cannot fix. Never install into or reset the main checkout, never delete the shared Playwright browser cache or another agent's worktree, and never run `git stash` (the stash stack is shared by every worktree).

## Reading

- Search before reading: `git grep -n <symbol> -- <path>`, then Read with `offset` and `limit` around the hit (40 to 120 lines).
- Never read a whole file over about 300 lines. Outline it first: `grep -n '^func \|^type ' file.go`, `grep -n '^export \|^function ' file.tsx`.
- Never Read generated files; grep them: `frontend/src/types/api.d.ts`, `go/docs/swagger.*`, `go/schema/schema.hcl`, `go/schema/migrations/_sqldata/`.
- Review a branch with `git diff --stat <base>...HEAD` first, then `git diff <base>...HEAD -- <path>` for only the files you need.
- Cap every output that can grow: `| head -50`, `| tail -30`, or a grep. Run `go test` without `-v` unless you are reading one test's log.
- Do not reread a file you just edited. The edit tool fails when an edit does not apply.
- Do not rerun a command to see its output again; save long output to a file under your scratch directory and grep it.

## Checks while iterating

Use the scoped scripts. They act on the checkout you run them in; paths here are relative to its root. Both compare against `origin/master` by default (`--base <ref>` to change) and count committed, staged, unstaged and untracked changes.

```sh
.agents/skills/agent-workflow/scripts/lint-changed.sh
.agents/skills/agent-workflow/scripts/test-changed.sh
```

- `lint-changed.sh` runs nolintguard, qtlint and golangci-lint on the touched Go packages; eslint and `prettier --check` on changed frontend files, and `tsc -b` only when a `.ts` or `.tsx` file changed; markdownlint-cli2 on changed Markdown; actionlint on changed workflows. The first `tsc -b` in a fresh worktree takes a few minutes; later runs are incremental.
- `test-changed.sh` runs the postgres registry tests only when `go/registry/postgres/` or `go/registry/registrytest/` changed. It takes `--race`, `--pg` (postgres tests regardless), `--list` (print what would run) and `-- <go test args>`, for example `-- -run TestSellerContract`.
- For a single test, run it directly: `cd go && go test ./services -run TestTrashPurge`. `go test` caches passing packages; do not add `-count=1` unless you need a real rerun.
- Do not run `make lint-go`, `eslint .`, `npm run typecheck`, `go test ./...` or `make test` after each edit. The full runs cost minutes each and are for the end.

## Before the push

Run the full chain once, when the change is complete:

1. Go changed: `make lint-go`. It runs three linters, nolintguard, qtlint and golangci-lint; golangci-lint alone reports a false clean. Then the tests of the touched areas (`test-changed.sh`, or `cd go && go test ./<area>/...`).
2. `go/registry/` changed: the postgres registry tests too, `cd go && go test ./registry/postgres/...`. With `POSTGRES_TEST_DSN` unset, `go/internal/pgtest` starts an embedded server from the cache in `~/.embedded-postgres-go`.
3. Frontend changed: `cd frontend && npx eslint . && npm run typecheck && npm run format:check`, then `npx vitest related --run <changed files>`.
4. Markdown changed: `markdownlint-cli2` from the repo root with no arguments (the CI invocation). Docs site changed: the build in AGENTS.md, "User-facing changes & documentation".
5. Generated artifacts regenerated and committed: swagger and frontend types, migrations with `ptah.sum`, `go/schema/schema.hcl`. The commands are in the code map.

## Local review

Backend and infra changes get a review from a fresh reviewer agent before the push. That covers Go, migrations, auth, middleware, registries, Helm, `infra/`, `k8s/`, `deploy/` and CI workflows.

- Go and migrations: `inventario-code-reviewer`. Add `security-reviewer` when auth, middleware, file handling or admin endpoints changed.
- Helm, Kubernetes and infra: `go-kubernetes-reviewer`.
- Give the reviewer the base ref, the branch and the worktree path, not your conclusions. Fix the real findings, rerun the scoped checks, then push.
- If you cannot start agents, stop before the push and hand back to the coordinator: branch, worktree, what needs review.
- Frontend-only and docs-only changes skip this step.

## Push and CI

- Push once, after the full chain and the review. Each push builds an image and takes a slot in the shared CI queue.
- Do not poll GitHub, wait for CI, or loop on `gh pr checks`. Report the branch, the PR link and the checks you ran, then stop. The coordinator or the owner watches CI.

## Rules that live in AGENTS.md

Point to them; do not restate or work around them:

- migrations are generated from model annotations, never hand-written ("Database Operations");
- `design-mocks/` is read-only ("Design mock");
- worktrees go under `.claude/worktrees/` ("Git Worktrees");
- tests use `bcrypt.MinCost` ("Security");
- user-facing changes update `docs/site/` ("User-facing changes & documentation");
- frontend work follows the `frontend-work` skill and `devdocs/frontend/`.

## Coordinating several agents

- One agent per task, each with a fresh context and its own worktree. Do not hand an unrelated task to an agent that already ran for hours.
- Write a narrow brief: the goal, the files and line ranges to start from, what is already ruled out, the checks to run, and where to stop (for example "commit, do not push").
- Prefer the specialized agents over general-purpose ones: `inventario-backend` for Go work, `inventario-code-reviewer` and `security-reviewer` for review, `go-kubernetes-reviewer` for Helm, Kubernetes and infra.
- Ask for a short report: files changed, commands run with their results, open questions. No file dumps.
- One CI watcher for all open pull requests, not one per agent. Agents push once, report and stop.
