import { useCallback, useState } from "react"

// The custom fields the items list shows as columns in its list layout
// (#2803), kept on this device per group, by field id: a renamed field
// stays a column, and a deleted one is no longer shown.
export const CUSTOM_FIELD_COLUMNS_MAX = 4

function storageKey(slug: string): string {
  return `commodities:customFieldColumns:${slug}`
}

export function readCustomFieldColumns(slug: string): string[] {
  if (!slug) return []
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(storageKey(slug)) ?? "[]")
    return Array.isArray(parsed)
      ? parsed
          .filter((id): id is string => typeof id === "string")
          .slice(0, CUSTOM_FIELD_COLUMNS_MAX)
      : []
  } catch {
    // Unreadable or unavailable storage shows no columns.
    return []
  }
}

export function storeCustomFieldColumns(slug: string, ids: readonly string[]): void {
  if (!slug) return
  try {
    if (ids.length === 0) localStorage.removeItem(storageKey(slug))
    else localStorage.setItem(storageKey(slug), JSON.stringify(ids))
  } catch {
    // Storage can be unavailable (private mode, blocked site data); the
    // columns then last until the page is left.
  }
}

// useCustomFieldColumns returns the column field ids of slug's list and a
// setter that stores them.
export function useCustomFieldColumns(slug: string): [string[], (ids: string[]) => void] {
  const [state, setState] = useState(() => ({ slug, ids: readCustomFieldColumns(slug) }))
  // A group switch reads the other group's columns.
  const current = state.slug === slug ? state : { slug, ids: readCustomFieldColumns(slug) }
  if (current !== state) setState(current)
  const set = useCallback(
    (ids: string[]) => {
      const next = ids.slice(0, CUSTOM_FIELD_COLUMNS_MAX)
      storeCustomFieldColumns(slug, next)
      setState({ slug, ids: next })
    },
    [slug]
  )
  return [current.ids, set]
}
