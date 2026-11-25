// The Items list layout picked from its toolbar is kept for this device under
// this key, and wins over the account's Default view setting
// (appearance.default_items_view) on this device.
export const COMMODITIES_VIEW_MODE_KEY = "commodities:viewMode"

/**
 * Drops the toolbar's layout on this device, so the Items list opens with
 * the Default view setting again. Settings calls it when that setting
 * changes.
 */
export function forgetCommoditiesViewMode(): void {
  try {
    localStorage.removeItem(COMMODITIES_VIEW_MODE_KEY)
  } catch {
    // Storage can be unavailable (private mode, blocked site data); then
    // nothing was stored either.
  }
}
