// Bridge for the anonymous "add your first item before you have an account"
// flow (#1988). An unauthenticated visitor fills the create dialog on the
// landing page; on save we stash the draft (already in localStorage +
// IndexedDB via the dialog's own draft machinery) and set THIS marker, then
// send them to register (the anonymous fill is new-user onboarding). After
// they create an account and sign in, FirstItemResolver reads the marker,
// POSTs the stashed commodity into the resolved group, uploads its pending
// files, and clears everything.
//
// Scope is intentionally localStorage (not sessionStorage like the invite
// bridge): the draft values + the IndexedDB pending files both live in
// localStorage/IDB, and the marker must survive the full login round-trip —
// including an OAuth redirect that can replace the tab. The consumer MUST
// clear it after use, and must tolerate a stale marker whose draft is gone
// (e.g. the user cleared site data between sessions) by falling through.

const STORAGE_KEY = "inventario_pending_first_item"

export interface PendingFirstItem {
  // The localStorage draft key the dialog persisted the form values under
  // (the fixed anonymous key, e.g. "commodity-draft:anon:create"). The same
  // key also addresses the IndexedDB pending-files entry.
  draftKey: string
  // ISO-4217 currency inferred client-side at stash time. Used to seed the
  // auto-created "Main" group when the user has none, so the stashed prices
  // stay coherent. The resolver re-runs toRequest with the real group
  // currency, so a wrong guess is non-fatal.
  currency: string
  // Epoch millis, informational (lets the consumer ignore very stale markers
  // if it ever wants to).
  savedAt: number
}

function safeStorage(): Storage | null {
  if (typeof window === "undefined") return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

export function savePendingFirstItem(item: PendingFirstItem): void {
  const s = safeStorage()
  if (!s) return
  try {
    s.setItem(STORAGE_KEY, JSON.stringify(item))
  } catch {
    // Quota / serialization errors must not break the redirect-to-login flow.
  }
}

export function peekPendingFirstItem(): PendingFirstItem | null {
  const s = safeStorage()
  if (!s) return null
  const raw = s.getItem(STORAGE_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as PendingFirstItem
    // Validate the full shape, not just draftKey: a malformed/partial marker
    // (hand-edited storage, a schema change between releases) must not leak
    // into the resolver, which seeds a group currency from `currency` and may
    // reason about `savedAt`.
    if (
      !parsed ||
      typeof parsed.draftKey !== "string" ||
      !parsed.draftKey ||
      typeof parsed.currency !== "string" ||
      !parsed.currency ||
      typeof parsed.savedAt !== "number" ||
      !Number.isFinite(parsed.savedAt)
    ) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

export function clearPendingFirstItem(): void {
  safeStorage()?.removeItem(STORAGE_KEY)
}

export function consumePendingFirstItem(): PendingFirstItem | null {
  const peek = peekPendingFirstItem()
  clearPendingFirstItem()
  return peek
}
