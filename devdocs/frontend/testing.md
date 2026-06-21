# Frontend React testing — Vitest, MSW, jest-axe

Owns the unit + integration test toolchain for `frontend/`. PR #1432 (#1418) wired the harness; this doc is the contract every per-feature page test should follow.

## Layout

```
frontend/src/test/
├── setup.ts          # global hooks: jest-dom, jest-axe, MSW server, sonner mock, i18n boot, matchMedia stub
├── server.ts         # shared msw `setupServer()` instance
├── render.tsx        # `renderWithProviders()` helper
└── handlers/         # per-endpoint MSW factories (18 modules)
    ├── index.ts      # public surface — re-exports + apiUrl helper
    ├── areas.ts
    ├── auth.ts
    ├── backoffice.ts
    ├── commodities.ts
    ├── commodityScan.ts
    ├── currencyMigrations.ts
    ├── exports.ts
    ├── files.ts
    ├── groups.ts
    ├── loans.ts
    ├── locations.ts
    ├── maintenance.ts
    ├── members.ts
    ├── search.ts
    ├── services.ts
    ├── supplies.ts
    └── tags.ts
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

## Driving Radix Select / portal-based primitives

`@testing-library/user-event`'s `selectOptions()` drives a real
`HTMLSelectElement` via DOM `change` events. shadcn `Select` (around
`@radix-ui/react-select`) renders no `<select>` — its trigger is a
`role="combobox"` button and the listbox lives in a portal. Calling
`selectOptions()` against the trigger silently no-ops, and the form
reads as un-interacted (see #1629 for the prior `.skip` graveyard).

Use the shared helper at `@/test/radix` instead:

```tsx
import userEvent from "@testing-library/user-event"
import { pickRadixSelect } from "@/test/radix"

const user = userEvent.setup()
await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
await pickRadixSelect(user, /^Location$/i, { optionLabel: /^Home$/i })
await pickRadixSelect(user, /^Area$/i, { optionLabel: /^Garage$/i })
```

The helper clicks the trigger (Radix listens for `onPointerDown` +
`onClick`, both of which `userEvent.click` issues), picks the option
inside the freshly-mounted listbox by accessible name, and then waits
for the portal to unmount before returning. That last step matters
for sequential picks: a stale listbox left over from the previous
Select is the most common flake source — `findByRole("listbox")`
happily returns the wrong one if you don't wait. Pre-flight, the
helper also asserts the trigger isn't disabled so paired selects
(e.g. Location → Area in `CommodityFormDialog`, where Area is
`disabled` until a Location is picked) fail fast with a useful error
rather than silently picking against the wrong listbox.

JSDOM gaps that the global setup already fills for Radix:

- `Element.prototype.hasPointerCapture` / `setPointerCapture` /
  `releasePointerCapture` — stubbed in `test/setup.ts`. Radix
  Select's `Trigger` touches these during pointer events; missing
  stubs throw before the listbox can open.
- `Element.prototype.scrollIntoView` — stubbed. Radix scrolls the
  active option into view when the listbox opens.
- `ResizeObserver` — stubbed. Radix's `use-size` reads it during
  layout.

If you're adding a new portal-based primitive (Popover, Dropdown,
Hover Card, Combobox) and the user-event API doesn't have a natural
fit, extend `src/test/radix.ts` with a sibling helper rather than
reaching for raw `fireEvent` in the test file — keeping the
interaction recipe in one place is what stops the next migration
from re-introducing the `.skip` graveyard.

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
| Statements | 76%       |
| Lines      | 79%       |
| Functions  | 75%       |
| Branches   | 67%       |

Excluded paths:

- `src/components/ui/**` — vendored shadcn primitives, owned upstream by shadcn.
- `src/app/**` — composition-only (router, providers, App, Shell). Covered by Playwright (#1419).
- `src/components/{AppSidebar,CommandPalette,GroupSelector}.tsx` — composite shell components; integration coverage lands with #1419 / #1414.
- `src/i18n/i18next.config.ts` — lazy backend for cs/ru exercised by the Settings page locale toggle (#1414).
- `src/main.tsx`, `src/types/**`, `src/vite-env.d.ts` — entry / generated.

Bump thresholds upward as feature pages land; only lower them with a written reason in the PR body (the current numbers reflect the #1629 Radix-Select coverage gap — see the rationale comment in `vitest.config.ts`).

## Snapshot policy

Avoid full-DOM snapshots. Allowed:

- Small stable outputs — e.g. `formatCurrency` return values, an icon's compiled SVG.
- Tightly scoped objects — e.g. `i18n.options` after init.

If a feature page benefits from a snapshot, scope it to a slice (`screen.getByRole("...").outerHTML`) rather than the whole render tree.

## Fake timers + userEvent + MSW

Tests that need to skip a `setTimeout`-driven UX milestone (e.g. "show success state for 1.5s, then log out") should use this exact recipe — the default `vi.useFakeTimers()` swallows microtasks MSW + react-query rely on, so naive setups deadlock (see #1439 for the failure list).

Scope the fake-timer window to the single test that needs it; don't move `useFakeTimers` into the file-level `beforeEach` — every other case in the file would then need its userEvent setup wired through the fake clock for no good reason.

```tsx
import { expect, it, vi } from "vitest"
import { waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

it("posts the success state, then logs out after 1500ms", async () => {
  // `shouldAdvanceTime: true` keeps msw + react-query happy by letting
  // their Promise / microtask chains run on the host scheduler while
  // the fake clock still tracks real time.
  vi.useFakeTimers({ shouldAdvanceTime: true })
  try {
    // Tell userEvent to feed its internal waits through the fake clock.
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    // …interact via `user`, then for the deferred milestone:
    await vi.advanceTimersByTimeAsync(1600) // jump past the page's setTimeout
    await waitFor(() => expect(thingThatHappensAfter).toBe(true))
  } finally {
    vi.useRealTimers()
  }
})
```

What NOT to do: bare `vi.useFakeTimers()` (default `toFake` set blocks msw), `toFake: ["setTimeout"]` alone (userEvent's internal `setTimeout` still deadlocks), or switching to fake timers *after* the page schedules the setTimeout (the timer is registered against real timers and won't fire under fake-time).

## Sonner

`vi.mock("sonner")` runs globally in `setup.ts` so the real `<Toaster />` doesn't portal during tests. Tests that want to assert toast behavior re-mock `sonner` locally — the per-file mock wins because `vi.mock` hoists.

## i18n in tests

`setup.ts` calls `await initI18n({ lng: "en" })` once before MSW starts. `useTranslation()` returns the en bundle; missing keys log a `[i18n] missing key …` warning per `i18next.config.ts`'s dev handler. Tests that want to assert against a specific locale call `await i18next.changeLanguage("...")` and reset back to `"en"` in their `afterEach`.

## When in doubt

- Need a test fixture? Add a factory under `src/test/handlers/<feature>.ts`.
- Need a new provider? Add it to `renderWithProviders` behind a flag.
- Need to lower a threshold? Don't — find the test instead.
