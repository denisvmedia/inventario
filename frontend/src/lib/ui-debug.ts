// Manual override for the crash screen's debug panel (#1965).
//
// The panel (error message + stack + component stack) is normally gated
// on `import.meta.env.DEV` (local vite) or the backend `system.debug`
// flag (preview / demo deploys set INVENTARIO_DEBUG_UI=true). This adds
// a per-browser manual switch so a developer can reveal it in ANY
// environment — including one where the backend flag is off — by
// appending `?debug=1` to the URL. The choice sticks in localStorage so
// it survives the navigations after the toggling load; `?debug=0` clears
// it. Reading it never throws (storage / URL access is wrapped).
const STORAGE_KEY = "inventario:debug-ui"

function truthy(value: string): boolean {
  return value === "1" || value === "true" || value === "yes" || value === "on"
}

export function isUiDebugOverrideEnabled(): boolean {
  try {
    const param = new URLSearchParams(window.location.search).get("debug")
    if (param !== null) {
      const on = truthy(param.toLowerCase())
      if (on) window.localStorage.setItem(STORAGE_KEY, "1")
      else window.localStorage.removeItem(STORAGE_KEY)
      return on
    }
    return window.localStorage.getItem(STORAGE_KEY) === "1"
  } catch {
    // Private-mode / disabled storage / non-browser context → no override.
    return false
  }
}
