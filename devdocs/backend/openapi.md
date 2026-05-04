# OpenAPI / Swagger

The backend's HTTP surface is described by an OpenAPI 2.0 spec generated from
[swag](https://github.com/swaggo/swag) annotations on the handler functions.
Those annotations are the source of truth. The generated artifacts live at:

- `go/docs/swagger.yaml` — generated, human-readable spec checked into the
  repo.
- `go/docs/swagger.json` — generated JSON form of the same spec, consumed by
  the React frontend's TypeScript codegen (`make codegen-frontend`).
- `go/docs/docs.go` — generated Go bindings registered by `apiserver.go` so
  the `/swagger/*` UI route can serve the spec at runtime.

All three files are regenerated together by a single command from the
annotations and must stay in sync with them; CI fails any PR where they don't.

## Adding or changing an endpoint

1. Add or update the swag annotation block above the handler function. See
   any existing handler in `go/apiserver/*.go` for the conventions (the
   `commodities.go` file is a good reference — it covers list / detail /
   create / update / delete / bulk shapes).
2. Run `make swagger` from the repository root. This calls
   `go tool swag init --output docs` inside `go/`. The tool walks the package,
   parses annotations, and rewrites all three files in `go/docs/`.
3. Run `make codegen-frontend` to regenerate the TypeScript types in
   `frontend/src/types/api.d.ts`. Run this whenever step 2 changes
   `go/docs/swagger.json` — including documentation-only annotation changes,
   since the generated `.d.ts` carries JSDoc derived from each operation's
   `@Summary` / `@Description`.
4. Commit `go/docs/*` and `frontend/src/types/api.d.ts` together in
   the same PR as the handler change.

## CI gates

| Workflow | Job | What it catches |
| --- | --- | --- |
| `go-swagger-docs.yml` | Check Swagger Docs Sync | `make swagger` produces a non-empty diff against `go/docs/` — i.e. annotations and committed spec disagree. |
| `go-swagger-docs.yml` | Check route coverage | `TestSwaggerRouteCoverage` walks the live chi router and the committed `swagger.json`. Fails if any registered `/api/v1/...` route is undocumented, or if any documented operation references a path that's no longer registered. |
| `frontend-codegen.yml` | codegen-check | `npm run codegen:check` produces a non-empty diff against `frontend/src/types/api.d.ts` — i.e. the generated TS types are stale relative to `swagger.json`. |

All three gates run on every push and pull request. They fail fast if the spec
or generated types drift from the source annotations, or if a new route lands
without an annotation.

## Reproducing a CI failure locally

If `Check Swagger Docs Sync` fails on your PR:

```bash
make swagger
git status -- go/docs/
git diff -- go/docs/
```

The diff shows what the workflow saw. Commit it.

If `codegen-check` fails:

```bash
make codegen-frontend
git status -- frontend/src/types/
```

## Route coverage gate

`go/apiserver/swagger_route_coverage_test.go` walks the chi router that
`apiserver.APIServer(...)` returns and compares it to the operations declared
in `go/docs/swagger.json`. It enforces a strict bidirectional invariant:

- Every registered `/api/v1/...` route must have a matching `(method, path)`
  in the spec.
- Every documented operation must point at a path that is actually registered.

The walker strips the `/api/v1` basePath, ignores the `/*` catch-all (the
frontend embed), and only inspects the standard CRUD verbs (GET / POST / PUT /
PATCH / DELETE) — anything else (HEAD, OPTIONS) doesn't need a swag block.

When you add a route, add the matching `@Router` annotation in the same PR; the
gate gives you a single failure listing the missing or stale entries so you
can correct them in one round-trip with `make swagger`.

### Group-scoped paths

Most data routes live under `/api/v1/g/{groupSlug}/...`. Their annotations
include the prefix in `@Router` and a `@Param groupSlug path string true
"Group slug"` block. The non-group-scoped surfaces — `/auth/...`,
`/system`, `/debug`, `/groups`, `/invites`, `/currencies`, `/seed`,
`/files/download/...`, `/register`, `/forgot-password`, `/reset-password`,
`/verify-email`, `/resend-verification` — keep their bare paths.
