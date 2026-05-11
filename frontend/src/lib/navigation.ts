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
export type NavigateToLogin = (currentPath: string, reason?: string) => void

const defaultNavigateToLogin: NavigateToLogin = (currentPath, reason) => {
  // Intentionally NOT a hard-reload via window.location.href anymore. If the
  // SPA navigator hasn't been installed yet (provider hasn't mounted, or
  // an unmount fired in tests / Strict Mode), do nothing visible — the next
  // page render via the router will pick the user up. Logging keeps the
  // path observable in dev tools without yanking the user out of the
  // current view.
  console.warn("[navigation] navigateToLogin called before SPA navigator was installed", {
    currentPath,
    reason,
  })
}

let navigator: NavigateToLogin = defaultNavigateToLogin

export function navigateToLogin(currentPath: string, reason?: string): void {
  navigator(currentPath, reason)
}

export function setNavigateToLogin(fn: NavigateToLogin): void {
  navigator = fn
}

// Test-only: restore the default navigator between cases.
export function __resetNavigationForTests(): void {
  navigator = defaultNavigateToLogin
}
