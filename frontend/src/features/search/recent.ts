// Client-side cache of the search results a user opened, from the search
// page or the command palette, shown when either is empty. Stored in
// localStorage under a scope that names the user and the group
// (recentScope), so two browser tabs at different groups don't pollute
// each other's recents, and the next person to sign in on this browser
// does not see what the last one opened. Signing out clears them all.
//
// Entries are append-with-dedupe + cap at MAX_RECENT, newest first.
// An entry's URL is the in-app path the cards link to; the type lets
// the page render a subtle resource icon next to the title.

const KEY_PREFIX = "inventario_recent_v1:"
const MAX_RECENT = 10

export type RecentResourceType = "commodity" | "location" | "area" | "tag" | "file"

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

// recentScope is the scope of a user's recent items in a group.
export function recentScope(userId: string, groupSlug: string): string {
  return `${userId}:${groupSlug}`
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

// clearAllRecent removes the recent items of every user and group on this
// browser. Signing out calls it.
export function clearAllRecent(): void {
  const s = safeStorage()
  if (!s) return
  try {
    const keys: string[] = []
    for (let i = 0; i < s.length; i++) {
      const key = s.key(i)
      if (key !== null && key.startsWith(KEY_PREFIX)) keys.push(key)
    }
    for (const key of keys) s.removeItem(key)
  } catch {
    // Disabled storage holds nothing to clear.
  }
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
