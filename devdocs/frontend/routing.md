# Routing

`react-router-dom` v7 with declarative `<Routes>`. The route tree lives
in `src/app/router.tsx`; guards, redirects, and the title broadcast live
in `src/components/routing/`.

## Tree shape

```
/                         → RootRedirect (lazy)
/login                    → LoginPage (lazy)
/register                 → RegisterPage (lazy)
/forgot-password          → ForgotPasswordPage (lazy)
/reset-password           → ResetPasswordPage (lazy)
/verify-email             → VerifyEmailPage (lazy)
/invite/:token            → InviteAcceptPage (lazy)
/no-group                 → NoGroupPage (lazy)
/profile                  → ProfilePage (Shell layout)
/settings                 → SettingsPage (Shell layout)
/g/:groupSlug/            → DashboardPage (lazy, Shell layout)
/g/:groupSlug/locations   → LocationsListPage
/g/:groupSlug/locations/:id
/g/:groupSlug/areas/:id
/g/:groupSlug/commodities
/g/:groupSlug/commodities/:id
/g/:groupSlug/files
/g/:groupSlug/tags
/g/:groupSlug/exports
/g/:groupSlug/search
/g/:groupSlug/groups/settings
/g/:groupSlug/groups/members
*                         → NotFoundPage
```

The protected subtree mounts under `<Shell>` — the layout component that
renders the sidebar, top bar, and the route's `<Outlet>`. Public auth
pages render outside the shell.

## Real pages are lazy

Every real page in `app/router.tsx` is a `lazy` import:

```tsx
const DashboardPage = lazy(() =>
  import("@/pages/Dashboard").then((m) => ({ default: m.DashboardPage }))
)
```

Why — perf gates live and die by the entry-bundle budget (200 KB gzip,
see [perf.md](perf.md)). Eagerly-loaded pages land in the entry chunk
and burn through it fast.

Rules:

- **Real pages: always `lazy`.** The `.then((m) => ({ default: ... }))`
  shape is what lets us export named functions instead of defaults.
- **Placeholder pages: eager.** `PlaceholderPage` is shared across
  every "coming soon" route — there's nothing to split until the real
  page lands.
- **Wrap the lazy tree in `<Suspense>`** with a real fallback (a
  skeleton shaped like the page, or `null`). The Shell already mounts
  one global Suspense boundary.

## Guards

Three guard components in `src/components/routing/`:

### `ProtectedRoute`

Bounces unauthenticated users to `/login?redirect=…&reason=auth_required`.
Tri-state on `user`:

| `user` value | Behavior |
| --- | --- |
| `undefined` (still resolving / transient backend error) | Render `fallback` (default: `null`) |
| `null` (definitively logged out) | `<Navigate to="/login?…" replace />` |
| `CurrentUser` | Render `children` |

Always wrap protected pages in `<ProtectedRoute>` — never check
`useAuth().user` inside the page.

### `GroupRequiredRoute`

Bounces a logged-in user without an active group to `/no-group`. Wraps
the `/g/:groupSlug/*` subtree.

### `UngroupedRedirect`

Reverse of the above: bounces a logged-in user with at least one group
*away* from `/no-group`. Used on the no-group page itself to handle the
race where a group is created in another tab.

## Group context

`GroupContext` (`features/group/GroupContext.tsx`) reads the slug from
`useParams()` and mirrors it into the module-level slot
`group-context.ts` — the slot the http client reads. The URL is the
canonical source of truth; the slot is a non-React-context cache for
non-React callers (http wrapper, codegen helpers).

A `useEffect` in `GroupProvider` writes the slug; an inline fallback in
`getCurrentGroupSlug()` reads from `window.location` so the very first
render of a `/g/:slug/*` route doesn't fire a query against
`/api/v1/<resource>` (un-rewritten) before the effect commits. See
`src/lib/group-context.ts` for the rationale comment.

When you add a route under `/g/:groupSlug/*`:

- Read the slug via `useCurrentGroup().slug` from the GroupContext —
  not `useParams()` directly. The context resolves the group object too,
  so you avoid a second lookup.
- TanStack Query keys include the slug. See [data.md](data.md).

## RouteTitle

`<RouteTitle>` (`src/components/routing/RouteTitle.tsx`) writes the
page's title both into `document.title` (for browser tab) and into
`RouteTitleContext` (for the top bar to render the title visually).

Mount it once near the top of every page:

```tsx
import { RouteTitle } from "@/components/routing/RouteTitle"

export function CommoditiesListPage() {
  const { t } = useTranslation()
  return (
    <>
      <RouteTitle title={t("commodities:list.title")} />
      {/* page content */}
    </>
  )
}
```

The component owns the `useEffect` that writes both targets and clears
on unmount.

## Adding a new route

1. **Create the page** at `src/pages/<feature>/<Name>Page.tsx`. Start
   with the standard outer wrapper from [styles-and-tokens.md](styles-and-tokens.md):
   ```tsx
   <div className="flex flex-col gap-6 p-6 max-w-2xl mx-auto w-full">
   ```
2. **Add a `lazy()` import** at the top of `src/app/router.tsx`.
3. **Place the `<Route>`** in the right subtree:
   - Public auth → outside `<ProtectedRoute>`.
   - Logged-in but groupless → outside `<GroupRequiredRoute>` but
     inside `<ProtectedRoute>` (e.g. `/profile`, `/no-group`).
   - Group-scoped → inside `<GroupRequiredRoute>`, under
     `/g/:groupSlug/`.
4. **Mount `<RouteTitle title={t("…")} />`** at the top of the page.
5. **Add the sidebar entry** in `src/components/AppSidebar.tsx` (and
   `i18n/locales/en/common.json` under `nav.<key>`) if the route should
   appear in nav.
6. **Add the command-palette entry** in `src/components/CommandPalette.tsx`
   if the route should be navigable from `Cmd-K`.
7. **Test the redirect chain**: protected, group-required, and
   non-redirected. Use `renderWithProviders({ initialPath: "/…",
   routes: <Route … /> })`. See [testing.md](testing.md).

## Deep links and `?redirect=`

The login page reads `?redirect=` and bounces the user back after a
successful login. `sanitizeRedirectPath()` (`src/lib/safe-redirect.ts`)
rejects absolute / protocol-relative URLs so a crafted query can't
open-redirect off the app.

When you redirect to login from a guarded route, encode the current
path:

```ts
const redirect = location.pathname + location.search
const params = new URLSearchParams({ redirect, reason: "auth_required" })
return <Navigate to={`/login?${params.toString()}`} replace />
```

`reason` keys are in `auth:session.*` — see `LoginPage.tsx`'s
`SESSION_REASON_KEY` map.

## Code splitting and chunk shape

- Real pages → one chunk per page.
- Placeholder routes → one shared chunk for `PlaceholderPage`.
- Cross-route components (Shell, AppSidebar, command palette) live in
  the entry chunk.

When you add a real page, the entry-bundle budget barely moves — the
new chunk only loads when the user navigates to that route. Watch for:

- **Eagerly imported page module** — if you write `import {
  CommoditiesListPage } from "@/pages/…"` somewhere in the entry tree,
  it un-splits the chunk. Run `npm run size:why` to spot it.
- **Heavy library only used on one page** — keep the import inside the
  page (or a leaf) so it lands in that page's chunk, not in the entry.

## Anti-patterns

- **Nested `<BrowserRouter>`s.** There is exactly one `BrowserRouter`,
  in `src/main.tsx`. Tests use `MemoryRouter` via `renderWithProviders`.
- **`window.location.href = …` to navigate.** Use `useNavigate()` or
  `<Navigate>`. The hard exception is `navigateToLogin()` in
  `src/lib/navigation.ts`, which fires from non-React HTTP code.
- **Putting auth checks inside the page** (`if (!user) return null`).
  Use `<ProtectedRoute>` so the redirect, the `?redirect=` param, and
  the loading state are uniform.
- **Reading the slug from `useParams()` in a non-route component.**
  Read `useCurrentGroup()` instead — it works for tests that mount the
  component without the route boundary.
