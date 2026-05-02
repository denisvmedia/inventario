// Navigation hook used by the HTTP client to redirect to /login on a 401 that
// can not be recovered by a refresh. The default uses window.location;
// react-router (issue #1404) replaces it with a router-aware navigator that
// preserves the SPA state.
export type NavigateToLogin = (currentPath: string, reason?: string) => void

const defaultNavigateToLogin: NavigateToLogin = (currentPath, reason) => {
  if (typeof window === "undefined") return
  const params = new URLSearchParams({ redirect: currentPath })
  if (reason) params.set("reason", reason)
  window.location.href = `/login?${params.toString()}`
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
