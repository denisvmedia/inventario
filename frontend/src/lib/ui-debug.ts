// Manual override for the crash screen's debug panel (#1965).
//
// The panel (error message + stack + component stack) is normally gated
// on `import.meta.env.DEV` (local vite) or the backend `system.debug`
// flag (preview / demo deploys set INVENTARIO_DEBUG_UI=true). This adds a
// per-browser manual switch so a developer can reveal it in ANY
// environment — including production — by appending `?debug=1` to the
// URL. That is deliberate and considered low-risk: the panel only
// surfaces a client-side render error (message + minified stack + React
// component names), never secrets or user data, and a viewer has to opt
// IN for their own browser. It is NOT an admin/auth gate; if a hard
// "no stack in prod, ever" guarantee is later required, drop the
// `isUiDebugOverrideEnabled()` term from the gate in UnexpectedErrorPage.
// The choice sticks in localStorage so it survives the navigations after
// the toggling load; `?debug=0` clears it.
const STORAGE_KEY = "inventario:debug-ui"

function truthy(value: string): boolean {
  return value === "1" || value === "true" || value === "yes" || value === "on"
}

export function isUiDebugOverrideEnabled(): boolean {
  try {
    const param = new URLSearchParams(window.location.search).get("debug")
    if (param !== null) {
      const on = truthy(param.toLowerCase())
      // Persistence is best-effort — a storage failure must NOT change the
      // override decision derived from the URL for this load.
      try {
        if (on) window.localStorage.setItem(STORAGE_KEY, "1")
        else window.localStorage.removeItem(STORAGE_KEY)
      } catch {
        /* keep the URL-derived decision */
      }
      return on
    }
    try {
      return window.localStorage.getItem(STORAGE_KEY) === "1"
    } catch {
      return false
    }
  } catch {
    // No window / URLSearchParams (non-browser context) → no override.
    return false
  }
}
