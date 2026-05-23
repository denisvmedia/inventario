// Tiny fetch wrapper used by every feature slice via TanStack Query.
//
// Behaviors (see issue #1403):
//   - JSON:API content type
//   - Bearer + CSRF
//   - Group-scoped URL rewriting (/api/v1/<resource> → /api/v1/g/{slug}/<resource>)
//   - 401 → access-token refresh via httpOnly refresh cookie, with single-flight
//     deduplication; on refresh failure, clear auth and redirect to /login
//   - Surfaces non-2xx as `HttpError` so React Query can react via onError
//
// Plane-awareness (#1785 Phase 6): the back-office and tenant auth planes
// are separately credentialed — see features/backoffice/auth/storage.ts.
// `isBackofficePath` routes each request to its plane's token, CSRF, and
// refresh endpoint without callers having to think about it.
import {
  clearAuth,
  clearImpersonationReturn,
  getAccessToken,
  getCsrfToken,
  getImpersonationReturn,
  setAccessToken,
  setCsrfToken,
} from "./auth-storage"
import {
  clearBackofficeAuth,
  getBackofficeAccessToken,
  getBackofficeCsrfToken,
  setBackofficeAccessToken,
  setBackofficeCsrfToken,
} from "@/features/backoffice/auth/storage"
import { getCurrentGroupSlug } from "./group-context"
import { hardRedirect, navigateToLogin, navigateToMaintenance } from "./navigation"

const BASE_URL = "/api/v1"

// Resource prefixes that live under /g/{slug}/ when a group is active. Order
// matters: more specific prefixes (e.g. `/commodities/values`) before the
// shorter ones (`/commodities`) so the longer match wins via array order.
const GROUP_SCOPED_PREFIXES = [
  "/commodities/values",
  "/locations",
  "/areas",
  "/commodities",
  "/files",
  "/exports",
  "/tags",
  // /loans and /loans/counts (#1452). The per-commodity loan paths are
  // rewritten via the /commodities prefix above; this entry handles the
  // group-wide /lent surface + the bulk-counts endpoint.
  "/loans",
  // /services and /services/counts (#1508). Same shape as /loans —
  // per-commodity service paths ride the /commodities prefix; the
  // group-wide "in-service" surface uses these entries.
  "/services",
  // /maintenance (#1368). Per-commodity maintenance paths ride the
  // /commodities prefix above; this entry handles the group-wide
  // /maintenance surface plus the per-row PATCH / DELETE / done
  // endpoints mounted at /g/{slug}/maintenance/{id}*.
  "/maintenance",
  "/upload-slots",
  "/uploads",
  "/settings",
  "/search",
  "/storage-usage",
] as const

// Auth endpoints where a 401 is an application-level error (bad credentials,
// invalid refresh token) rather than a session-expiry event — never trigger a
// refresh-and-retry loop on these. Both planes' login + refresh paths are
// listed: the BE rejects malformed credentials with 401 and we want the
// FE to surface the actual error body rather than re-fire a doomed refresh.
const NON_REFRESHABLE_AUTH_PATHS = new Set([
  "/auth/login",
  "/auth/register",
  "/auth/refresh",
  "/backoffice/auth/login",
  "/backoffice/auth/login/mfa",
  "/backoffice/auth/refresh",
])

// Routes the user might already be on when a 401 fires; redirecting from them
// would either be a no-op (already at /login) or interrupt a flow that
// intentionally allows unauthenticated access. The `/backoffice/login` entry
// keeps the back-office plane from refresh-bouncing while an operator is
// already on the back-office login screen (mirrors the tenant `/login`).
const PUBLIC_PATHS = [
  "/login",
  "/register",
  "/verify-email",
  "/reset-password",
  "/invite",
  "/backoffice/login",
]

// Path → auth plane. A path belongs to the back-office plane iff it lives
// under /admin/* (gated by RequireBackofficeAuth since Phase 3) or the
// /backoffice/* subtree itself. Everything else uses the tenant plane.
function isBackofficePath(path: string): boolean {
  return (
    path === "/admin" ||
    path.startsWith("/admin/") ||
    path === "/backoffice" ||
    path.startsWith("/backoffice/")
  )
}

const MUTATING_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"])

export type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE"

export interface HttpRequestInit {
  method?: HttpMethod
  // JSON-serializable body. For GET/HEAD it is ignored.
  body?: unknown
  // Extra headers — merged after the wrapper's defaults.
  headers?: Record<string, string>
  // Forwarded to fetch — TanStack Query passes its own AbortSignal here.
  signal?: AbortSignal
  // Skip the /g/{slug}/ rewrite even if a group is active (for explicit
  // cross-group lookups; rare).
  skipGroupRewrite?: boolean
  // Skip the 401 → refresh → retry dance. The wrapper sets this on its own
  // refresh request; callers normally never set it.
  skipAuthRefresh?: boolean
  // Tag for /auth/me probes. "background" 401s do not clear auth or redirect;
  // "user-initiated" follows the normal flow. Default is "user-initiated".
  authCheck?: "background" | "user-initiated"
}

export interface HttpResponse<T> {
  data: T
  response: Response
  status: number
}

export class HttpError extends Error {
  readonly status: number
  readonly url: string
  readonly data: unknown

  constructor(message: string, status: number, url: string, data: unknown) {
    super(message)
    this.name = "HttpError"
    this.status = status
    this.url = url
    this.data = data
  }
}

interface RefreshResponse {
  access_token?: string
  csrf_token?: string
}

// Single-flight refresh: concurrent 401s wait on the same in-flight refresh
// promise so the backend sees one /auth/refresh call, not N. One slot per
// plane — a back-office refresh in flight must not block (or be deduped by)
// a tenant refresh.
let refreshInFlight: Promise<string> | null = null
let backofficeRefreshInFlight: Promise<string> | null = null

// Single-flight impersonation-expiry recovery: 401s that overlap the
// in-flight `end` call's window all wait on the same promise, so the
// backend sees one POST /admin/impersonation/end per in-flight burst rather
// than one per concurrent 401. This dedups requests that overlap the
// in-flight window — it does NOT dedup a request that arrives in the gap
// between the `end` succeeding and the browser navigating away; that gap is
// closed by the hard-redirect tearing the page down, not by this guard.
let impersonationEndInFlight: Promise<void> | null = null

// The /admin/impersonation/end path — kept as a constant so handle401 can
// recognise (and skip recovery for) the `end` call itself.
const IMPERSONATION_END_PATH = "/admin/impersonation/end"

function applyGroupRewrite(path: string): string {
  const slug = getCurrentGroupSlug()
  if (!slug) {
    if (import.meta.env.DEV) {
      for (const prefix of GROUP_SCOPED_PREFIXES) {
        if (path.startsWith(prefix)) {
          console.warn(
            `[http] group-scoped request ${path} issued from a non-group route — ` +
              "no /g/{slug}/ in the URL; backend will likely 404."
          )
          break
        }
      }
    }
    return path
  }
  for (const prefix of GROUP_SCOPED_PREFIXES) {
    if (path === prefix || path.startsWith(`${prefix}/`) || path.startsWith(`${prefix}?`)) {
      return `/g/${encodeURIComponent(slug)}${path}`
    }
  }
  return path
}

function buildUrl(path: string, skipGroupRewrite: boolean | undefined): string {
  // Accept either a path ("/commodities") or a full /api/v1-prefixed URL
  // (rare, used by tests and by the refresh helper). Strip the prefix so we
  // operate on the canonical short form throughout the rewrite logic.
  let normalized = path.startsWith(BASE_URL) ? path.slice(BASE_URL.length) : path
  if (!normalized.startsWith("/")) normalized = `/${normalized}`
  const rewritten = skipGroupRewrite ? normalized : applyGroupRewrite(normalized)
  return `${BASE_URL}${rewritten}`
}

function buildHeaders(method: HttpMethod, path: string, init: HttpRequestInit): Headers {
  const headers = new Headers({
    Accept: "application/vnd.api+json",
  })
  if (init.body !== undefined && method !== "GET" && !(init.body instanceof FormData)) {
    // FormData uploads (multipart) need the browser-generated
    // `multipart/form-data; boundary=...` header — overriding it with
    // application/vnd.api+json strips the boundary and the request
    // arrives at the BE empty.
    headers.set("Content-Type", "application/vnd.api+json")
  }
  const backoffice = isBackofficePath(path)
  const accessToken = backoffice ? getBackofficeAccessToken() : getAccessToken()
  if (accessToken) {
    headers.set("Authorization", `Bearer ${accessToken}`)
  }
  if (MUTATING_METHODS.has(method)) {
    const csrf = backoffice ? getBackofficeCsrfToken() : getCsrfToken()
    if (csrf) headers.set("X-CSRF-Token", csrf)
  }
  if (init.authCheck === "user-initiated") {
    headers.set("X-Auth-Check", "user-initiated")
  }
  if (init.headers) {
    for (const [k, v] of Object.entries(init.headers)) {
      headers.set(k, v)
    }
  }
  return headers
}

async function parseBody(response: Response): Promise<unknown> {
  if (response.status === 204) return null
  const ctype = response.headers.get("content-type") ?? ""
  if (ctype.includes("json")) {
    const text = await response.text()
    return text ? JSON.parse(text) : null
  }
  return await response.text()
}

// hasTypedProductError checks the response body for a JSON:API errors[]
// entry carrying a typed feature-namespaced code (e.g. `commodity_scan.
// provider_disabled`). Those are product-level errors riding on 503 by
// BE convention (#1720), distinct from "the whole API is down" — the
// feature handler maps them to an inline banner, so the global
// 503 → /maintenance bounce in performRequest must skip them.
//
// A code is considered "typed" when it contains a dot — every typed BE
// error code is `<feature>.<reason>`. Untyped server errors (plain
// "internal server error" string bodies, JSON:API "Service Unavailable"
// titles) keep the maintenance bounce.
function hasTypedProductError(body: unknown): boolean {
  if (body === null || typeof body !== "object") return false
  const errors = (body as { errors?: unknown }).errors
  if (!Array.isArray(errors)) return false
  return errors.some((e) => {
    if (e === null || typeof e !== "object") return false
    const code = (e as { code?: unknown }).code
    return typeof code === "string" && code.includes(".")
  })
}

async function refreshAccessToken(): Promise<string> {
  if (refreshInFlight) return refreshInFlight
  refreshInFlight = (async () => {
    const url = `${BASE_URL}/auth/refresh`
    const response = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: "{}",
      // Match the performRequest contract — no browser HTTP cache for
      // API traffic; React Query owns the freshness layer above.
      cache: "no-store",
    })
    if (!response.ok) {
      throw new HttpError(
        `Refresh failed with ${response.status}`,
        response.status,
        url,
        await parseBody(response).catch(() => null)
      )
    }
    const payload = (await response.json()) as RefreshResponse
    if (!payload.access_token) {
      throw new HttpError("Refresh returned no access_token", response.status, url, payload)
    }
    setAccessToken(payload.access_token)
    if (payload.csrf_token) {
      setCsrfToken(payload.csrf_token)
    }
    return payload.access_token
  })()
  try {
    return await refreshInFlight
  } finally {
    refreshInFlight = null
  }
}

// refreshBackofficeAccessToken mirrors refreshAccessToken for the back-office
// plane (#1785 Phase 6). The two planes share zero state — separate httpOnly
// refresh cookies (rooted at /api/v1 vs /api/v1/backoffice), separate access
// tokens, separate CSRF tokens — so a 401 on /admin/* MUST hit
// /backoffice/auth/refresh and a 401 on a tenant path MUST hit
// /auth/refresh. The dispatcher in `handle401` picks the right one.
async function refreshBackofficeAccessToken(): Promise<string> {
  if (backofficeRefreshInFlight) return backofficeRefreshInFlight
  backofficeRefreshInFlight = (async () => {
    const url = `${BASE_URL}/backoffice/auth/refresh`
    const response = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: "{}",
      cache: "no-store",
    })
    if (!response.ok) {
      throw new HttpError(
        `Back-office refresh failed with ${response.status}`,
        response.status,
        url,
        await parseBody(response).catch(() => null)
      )
    }
    const payload = (await response.json()) as RefreshResponse
    if (!payload.access_token) {
      throw new HttpError(
        "Back-office refresh returned no access_token",
        response.status,
        url,
        payload
      )
    }
    setBackofficeAccessToken(payload.access_token)
    // The back-office BE rotates CSRF via the X-CSRF-Token header (read by
    // performRequest below); the JSON body does not always carry it, so the
    // header path is the source of truth. Keep this opportunistic assignment
    // for parity with refreshAccessToken — harmless when absent.
    const csrf = response.headers.get("X-CSRF-Token") ?? response.headers.get("x-csrf-token")
    if (csrf) setBackofficeCsrfToken(csrf)
    return payload.access_token
  })()
  try {
    return await backofficeRefreshInFlight
  } finally {
    backofficeRefreshInFlight = null
  }
}

interface ImpersonationEndResponse {
  access_token?: string
  csrf_token?: string
}

// Recovers the operator's back-office session when an impersonation
// access token has auto-expired (#1757; updated for #1785 Phase 5/6). The
// marker refresh cookie is a non-refreshable primitive, so the normal
// /auth/refresh path cannot help here — instead we call POST
// /admin/impersonation/end, which the BE deliberately tolerates being
// called with an EXPIRED impersonation token (it self-validates off the
// Authorization header + the httpOnly marker cookie).
//
// Phase 5 moved start/end onto the back-office plane, so the response is
// now a BackofficeLoginResponse: a fresh BACK-OFFICE access token + the
// rotated CSRF in the X-CSRF-Token header. We write through the
// back-office storage keys (not the tenant ones) so the next /admin/*
// request lands with the right credentials. The stale impersonation
// token in tenant storage is left in place; the next tenant request
// either gets re-issued through the tenant refresh path or 401s into a
// /login redirect — neither matters here, because the page is about to
// hard-redirect anyway.
//
// On failure the operator session is unrecoverable, so we clear BOTH
// planes and bounce to /backoffice/login. Either way this throws — the
// original request's promise must reject because the page is being
// replaced (mirrors refreshAccessToken's failure path). A raw `fetch`
// keeps lib/http.ts free of a layering inversion into features/admin
// (mirrors refreshAccessToken).
async function recoverFromImpersonationExpiry(url: string): Promise<never> {
  if (!impersonationEndInFlight) {
    impersonationEndInFlight = (async () => {
      const endUrl = `${BASE_URL}${IMPERSONATION_END_PATH}`
      const headers = new Headers({ Accept: "application/json" })
      // The expired impersonation token lives in TENANT storage — that's
      // the Authorization header the BE expects to self-validate. CSRF
      // also rides the tenant pair for this call.
      const accessToken = getAccessToken()
      if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`)
      const csrf = getCsrfToken()
      if (csrf) headers.set("X-CSRF-Token", csrf)
      const response = await fetch(endUrl, {
        method: "POST",
        credentials: "include",
        cache: "no-store",
        headers,
      })
      if (!response.ok) {
        clearAuth()
        clearBackofficeAuth()
        if (shouldRedirectFromCurrentPath()) {
          navigateToLogin(currentReturnTo(), "session_expired", "backoffice")
        }
        throw new HttpError(
          `Impersonation-end recovery failed with ${response.status}`,
          response.status,
          endUrl,
          await parseBody(response).catch(() => null)
        )
      }
      const payload = (await response.json()) as ImpersonationEndResponse
      // A 2xx without an access token is a malformed response — it cannot
      // restore the operator session, so do the same terminal fallback as
      // the non-ok branch instead of falling through as "success" (which
      // would keep the expired impersonation token, clear the return slot,
      // and hard-redirect into a reload/401 loop). Mirrors the fail-fast
      // guards in startImpersonation / endImpersonation.
      if (!payload.access_token) {
        clearAuth()
        clearBackofficeAuth()
        if (shouldRedirectFromCurrentPath()) {
          navigateToLogin(currentReturnTo(), "session_expired", "backoffice")
        }
        throw new HttpError(
          "Impersonation-end recovery returned no access_token",
          response.status,
          endUrl,
          payload
        )
      }
      // Write the operator's restored credentials through the BACK-OFFICE
      // storage keys (Phase 5/6). The CSRF lands via the X-CSRF-Token
      // response header — read it explicitly here since this helper does
      // its own raw fetch and bypasses performRequest's header-rotation
      // path.
      setBackofficeAccessToken(payload.access_token)
      const newCsrf = response.headers.get("X-CSRF-Token") ?? response.headers.get("x-csrf-token")
      if (newCsrf) setBackofficeCsrfToken(newCsrf)
      // Read the return target BEFORE clearing the slot. With no stored
      // target, fall back to the admin landing route.
      const targetUserId = getImpersonationReturn()?.targetUserId
      clearImpersonationReturn()
      hardRedirect(
        targetUserId ? `/admin/users/${encodeURIComponent(targetUserId)}` : "/admin/tenants"
      )
    })()
  }
  try {
    await impersonationEndInFlight
  } finally {
    impersonationEndInFlight = null
  }
  // The page is being replaced — reject the original request's promise.
  throw new HttpError("Impersonation session ended", 401, url, null)
}

function shouldRedirectFromCurrentPath(): boolean {
  if (typeof window === "undefined") return false
  return !PUBLIC_PATHS.some((p) => window.location.pathname.startsWith(p))
}

function currentReturnTo(): string {
  if (typeof window === "undefined") return "/"
  return window.location.pathname + window.location.search
}

async function handle401(
  url: string,
  originalPath: string,
  init: HttpRequestInit,
  response: Response
): Promise<HttpResponse<unknown>> {
  // Background /auth/me probes during boot must not clear auth or redirect —
  // the legacy frontend's behavior we want to preserve so the user is not
  // bounced to /login on a transient network blip during initial mount.
  if (
    init.authCheck === "background" &&
    (originalPath.startsWith("/auth/me") || originalPath.startsWith("/backoffice/auth/me"))
  ) {
    throw new HttpError("Unauthorized", 401, url, null)
  }
  // For login/register/refresh, a 401 is an application-level error (bad
  // credentials, invalid refresh token) — surface the body so callers can
  // render the actual server message instead of a generic "unauthorized".
  // This check runs BEFORE the impersonation-expiry branch below so that a
  // `skipAuthRefresh` request (including `endImpersonation` itself) always
  // deterministically bypasses impersonation recovery — it must never be
  // ambiguous which 401 path a refresh-opted-out request takes.
  if (NON_REFRESHABLE_AUTH_PATHS.has(originalPath) || init.skipAuthRefresh) {
    const data = await parseBody(response).catch(() => null)
    throw new HttpError("Unauthorized", 401, url, data)
  }
  // Impersonation auto-expiry (#1757): if a return-slot is recorded the
  // browser is inside an impersonation session. When its short-lived
  // access token expires a normal /auth/refresh cannot help — the marker
  // refresh cookie is non-refreshable — so recover the admin session via
  // POST /admin/impersonation/end instead. The `skipAuthRefresh` /
  // non-refreshable check above already excludes the `end` call and any
  // refresh-opted-out request; the explicit `IMPERSONATION_END_PATH` guard
  // here is belt-and-suspenders in case `end` is ever issued without
  // `skipAuthRefresh`.
  //
  // This check stays plane-agnostic on purpose: an impersonation session
  // is a tenant-plane construct (the impersonated identity is a tenant
  // user), but the operator-side recovery via POST /admin/impersonation/end
  // restores the BACK-OFFICE plane (Phase 5 moved start/end onto the
  // back-office plane). The recovery helper writes back-office tokens
  // through `endImpersonation`, which is fine — the impersonated-tenant 401
  // and the back-office-plane recovery share the same upstream control flow.
  if (getImpersonationReturn() !== null && originalPath !== IMPERSONATION_END_PATH) {
    return recoverFromImpersonationExpiry(url)
  }
  const backoffice = isBackofficePath(originalPath)
  // Capture whether the plane that just 401'd actually had a session BEFORE
  // attempting refresh — the refresh failure path below clears the tokens,
  // so checking after would always be false.
  //
  // The bounce to the plane's login screen is only correct when there *was*
  // a session that expired. A tenant-only user touching a back-office-gated
  // path (e.g. `<ImpersonationProvider>` in the tenant Shell probing
  // /admin/impersonation/current after #1838 hardened /admin/* on the
  // back-office plane) has no back-office tokens at all — bouncing them to
  // /backoffice/login on every page render is a regression. Let those
  // callers' onError handle the 401 quietly instead.
  const hadBackofficeSession = !!getBackofficeAccessToken()
  const hadTenantSession = !!getAccessToken()
  try {
    if (backoffice) {
      await refreshBackofficeAccessToken()
    } else {
      await refreshAccessToken()
    }
  } catch (refreshErr) {
    if (backoffice) {
      clearBackofficeAuth()
      if (hadBackofficeSession && shouldRedirectFromCurrentPath()) {
        navigateToLogin(currentReturnTo(), "session_expired", "backoffice")
      }
    } else {
      clearAuth()
      if (hadTenantSession && shouldRedirectFromCurrentPath()) {
        navigateToLogin(currentReturnTo(), "session_expired")
      }
    }
    throw refreshErr instanceof HttpError
      ? refreshErr
      : new HttpError("Refresh failed", 401, url, refreshErr)
  }
  return performRequest(originalPath, { ...init, skipAuthRefresh: true })
}

async function performRequest<T = unknown>(
  path: string,
  init: HttpRequestInit
): Promise<HttpResponse<T>> {
  const method = (init.method ?? "GET") as HttpMethod
  const url = buildUrl(path, init.skipGroupRewrite)
  const headers = buildHeaders(method, path, init)
  const body =
    init.body === undefined || method === "GET"
      ? undefined
      : typeof init.body === "string" || init.body instanceof FormData
        ? init.body
        : JSON.stringify(init.body)
  const response = await fetch(url, {
    method,
    headers,
    body,
    signal: init.signal,
    credentials: "include",
    // `no-store` bypasses the browser's HTTP cache for both reads and
    // writes. The backend doesn't set Cache-Control on JSON:API
    // responses, so without this WebKit's heuristic-freshness cache
    // serves stale GETs on `page.reload()` (#1650 / webkit e2e flake
    // matrix: tags usage, user-isolation, warranties). React Query
    // already owns the in-memory cache layer and handles invalidation,
    // so deferring HTTP caching entirely is the right contract — the
    // browser cache adds no value on top of it for API data, only
    // staleness bugs.
    cache: "no-store",
  })
  // Backend may rotate the CSRF token on any response — pick it up. The
  // rotated value lands in whichever plane the request belonged to so the
  // tenant and back-office CSRF tokens never bleed into each other (the BE
  // signs them with plane-specific keys; a swap would just cause the next
  // mutation to fail CSRF verification).
  const newCsrf = response.headers.get("X-CSRF-Token") ?? response.headers.get("x-csrf-token")
  if (newCsrf) {
    if (isBackofficePath(path)) {
      setBackofficeCsrfToken(newCsrf)
    } else {
      setCsrfToken(newCsrf)
    }
  }

  if (response.status === 401) {
    return (await handle401(url, path, init, response)) as HttpResponse<T>
  }
  const data = (await parseBody(response)) as T
  if (response.status === 503) {
    // BE convention (#1542): a 503 means the API is in scheduled maintenance
    // or otherwise unreachable. Bounce the user to /maintenance with the
    // Retry-After + X-Maintenance-Status headers carried as URL params so a
    // refresh keeps showing the page. The actual HttpError still propagates
    // so any onError that wanted to react (e.g. revalidation queries) can.
    //
    // Exception (#1720 / #1835): some endpoints use 503 to carry a typed
    // product-level error (e.g. `commodity_scan.provider_disabled` when the
    // AI vision provider is intentionally turned off — a per-feature error,
    // not an infra outage). Those responses carry a JSON:API errors[].code
    // that the feature handler already maps to an inline banner; bouncing
    // the whole shell to /maintenance hides that banner. Skip the global
    // bounce when the response body looks like a typed product error.
    if (
      typeof window !== "undefined" &&
      !window.location.pathname.startsWith("/maintenance") &&
      !hasTypedProductError(data)
    ) {
      navigateToMaintenance({
        retryAfter: response.headers.get("Retry-After"),
        componentStatus: response.headers.get("X-Maintenance-Status"),
      })
    }
  }
  if (!response.ok) {
    throw new HttpError(
      `Request to ${url} failed with ${response.status}`,
      response.status,
      url,
      data
    )
  }
  return { data, response, status: response.status }
}

async function send<T>(path: string, init: HttpRequestInit): Promise<T> {
  const { data } = await performRequest<T>(path, init)
  return data
}

export const http = {
  request: performRequest,
  get: <T = unknown>(
    path: string,
    init: Omit<HttpRequestInit, "method" | "body"> = {}
  ): Promise<T> => send<T>(path, { ...init, method: "GET" }),
  post: <T = unknown>(
    path: string,
    body?: unknown,
    init: Omit<HttpRequestInit, "method"> = {}
  ): Promise<T> => send<T>(path, { ...init, method: "POST", body }),
  put: <T = unknown>(
    path: string,
    body?: unknown,
    init: Omit<HttpRequestInit, "method"> = {}
  ): Promise<T> => send<T>(path, { ...init, method: "PUT", body }),
  patch: <T = unknown>(
    path: string,
    body?: unknown,
    init: Omit<HttpRequestInit, "method"> = {}
  ): Promise<T> => send<T>(path, { ...init, method: "PATCH", body }),
  del: <T = unknown>(
    path: string,
    init: Omit<HttpRequestInit, "method" | "body"> = {}
  ): Promise<T> => send<T>(path, { ...init, method: "DELETE" }),
}

// Test-only: reset module state between cases.
export function __resetHttpForTests(): void {
  refreshInFlight = null
  backofficeRefreshInFlight = null
  impersonationEndInFlight = null
}
