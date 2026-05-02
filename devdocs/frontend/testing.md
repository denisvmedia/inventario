# Frontend React testing — Vitest, MSW, jest-axe

Owns the unit + integration test toolchain for `frontend/`. PR #1432 (#1418) wired the harness; this doc is the contract every per-feature page test should follow.

## Layout

```
frontend/src/test/
├── setup.ts          # global hooks: jest-dom, jest-axe, MSW server, sonner mock, i18n boot, matchMedia stub
├── server.ts         # shared msw `setupServer()` instance
├── render.tsx        # `renderWithProviders()` helper
└── handlers/         # per-endpoint MSW factories
    ├── index.ts      # public surface — re-exports + apiUrl helper
    ├── auth.ts
    ├── groups.ts
    ├── commodities.ts
    ├── locations.ts
    ├── files.ts
    ├── tags.ts
    ├── exports.ts
    ├── members.ts
    └── search.ts
```

## `renderWithProviders`

Composes the same provider stack the app boots with, in the same order, so a passing test maps to a passing production render:

```
ThemeProvider → DensityProvider → QueryClientProvider → MemoryRouter
                                                     └─ (optional) AuthProvider → GroupProvider
```

`AuthProvider` and `GroupProvider` are **opt-in** (`withAuth`, `withGroup`) because most route-level tests want to mount them inside a `<Route element>` so the `/auth/me` probe and the `:groupSlug` resolution fire only after navigation resolves. Leaf-component tests that read `useAuth()` / `useCurrentGroup()` without owning the route boundary set the flags.

```tsx
import { Route, renderWithProviders } from "@/test/render"

// 1. Route-level test — mount providers per-route inside the element:
renderWithProviders({
  initialPath: "/private",
  routes: (
    <>
      <Route
        path="/private"
        element={
          <AuthProvider>
            <ProtectedRoute>
              <Probe />
            </ProtectedRoute>
          </AuthProvider>
        }
      />
      <Route path="/login" element={<LoginStub />} />
    </>
  ),
})

// 2. Leaf-component test — let the helper wrap the providers for you:
renderWithProviders({
  withAuth: true,
  withGroup: true,
  children: <SomeWidgetThatReadsBothContexts />,
})
```

Returns the standard `RenderResult` plus the `queryClient` so cache assertions are a one-liner:

```tsx
const { queryClient } = renderWithProviders({ ... })
expect(queryClient.getQueryData(authKeys.currentUser())).toMatchObject({ ... })
```

## MSW handler factories

Tests register per-case handlers via `server.use(...)` — never edit `server.ts`'s base set. Handlers live under `src/test/handlers/<feature>.ts` and are namespace-imported from the index:

```ts
import { authHandlers, groupHandlers } from "@/test/handlers"

server.use(
  ...authHandlers.signedIn({ user: { name: "Test" } }),
  ...groupHandlers.list([{ id: "g1", slug: "household", name: "Household" }])
)
```

Each module exposes focused factories that take a fixture and return an array of handlers. The array shape is what makes `...spread` composition feel native at the call site.

| Variant            | Returns                                                     |
| ------------------ | ----------------------------------------------------------- |
| `signedIn(opts?)`  | `[GET /auth/me, POST /auth/refresh, POST /auth/logout]`     |
| `signedOut()`      | `[GET /auth/me 401, POST /auth/refresh 401]`                |
| `transientServerError()` | `[GET /auth/me 503]` — for "blip doesn't bounce to /login" tests |
| `groupHandlers.list(groups?)`  | `[GET /groups]` returning the JSON:API envelope          |
| `groupHandlers.empty()`        | `[GET /groups]` returning `[]` (no-group state)          |
| `groupHandlers.error(status?)` | `[GET /groups]` returning the given status code          |

For ad-hoc handlers, import `apiUrl` from `@/test/handlers` and reach for `msw.http.*` directly — no need to add a module.

## a11y assertions

`jest-axe` is registered globally. Page-level component tests should run `axe(container)` and fail on violations:

```tsx
import { axe } from "jest-axe"

it("has no axe violations", async () => {
  const { container } = render(...)
  expect(await axe(container)).toHaveNoViolations()
})
```

If a violation is intentional (e.g. a vendored Radix primitive's known issue), suppress it locally with `jest-axe`'s `runOptions` rather than skipping the test.

## Coverage gate

Enforced in `vitest.config.ts` — CI fails if coverage drops below:

| Metric     | Threshold |
| ---------- | --------- |
| Statements | 80%       |
| Lines      | 80%       |
| Functions  | 80%       |
| Branches   | 70%       |

Excluded paths:

- `src/components/ui/**` — vendored shadcn primitives owned by the design mock.
- `src/app/**` — composition-only (router, providers, App, Shell). Covered by Playwright (#1419).
- `src/components/{AppSidebar,CommandPalette,GroupSelector}.tsx` — composite shell components; integration coverage lands with #1419 / #1414.
- `src/i18n/i18next.config.ts` — lazy backend for cs/ru exercised by the Settings page locale toggle (#1414).
- `src/main.tsx`, `src/types/**`, `src/vite-env.d.ts` — entry / generated.

Bump thresholds upward as feature pages land — never downward without a written reason in the PR body.

## Snapshot policy

Avoid full-DOM snapshots. Allowed:

- Small stable outputs — e.g. `formatCurrency` return values, an icon's compiled SVG.
- Tightly scoped objects — e.g. `i18n.options` after init.

If a feature page benefits from a snapshot, scope it to a slice (`screen.getByRole("...").outerHTML`) rather than the whole render tree.

## Sonner

`vi.mock("sonner")` runs globally in `setup.ts` so the real `<Toaster />` doesn't portal during tests. Tests that want to assert toast behavior re-mock `sonner` locally — the per-file mock wins because `vi.mock` hoists.

## i18n in tests

`setup.ts` calls `await initI18n({ lng: "en" })` once before MSW starts. `useTranslation()` returns the en bundle; missing keys log a `[i18n] missing key …` warning per `i18next.config.ts`'s dev handler. Tests that want to assert against a specific locale call `await i18next.changeLanguage("...")` and reset back to `"en"` in their `afterEach`.

## When in doubt

- Need a test fixture? Add a factory under `src/test/handlers/<feature>.ts`.
- Need a new provider? Add it to `renderWithProviders` behind a flag.
- Need to lower a threshold? Don't — find the test instead.
