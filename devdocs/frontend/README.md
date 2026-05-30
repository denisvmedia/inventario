# Frontend developer docs

Engineering docs for `frontend/` — the React 19 + Tailwind v4 + shadcn/ui
app that ships with Inventario. The canonical visual contract lives in
[`design-mocks/`](../../design-mocks/) at the repo root (a read-only mirror
of `github.com/denisvmedia/inventario-design`, synchronized by an external
tool — see [AGENTS.md](../../AGENTS.md) for the full read-only rule). This
tree translates that mock into the conventions and patterns the real
codebase uses.

## When to read what

| Doc | Read it when … |
| --- | --- |
| [coding-standards.md](coding-standards.md) | Naming, file structure, imports, formatting, console policy. |
| [components.md](components.md) | Deciding where a new component lives (`components/ui` vs `components/` vs `features/<x>/components`). |
| [styles-and-tokens.md](styles-and-tokens.md) | Adding a color, working with Tailwind v4 `@theme`, dark mode, density. |
| [forms.md](forms.md) | Building a form with react-hook-form + zod and surfacing server errors. |
| [icons.md](icons.md) | Picking an icon, sizing it, deciding `aria-hidden` vs `aria-label`. |
| [accessibility.md](accessibility.md) | Focus rings, label/htmlFor, modal a11y, contrast, reduced motion. |
| [data.md](data.md) | TanStack Query keys, mutations, optimistic updates, cache invalidation. |
| [routing.md](routing.md) | Adding a route, group context, route guards. |
| [auth.md](auth.md) | Auth state model, token/session storage, the 401-refresh interceptor, host-based tenancy, route-guard redirects. |
| [i18n.md](i18n.md) | Adding a translatable string, namespaces, locale fallback, `preservePatterns`. |
| [imports-and-bans.md](imports-and-bans.md) | Why a dependency is on the no-fly list (next-themes, @base-ui/react, FontAwesome, Bolt artifacts). |
| [testing.md](testing.md) | Vitest harness, MSW handler factories, jest-axe, coverage thresholds. |
| [perf.md](perf.md) | Bundle-size + Lighthouse gates and what to do when they trip. |
| [screenshots.md](screenshots.md) | Capturing local screenshots end-to-end (seed → server → script). |
| [migration-conventions.md](migration-conventions.md) | History/cutover notes — Vue→React rewrite (epic #1397). |
| [pr-checklist.md](pr-checklist.md) | Copy-paste PR checklist for FE changes. |
| [design-deviations.md](design-deviations.md) | Logging or reading every intentional divergence from `design-mocks/`. Read before designing a surface; append after landing one. |
| [design/README.md](design/README.md) | Design-direction brief — 23 docs. Read when working on a new surface or arguing about taste. |

## Stack at a glance

| Layer | Choice | Pinned in `frontend/package.json` |
| --- | --- | --- |
| Framework | React 19 + TypeScript (strict) + Vite 8 | `react`, `react-dom`, `vite` |
| Styling | Tailwind v4 (OKLCH tokens) + `tw-animate-css` | `tailwindcss`, `tw-animate-css` |
| Components | shadcn/ui (new-york / neutral) on Radix primitives via the `radix-ui` umbrella | `radix-ui` |
| Icons | `lucide-react` only | `lucide-react` |
| Forms | `react-hook-form` + `zod` (via `@hookform/resolvers/zod`) | `react-hook-form`, `zod`, `@hookform/resolvers` |
| Data | TanStack Query 5 + a tiny `lib/http.ts` wrapper | `@tanstack/react-query` |
| Routing | `react-router-dom` v7 (declarative `<Routes>`) | `react-router-dom` |
| i18n | `react-i18next` + lazy-loaded cs/ru namespaces | `i18next`, `react-i18next` |
| Toasts | `sonner` | `sonner` |
| Command palette | `cmdk` | `cmdk` |
| Tests | Vitest + RTL + jest-axe + MSW | `vitest`, `@testing-library/react`, `jest-axe`, `msw` |

Hard bans live in [imports-and-bans.md](imports-and-bans.md). The short list:
no `next-themes` (Vite SPA, not Next.js), no `@base-ui/react` (we lock to
Radix), no FontAwesome / PrimeIcons (Lucide only), no leftover Bolt
scaffolding artifacts (asserted by `go/apiserver/frontend_embed_test.go`).

## How this differs from the design mock

The design mock — checked in at [`design-mocks/`](../../design-mocks/) and
mirrored from `github.com/denisvmedia/inventario-design` — is a single-page
state-machine demo (no router, no data layer, no i18n). The Inventario app
keeps the mock's visual contract verbatim and wires it onto the same
hierarchy of abstractions a multi-tenant CRUD app needs:

| Mock | Inventario |
| --- | --- |
| `view` state in `App.tsx` | `react-router-dom` v7 routes (`src/app/router.tsx`) |
| `MOCK_ITEMS.find(...)` | TanStack Query hooks per feature (`features/<name>/hooks.ts`) |
| Inline strings | `t("namespace:key")` via react-i18next, en bundled + cs/ru lazy |
| Plain prop drilling | Feature contexts (`AuthContext`, `GroupContext`) for cross-cutting state |
| One JSON `mock.ts` | OpenAPI-generated DTOs in `src/types/api.d.ts` + per-feature `api.ts` |

The visual rules (no `forwardRef`, no `hsl()` wrappers, no
`@tailwindcss/animate`, OKLCH tokens, no purple, no drop shadows) are
mirrored in [styles-and-tokens.md](styles-and-tokens.md) and
[components.md](components.md).

## Cross-references

- Epic: [#1397](https://github.com/denisvmedia/inventario/issues/1397) — full
  React rewrite (28 sub-issues #1398–#1425; this doc is #1424).

- Testing harness: [testing.md](testing.md) (#1418, PR #1433).
- Perf gates: [perf.md](perf.md) (#1420, PR #1437).
- Screenshots: [screenshots.md](screenshots.md).
- Migration history: [migration-conventions.md](migration-conventions.md)
  (cutover #1423, PR #1457).
