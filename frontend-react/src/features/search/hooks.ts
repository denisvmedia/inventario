import { useQuery } from "@tanstack/react-query"

import { search, type SearchableType, type SearchPage } from "./api"
import { searchKeys } from "./keys"

interface UseSearchOptions {
  // Don't fire below this character count — keeps the BE quiet for one-
  // letter typing in the palette while the user is still writing.
  minChars?: number
  // Forwarded to the BE `limit=`. Default 5 (grouped-page section width);
  // pass 3 for palette previews.
  limit?: number
  // Disables the query without changing the cache key — useful for the
  // palette's debounced query.
  enabled?: boolean
}

// useSearch is a per-type hook. The grouped page invokes it once per
// resource (commodities, files, locations, areas); the palette only
// hits commodities + files. Errors (e.g. 501 Not Implemented for
// areas/locations on older backends) settle to TanStack's `error`
// state and the consumer renders an empty section gracefully.
export function useSearch<TAttrs>(
  query: string,
  type: SearchableType,
  options: UseSearchOptions = {}
) {
  const minChars = options.minChars ?? 1
  const limit = options.limit ?? 5
  const trimmed = query.trim()
  const eligible = options.enabled !== false && trimmed.length >= minChars
  return useQuery<SearchPage<TAttrs>>({
    queryKey: searchKeys.query(type, trimmed, limit),
    queryFn: ({ signal }) => search<TAttrs>(trimmed, { type, limit, signal }),
    enabled: eligible,
    // Don't retry — search is best-effort and a 501 from an unimplemented
    // resource type would otherwise burn three round-trips.
    retry: false,
    // Stale-after-30s: the grouped page can keep a recent query warm
    // while the user navigates away and back; refresh on focus is too
    // aggressive for a keyword search.
    staleTime: 30_000,
  })
}
