# OpenAPI / Swagger

The backend's HTTP surface is described by an OpenAPI 2.0 spec generated from
[swag](https://github.com/swaggo/swag) annotations on the handler functions.
Those annotations are the source of truth. The generated artifacts live at:

- `go/docs/swagger.yaml` — generated, human-readable spec checked into the
  repo.
- `go/docs/swagger.json` — generated JSON form of the same spec, consumed by
  the React frontend's TypeScript codegen (`make codegen-frontend-react`).
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
3. Run `make codegen-frontend-react` to regenerate the TypeScript types in
   `frontend-react/src/types/api.d.ts`. Run this whenever step 2 changes
   `go/docs/swagger.json` — including documentation-only annotation changes,
   since the generated `.d.ts` carries JSDoc derived from each operation's
   `@Summary` / `@Description`.
4. Commit `go/docs/*` and `frontend-react/src/types/api.d.ts` together in
   the same PR as the handler change.

## CI gates

| Workflow | Job | What it catches |
| --- | --- | --- |
| `go-swagger-docs.yml` | Check Swagger Docs Sync | `make swagger` produces a non-empty diff against `go/docs/` — i.e. annotations and committed spec disagree. |
| `frontend-react-codegen.yml` | codegen-check | `npm run codegen:check` produces a non-empty diff against `frontend-react/src/types/api.d.ts` — i.e. the generated TS types are stale relative to `swagger.json`. |

Both gates run on every push and pull request. They fail fast if the spec or
generated types drift from the source annotations.

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
make codegen-frontend-react
git status -- frontend-react/src/types/
```

## What's NOT yet checked

The current drift gate verifies that the spec matches the annotations in code,
and that the FE types match the spec. It does **not** verify that every
registered chi route has a matching swagger operation — i.e. an endpoint can
exist in code with no annotation at all and the gate will pass. That coverage
check (and the cleanup of currently-undocumented group-scoped routes under
`/g/{groupSlug}/...`) is tracked separately under epic #1397.
