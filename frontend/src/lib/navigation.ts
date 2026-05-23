// Navigation hook used by the HTTP client to redirect to /login on a 401 that
// can not be recovered by a refresh. The AuthProvider installs a
// react-router-aware navigator on mount (issue #1404); until that runs we
// noop + warn rather than reaching for `window.location.href`. A naked
// location-href would cause a full page reload — the original sin behind
// the "form vanishes after a long idle" bug we hit on mobile Chrome (see
// CommodityFormDialog notes, query-client.ts). Pre-provider 401s are rare
// (boot-time `/auth/me` probe handles its own retry); if one slips through
// it stays on the current view and the next `setNavigateToLogin` install
// from AuthProvider takes over for subsequent calls.
//
// `plane` (#1785 Phase 6) routes the redirect to /backoffice/login vs the
// tenant /login depending on which plane's 401 fired. Default "tenant"
// preserves backward-compat for every existing call site.
export type AuthPlane = "tenant" | "backoffice"
export type NavigateToLogin = (currentPath: string, reason?: string, plane?: AuthPlane) => void

const defaultNavigateToLogin: NavigateToLogin = (currentPath, reason, plane) => {
  // Intentionally NOT a hard-reload via window.location.href anymore. If the
  // SPA navigator hasn't been installed yet (provider hasn't mounted, or
  // an unmount fired in tests / Strict Mode), do nothing visible — the next
  // page render via the router will pick the user up. Logging keeps the
  // path observable in dev tools without yanking the user out of the
  // current view.
  console.warn("[navigation] navigateToLogin called before SPA navigator was installed", {
    currentPath,
    reason,
    plane,
  })
}

let navigator: NavigateToLogin = defaultNavigateToLogin

export function navigateToLogin(currentPath: string, reason?: string, plane?: AuthPlane): void {
  navigator(currentPath, reason, plane)
}

export function setNavigateToLogin(fn: NavigateToLogin): void {
  navigator = fn
}

// Maintenance redirect — installed by AuthProvider so the http client's
// 503-handler can bounce to /maintenance without reaching for
// window.location.href (same reasoning as navigateToLogin above).
//
// retryAfter is parsed from the standard `Retry-After` HTTP header (RFC 9110):
// the BE may send either an HTTP-date or a delta-seconds value. We pass it
// through verbatim as a string; the maintenance page resolves it to a local
// time. componentStatus is sourced from an optional
// `X-Maintenance-Status: api=degraded,database=maintenance,storage=operational`
// header so an outage that only affects part of the stack can still
// communicate the surviving components without bouncing every request
// (#1542 item 1 / design-audit #1527).
export interface MaintenanceContext {
  retryAfter: string | null
  componentStatus: string | null
}

export type NavigateToMaintenance = (ctx: MaintenanceContext) => void

const defaultNavigateToMaintenance: NavigateToMaintenance = (ctx) => {
  console.warn("[navigation] navigateToMaintenance called before SPA navigator was installed", ctx)
}

let maintenanceNavigator: NavigateToMaintenance = defaultNavigateToMaintenance

export function navigateToMaintenance(ctx: MaintenanceContext): void {
  maintenanceNavigator(ctx)
}

export function setNavigateToMaintenance(fn: NavigateToMaintenance): void {
  maintenanceNavigator = fn
}

// Hard, full-page redirect (#1757). Unlike navigateToLogin — which is a
// soft SPA navigation deliberately avoiding a reload — the impersonation
// start/end flows REQUIRE a full document reload so the new identity
// (admin ⇄ target user) takes effect cleanly: every in-memory cache,
// context, and query observer is rebuilt from scratch. The default impl
// is `window.location.assign`; jsdom cannot navigate, so tests override
// it via `setHardRedirect` (mirrors the `setNavigateToLogin` indirection).
export type HardRedirect = (path: string) => void

const defaultHardRedirect: HardRedirect = (path) => {
  if (typeof window !== "undefined") {
    window.location.assign(path)
  }
}

let hardRedirector: HardRedirect = defaultHardRedirect

export function hardRedirect(path: string): void {
  hardRedirector(path)
}

export function setHardRedirect(fn: HardRedirect): void {
  hardRedirector = fn
}

// Test-only: restore the default navigators between cases.
export function __resetNavigationForTests(): void {
  navigator = defaultNavigateToLogin
  maintenanceNavigator = defaultNavigateToMaintenance
  hardRedirector = defaultHardRedirect
}
