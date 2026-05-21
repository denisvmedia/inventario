// Tiny fetch wrapper used by every feature slice via TanStack Query.
//
// Behaviors (see issue #1403):
//   - JSON:API content type
//   - Bearer + CSRF
//   - Group-scoped URL rewriting (/api/v1/<resource> → /api/v1/g/{slug}/<resource>)
//   - 401 → access-token refresh via httpOnly refresh cookie, with single-flight
//     deduplication; on refresh failure, clear auth and redirect to /login
//   - Surfaces non-2xx as `HttpError` so React Query can react via onError
import {
  clearAuth,
  clearImpersonationReturn,
  getAccessToken,
  getCsrfToken,
  getImpersonationReturn,
  setAccessToken,
  setCsrfToken,
} from "./auth-storage"
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
// refresh-and-retry loop on these.
const NON_REFRESHABLE_AUTH_PATHS = new Set(["/auth/login", "/auth/register", "/auth/refresh"])

// Routes the user might already be on when a 401 fires; redirecting from them
// would either be a no-op (already at /login) or interrupt a flow that
// intentionally allows unauthenticated access.
const PUBLIC_PATHS = ["/login", "/register", "/verify-email", "/reset-password", "/invite"]

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
// promise so the backend sees one /auth/refresh call, not N.
let refreshInFlight: Promise<string> | null = null

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

function buildHeaders(method: HttpMethod, init: HttpRequestInit): Headers {
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
  const accessToken = getAccessToken()
  if (accessToken) {
    headers.set("Authorization", `Bearer ${accessToken}`)
  }
  if (MUTATING_METHODS.has(method)) {
    const csrf = getCsrfToken()
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

interface ImpersonationEndResponse {
  access_token?: string
  csrf_token?: string
}

// Recovers the admin session when an impersonation access token has
// auto-expired (#1757). The marker refresh cookie is a non-refreshable
// primitive, so the normal /auth/refresh path cannot help here — instead
// we call POST /admin/impersonation/end, which the BE deliberately
// tolerates being called with an EXPIRED impersonation token (it
// self-validates off the Authorization header + the httpOnly marker
// cookie). On success the admin's fresh tokens are stored and the browser
// hard-redirects back to the impersonated user's admin detail page; on
// failure the session is unrecoverable so we clear auth and bounce to
// /login. Either way this throws — the original request's promise must
// reject because the page is being replaced (mirrors refreshAccessToken's
// failure path). A raw `fetch` keeps lib/http.ts free of a layering
// inversion into features/admin (mirrors refreshAccessToken).
async function recoverFromImpersonationExpiry(url: string): Promise<never> {
  if (!impersonationEndInFlight) {
    impersonationEndInFlight = (async () => {
      const endUrl = `${BASE_URL}${IMPERSONATION_END_PATH}`
      const headers = new Headers({ Accept: "application/json" })
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
        if (shouldRedirectFromCurrentPath()) {
          navigateToLogin(currentReturnTo(), "session_expired")
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
      // restore the admin session, so do the same terminal fallback as the
      // non-ok branch instead of falling through as "success" (which would
      // keep the expired impersonation token, clear the return slot, and
      // hard-redirect into a reload/401 loop). Mirrors the fail-fast guards
      // in startImpersonation / endImpersonation.
      if (!payload.access_token) {
        clearAuth()
        if (shouldRedirectFromCurrentPath()) {
          navigateToLogin(currentReturnTo(), "session_expired")
        }
        throw new HttpError(
          "Impersonation-end recovery returned no access_token",
          response.status,
          endUrl,
          payload
        )
      }
      setAccessToken(payload.access_token)
      if (payload.csrf_token) setCsrfToken(payload.csrf_token)
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
  if (init.authCheck === "background" && originalPath.startsWith("/auth/me")) {
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
  if (getImpersonationReturn() !== null && originalPath !== IMPERSONATION_END_PATH) {
    return recoverFromImpersonationExpiry(url)
  }
  try {
    await refreshAccessToken()
  } catch (refreshErr) {
    clearAuth()
    if (shouldRedirectFromCurrentPath()) {
      navigateToLogin(currentReturnTo(), "session_expired")
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
  const headers = buildHeaders(method, init)
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
  // Backend may rotate the CSRF token on any response — pick it up.
  const newCsrf = response.headers.get("X-CSRF-Token") ?? response.headers.get("x-csrf-token")
  if (newCsrf) setCsrfToken(newCsrf)

  if (response.status === 401) {
    return (await handle401(url, path, init, response)) as HttpResponse<T>
  }
  if (response.status === 503) {
    // BE convention (#1542): a 503 means the API is in scheduled maintenance
    // or otherwise unreachable. Bounce the user to /maintenance with the
    // Retry-After + X-Maintenance-Status headers carried as URL params so a
    // refresh keeps showing the page. The actual HttpError still propagates
    // so any onError that wanted to react (e.g. revalidation queries) can.
    if (typeof window !== "undefined" && !window.location.pathname.startsWith("/maintenance")) {
      navigateToMaintenance({
        retryAfter: response.headers.get("Retry-After"),
        componentStatus: response.headers.get("X-Maintenance-Status"),
      })
    }
  }
  const data = (await parseBody(response)) as T
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
  impersonationEndInFlight = null
}
