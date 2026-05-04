# Data layer

TanStack Query + a small `lib/http.ts` wrapper. One feature slice per
domain (auth, group, commodity, location, area, file, tag, export,
search, member, invite). The split is documented as part of issue #1403.

## Pieces

```
frontend/src/lib/
├── http.ts              # fetch wrapper: bearer + CSRF + group rewrite + 401-refresh
├── auth-storage.ts      # localStorage tokens
├── group-context.ts     # active group slug (read by http.ts)
├── navigation.ts        # navigateToLogin / safe redirect helpers
└── query-client.ts      # createQueryClient() — staleTime/retry defaults

frontend/src/features/<name>/
├── api.ts        # raw HTTP calls returning typed DTOs (no React)
├── keys.ts       # query-key factory: <name>Keys.list(slug, opts) etc.
├── hooks.ts      # useFoo / useUpdateFoo wrappers around useQuery / useMutation
├── schemas.ts    # zod schemas for forms (see forms.md)
└── constants.ts  # closed enum constants (status, type, sort field)
```

## HTTP wrapper

`src/lib/http.ts` is a thin `fetch` adapter; never call `fetch` directly
from feature code. It does:

- JSON:API content type (`application/vnd.api+json`).
- Bearer token from `auth-storage.ts`.
- CSRF header on mutating methods.
- `/api/v1/<resource>` → `/api/v1/g/{slug}/<resource>` rewriting when a
  group is active. The slug is read from `getCurrentGroupSlug()`,
  which falls back to the URL when the React context hasn't mirrored
  yet (first render of a `/g/:slug/*` route).
- 401 → single-flight refresh via the httpOnly refresh cookie. On
  refresh failure: clear auth, redirect to `/login` (skipped on the
  public paths and on `/auth/login`/`/register`/`/refresh`
  themselves).
- Non-2xx → throws `HttpError` with `status`, `url`, `data`. React
  Query surfaces it through `error`.

## QueryClient defaults

`createQueryClient()` (`src/lib/query-client.ts`):

```ts
defaults = {
  queries: {
    staleTime: 30_000,
    retry: (count, err) => {
      // Don't retry 4xx — http.ts has already resolved auth, retrying
      // races the redirect.
      if (err instanceof HttpError && err.status >= 400 && err.status < 500) return false
      return count < 1
    },
    refetchOnWindowFocus: true,
  },
  mutations: { retry: false },
}
```

Don't change these per-call without a reason. If you find yourself
setting `staleTime: Infinity` on every query in a slice, the data isn't
TanStack-Query-shaped — file an issue.

## Query keys

Every feature owns a `<name>Keys` factory in `features/<name>/keys.ts`:

```ts
// frontend/src/features/auth/keys.ts
export const authKeys = {
  all: ["auth"] as const,
  currentUser: () => [...authKeys.all, "currentUser"] as const,
}

// frontend/src/features/commodities/keys.ts
export const commodityKeys = {
  all: ["commodity"] as const,
  group: (slug: string) => [...commodityKeys.all, slug] as const,
  list: (slug: string, opts?: ListCommoditiesOptions) =>
    [...commodityKeys.group(slug), "list", listKeySuffix(opts)] as const,
  detail: (slug: string, id: string) => [...commodityKeys.group(slug), "detail", id] as const,
  values: (slug: string) => [...commodityKeys.group(slug), "values"] as const,
}
```

Rules:

- **Group-scoped keys include the slug.** Without it, navigating from
  `/g/household` to `/g/office` would reuse the cached household list
  while the http call goes to office, and the mismatch only resolves on
  the next refetch.
- **List keys serialize options deterministically.** `listKeySuffix`
  sorts arrays before serialising so two equivalent options objects
  produce the same key (see `commodities/keys.ts` for the reference
  implementation).
- **Other slices invalidate via the typed entry point.** Import the
  key factory; never copy-paste the raw array.

## Hooks

Each feature exposes `useFoo()` / `useUpdateFoo()` thin wrappers around
`useQuery` / `useMutation`:

```ts
// useCurrentUser — read
export function useCurrentUser() {
  return useQuery<CurrentUser>({
    queryKey: authKeys.currentUser(),
    queryFn: ({ signal }) => getCurrentUser(signal),
    enabled: !!getAccessToken(),
  })
}

// useLogin — mutation that primes the cache
export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation<CurrentUser | undefined, Error, LoginVars>({
    mutationFn: ({ email, password }) => login(email, password),
    onSuccess: (user) => {
      if (user) queryClient.setQueryData(authKeys.currentUser(), user)
      queryClient.invalidateQueries({ queryKey: authKeys.all })
    },
  })
}
```

Rules:

- **Forward `signal`** from `queryFn` to the underlying `fetch`. React
  Query passes its own `AbortSignal` and uses it for cancellation when
  components unmount or queries are invalidated.
- **`enabled`** gates queries that depend on prerequisites — auth token,
  active slug, a route param. Skipping the query is preferable to
  letting it 401 / 404 on every render.
- **Don't call `useQueryClient` inside leaf components** to nudge the
  cache. Build it into the feature hook so the contract lives in one
  place.

## Mutations and cache updates

Three patterns, in order of preference:

### 1. Invalidate on success (default)

```ts
useMutation({
  mutationFn: createCommodity,
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: commodityKeys.group(slug) })
  },
})
```

The simplest, hardest-to-get-wrong shape. Use it unless you have
measured-flicker pain.

### 2. Optimistic update with rollback

For hot paths where the user expects an instant change (toggle a tag,
delete a commodity, mark warranty seen). The pattern is the
`useLogout` reference from `features/auth/hooks.ts`:

```ts
useMutation<void, Error, void, { previousUser: CurrentUser | undefined }>({
  mutationFn: () => logout(),
  onMutate: async () => {
    await queryClient.cancelQueries({ queryKey: authKeys.currentUser() })
    const previousUser = queryClient.getQueryData<CurrentUser>(authKeys.currentUser())
    queryClient.removeQueries({ queryKey: authKeys.currentUser(), exact: true })
    return { previousUser }
  },
  onError: (_err, _vars, ctx) => {
    if (ctx?.previousUser) {
      queryClient.setQueryData(authKeys.currentUser(), ctx.previousUser)
    }
  },
  onSettled: () => {
    queryClient.invalidateQueries({ queryKey: authKeys.all })
  },
})
```

Required steps every time:

- `cancelQueries` first — otherwise an in-flight refetch overwrites
  your optimistic write.
- Return `{ previous… }` from `onMutate` so `onError` can roll back.
- `onSettled` invalidates so the cache settles to the server's truth.

### 3. Direct cache write (rare)

When the server response IS the new state and you don't want a refetch,
`setQueryData(key, response)` works. `useLogin` uses this to seed the
current-user cache and skip the boot probe. Don't do it for list-shaped
data — list pagination + filters make manual cache surgery brittle.

## Pagination, sort, search

The reference is the commodities list — `features/commodities/api.ts` +
`features/commodities/keys.ts`:

- Options object (`ListCommoditiesOptions`) → URL params via the same
  shape on both sides.
- The key factory's `listKeySuffix` serialises the options into a
  stable suffix, sorting arrays so order doesn't matter.
- `keepPreviousData: true` is **off by default** — turn it on per
  query if you want the previous page to stay rendered while the next
  page loads.

## CSRF + tokens

`auth-storage.ts` is the single localStorage gateway. Don't read
`localStorage["…"]` directly anywhere else:

- `getAccessToken()` / `setAccessToken()` / `clearAuth()`.
- `getCsrfToken()` / `setCsrfToken()`.

The CSRF token comes back in the login / refresh response and is
attached automatically on mutating methods.

## SSE / streams

Long-running ops (export, restore) use Server-Sent Events. The pattern:

- `useQuery` polls the resource (`exports:detail`) with a short
  `refetchInterval` while status is `pending` / `running`.
- For genuinely streaming progress, mount an `EventSource` inside a
  feature hook and write into the cache via `setQueryData` per event.
  See `features/export/` for the reference once it lands.

## Tests

MSW handler factories under `src/test/handlers/<feature>.ts` return
arrays for spread-into-`server.use(...)`:

```ts
import { authHandlers, groupHandlers } from "@/test/handlers"

server.use(
  ...authHandlers.signedIn({ user: { name: "Test" } }),
  ...groupHandlers.list([{ id: "g1", slug: "household", name: "Household" }])
)
```

Add a factory when a test needs a one-off variant; never edit the base
set in `server.ts` — that breaks isolation between tests. Pattern lives
in [testing.md](testing.md).

## Anti-patterns

- **`useEffect` to fetch.** Never. Use `useQuery` and let React Query
  own the lifecycle.
- **Calling `fetch` directly.** The wrapper's group rewrite, refresh,
  and CSRF behavior matter on every endpoint.
- **Dropping the slug from a list query key.** Cache key collisions
  between groups are silent until the user notices stale data.
- **Manual loading flags (`const [loading, setLoading] = useState`).**
  `useQuery` / `useMutation` already expose `isPending` / `isFetching`.
- **Try/catching a query inside the component.** Read `error` from the
  hook; surface it via the component's render path.
