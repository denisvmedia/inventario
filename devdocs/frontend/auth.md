# Authentication

How the React frontend authenticates, stores credentials, refreshes
sessions, and decides who is signed in. This is the **frontend** view —
the wire protocol and server-side token lifecycle live in
[`../REFRESH_TOKEN_SYSTEM.md`](../REFRESH_TOKEN_SYSTEM.md) and
[`../CSRF_PROTECTION.md`](../CSRF_PROTECTION.md); route-guard placement is in
[routing.md](routing.md). This doc does not repeat them — it explains how the
frontend implements its half of the contract.

Source of truth in code:

| Concern | File |
| --- | --- |
| Auth state (one source of truth for "who is signed in") | `src/features/auth/AuthContext.tsx` |
| Credential storage | `src/lib/auth-storage.ts` |
| Boot-time speculative refresh | `src/features/auth/bootRefresh.ts` |
| HTTP client (Bearer, CSRF, 401-retry, plane routing) | `src/lib/http.ts` |
| Auth API calls (login/logout/register/MFA/…) | `src/features/auth/api.ts` |
| TanStack Query wrappers + keys | `src/features/auth/hooks.ts`, `keys.ts` |
| Route guards + redirects | `src/components/routing/`, see [routing.md](routing.md) |
| Login page + error surfacing | `src/pages/auth/LoginPage.tsx` |

## Auth state model

`AuthContext` exposes one source of truth via `useAuth()`. The `user` field is
**tri-state** — and the third state is the one that matters:

| `user` | Meaning | Guard behavior |
| --- | --- | --- |
| `CurrentUser` (object) | Signed in. Only branch where `isAuthenticated` is `true`. | Render the page. |
| `null` | Definitively signed out — no token, or `/auth/me` returned **401**. | Redirect to `/login`. |
| `undefined` | **Unknown** — the boot probe hasn't settled, or it errored with a non-401 status (transient 5xx / network blip). | Hold the boot fallback; do **not** bounce to `/login`. |

`isInitialized` is `true` once there is a definitive answer (the probe settled,
or there was no token to probe with). It stays `false` while the boot refresh is
in flight and on transient backend errors, so the boot fallback renders instead
of the login page.

Read these through `useAuth()` (or `useOptionalAuth()` for chrome that mounts in
both authenticated and bare-boot states). **Never** branch on `useAuth().user`
inside a page to gate access — wrap the page in `<ProtectedRoute>` so the
redirect, the `?redirect=` param, and the loading state stay uniform
([routing.md](routing.md)).

## Token & session handling

Three credentials, three homes:

| Credential | Storage | Accessor | Notes |
| --- | --- | --- | --- |
| Access token (short-lived JWT) | `localStorage["inventario_token"]` | `getAccessToken` / `setAccessToken` | Sent as `Authorization: Bearer …`. |
| CSRF token | `sessionStorage["inventario_csrf_token"]` (+ in-memory cache) | `getCsrfToken` / `setCsrfToken` | Sent as `X-CSRF-Token` on mutating requests. |
| Refresh token (long-lived) | **httpOnly cookie** (never JS-readable) | — (browser-attached) | Rooted at `/api/v1` (tenant) and `/api/v1/backoffice` (back-office). |

`src/lib/auth-storage.ts` is the only module that touches storage; all access
goes through it (it also degrades gracefully when storage is unavailable, e.g.
SSR or a locked-down browser).

### Boot sequence

On startup `AuthProvider` resolves who is signed in:

1. **Token present at mount?** → skip the speculative refresh; go straight to the
   `/auth/me` probe.
2. **No token?** → call `tryBootRefresh()` once. This speculatively POSTs
   `/auth/refresh`; the browser attaches the httpOnly refresh cookie if it
   exists. This closes the OAuth-callback gap (#1394): the provider 302s the
   browser back with **no** access token in `localStorage`, only the cookie the
   backend just minted — without this step the guard would see "no token" and
   undo the sign-in.
   - **Success** (200 + `access_token`) → store the token + CSRF, invalidate the
     `currentUser` query so the `/auth/me` probe fires with the new Bearer token.
   - **Failure** (401, network error, no cookie) → leave storage empty; the guard
     bounces to `/login` as normal.
3. **`/auth/me` probe** (`useCurrentUser`, enabled only when a token is present)
   resolves the `CurrentUser`. Its outcome drives the tri-state above.

`tryBootRefresh()` is a **one-shot** (module-level `attempted` guard + a
memoized in-flight promise) so a React StrictMode double-mount doesn't fire two
refreshes. It never throws — a cold tab with no cookie is a normal "no session
yet" outcome, not an error.

## HTTP client (`src/lib/http.ts`)

A tiny `fetch` wrapper every feature slice uses through TanStack Query. Per
request it:

- **Sets `Accept: application/vnd.api+json`** and, for non-GET JSON bodies,
  `Content-Type: application/vnd.api+json` (FormData uploads keep the browser's
  multipart boundary).
- **Attaches the Bearer token** for the request's plane (see below).
- **Attaches `X-CSRF-Token`** on mutating verbs (`POST`/`PUT`/`PATCH`/`DELETE`).
  CSRF rides the **header only — never the body**.
- **Rewrites group-scoped paths**: `/commodities` → `/g/{slug}/commodities` when
  a group is active (see [routing.md](routing.md) for the group-context slot).
- **Handles 401 with single-flight refresh-and-retry**: on a 401 it calls the
  plane's refresh endpoint once (concurrent 401s await the same in-flight
  promise, so the backend sees one `/auth/refresh`, not N), then retries the
  original request with the new token. If the refresh fails, it clears auth and
  redirects to the login surface via `navigateToLogin` (an SPA navigation
  installed by `AuthProvider`, not a full-page reload).
- **Surfaces non-2xx as `HttpError`** (`status`, `url`, `data`) so React Query
  and pages can react. See "Surfacing auth errors" below.

### Non-refreshable auth paths

A 401 on `/auth/login`, `/auth/register`, or `/auth/refresh` (and the
back-office equivalents) is an **application-level error** — bad credentials or
an invalid refresh token — not session expiry. The wrapper lists these in
`NON_REFRESHABLE_AUTH_PATHS` and surfaces the real error body instead of firing
a doomed refresh loop. This is why a wrong-password login shows an inline error
rather than silently retrying.

### Plane awareness (tenant vs back-office)

The tenant plane and the back-office plane (`/admin/*`, `/backoffice/*`, gated
by `RequireBackofficeAuth` since #1785 Phase 6) are **separately credentialed** —
different access tokens, CSRF tokens, and httpOnly refresh cookies.
`isBackofficePath()` routes each request to the right plane's credentials and
refresh endpoint, with an independent single-flight slot per plane, so a 401 on
`/admin/*` refreshes via `/backoffice/auth/refresh` and never deduplicates
against a tenant refresh.

### Surfacing auth errors

Pages map an `HttpError` to copy with `parseServerError(err, fallback)`
(`src/lib/server-error.ts`). `LoginPage` funnels every login failure — 401, 422,
429, 5xx — through it into the destructive `server-error` Alert; the page stays
on `/login` and the banner clears as soon as the user edits a field. See
[forms.md](forms.md) for the broader server-error pattern.

## Route guards & unauthorized redirects

Full guard placement is in [routing.md](routing.md); the auth-relevant summary:

- `<ProtectedRoute>` bounces a signed-out user to
  `/login?redirect=<path>&reason=auth_required`. It branches on the tri-state
  `user` (render fallback while `undefined`, redirect while `null`, render
  children for a `CurrentUser`).
- The login page reads `?redirect=` and returns the user there after sign-in.
  `sanitizeRedirectPath()` (`src/lib/safe-redirect.ts`) rejects absolute /
  protocol-relative targets so a crafted query can't open-redirect off the app.
- `reason` keys live under `auth:session.*` (`session_expired`, `auth_required`)
  and render the banner at the top of the login page.
- A failed refresh from the http client redirects with
  `reason=session_expired`; a guard redirect uses `reason=auth_required`.

## Login input model — host-based tenancy

**The login request body is `{ email, password }` only. There is no
`tenantSlug`.** Tenancy is resolved **server-side** from the request host:
`HostTenantResolver` (`../../go/apiserver/tenant_context.go`) maps the subdomain
to a tenant (e.g. `acme.inventario.app` → tenant `acme`) before the login
handler runs, via `PublicTenantMiddleware`. In single-tenant mode (empty base
domain) the middleware selects the one tenant. The frontend never sends, asks
for, or displays a tenant identifier at login.

The issue that tracked this (#1038) noted an **optional** per-deployment "tenant
field on the login form" for installs that don't use subdomains. That field is
**intentionally not implemented** — the host-based model covers the supported
deployments, and adding a field would also require backend support to accept it.
Revisit only if a non-subdomain deployment needs it.

## Tenant UX — intentionally not surfaced

The tenant / organization name is **deliberately not shown** in the regular UI
(header, user menu, sidebar). This matches the canonical
[`design-mocks/`](../../design-mocks/), which carry no tenant branding, and the
host-based model where the subdomain already identifies the org. Tenant identity
is visible only to system admins on the `/admin/*` pages.

`CurrentUser` (the `/auth/me` payload, `Schema<"models.User">`) carries **no
tenant-name field**, so surfacing the tenant in the shell would require
extending that endpoint on the backend **and** a logged deviation in
[design-deviations.md](design-deviations.md). Neither is in scope today; this is
a recorded decision, not an oversight.

## MFA (two-step login)

When a user has TOTP enabled, `POST /auth/login` returns **200** with
`mfa_required: true` + a short-lived `mfa_token` instead of issuing tokens
(credentials were correct — only the *step* is incomplete). `login()` returns a
discriminated `LoginOutcome` (`"ok"` | `"mfa_required"`); `LoginPage` swaps the
password form for `<MFAChallenge>`, which exchanges the `mfa_token` + a TOTP or
backup code at `POST /auth/login/mfa` for a session. Cancelling drops the
challenge (the `mfa_token` expires server-side anyway).

## See also

- [`../REFRESH_TOKEN_SYSTEM.md`](../REFRESH_TOKEN_SYSTEM.md) — dual-token model,
  refresh/revocation/blacklisting, impersonation sessions (backend).
- [`../CSRF_PROTECTION.md`](../CSRF_PROTECTION.md) — CSRF token issuance and
  validation (backend).
- [routing.md](routing.md) — guards, the group-context slot, `?redirect=`.
- [forms.md](forms.md) — `parseServerError` and server-error surfacing.
- [testing.md](testing.md) — the MSW auth handlers and `renderWithProviders`.
