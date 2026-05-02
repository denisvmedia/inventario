// Client-side "recently visited" cache for the search page's empty
// state. Stored in localStorage scoped per group slug so two browser
// tabs at different groups don't pollute each other's recents.
//
// Entries are append-with-dedupe + cap at MAX_RECENT, newest first.
// An entry's URL is the in-app path the cards link to; the type lets
// the page render a subtle resource icon next to the title.

const KEY_PREFIX = "inventario_recent_v1:"
const MAX_RECENT = 10

export type RecentResourceType = "commodity" | "location" | "area" | "file"

export interface RecentEntry {
  type: RecentResourceType
  // Stable id from the BE — used for dedupe on push.
  id: string
  title: string
  // Absolute in-app path (not a full URL): "/g/<slug>/commodities/<id>".
  url: string
  // Epoch milliseconds; helps the page sort + dim very-old entries if
  // we ever want to.
  visitedAt: number
}

function safeStorage(): Storage | null {
  if (typeof window === "undefined") return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

function storageKey(scope: string): string {
  return `${KEY_PREFIX}${scope || "default"}`
}

export function getRecent(scope: string): RecentEntry[] {
  const s = safeStorage()
  if (!s) return []
  const raw = s.getItem(storageKey(scope))
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(isValidEntry)
  } catch {
    return []
  }
}

export function pushRecent(scope: string, entry: Omit<RecentEntry, "visitedAt">): void {
  const s = safeStorage()
  if (!s) return
  const visitedAt = Date.now()
  const next: RecentEntry = { ...entry, visitedAt }
  const existing = getRecent(scope).filter((e) => !(e.type === entry.type && e.id === entry.id))
  const merged = [next, ...existing].slice(0, MAX_RECENT)
  try {
    s.setItem(storageKey(scope), JSON.stringify(merged))
  } catch {
    // Quota / serialization failures: silently drop. Recent items are
    // a quality-of-life surface; we don't want to break navigation
    // because the user filled localStorage.
  }
}

export function clearRecent(scope: string): void {
  safeStorage()?.removeItem(storageKey(scope))
}

function isValidEntry(value: unknown): value is RecentEntry {
  if (!value || typeof value !== "object") return false
  const v = value as Record<string, unknown>
  return (
    typeof v.type === "string" &&
    typeof v.id === "string" &&
    typeof v.title === "string" &&
    typeof v.url === "string" &&
    typeof v.visitedAt === "number"
  )
}
