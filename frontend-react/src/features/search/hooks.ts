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
  // Active group slug. Required for the cache key (so group-A and
  // group-B never share a slot) and as a readiness gate — until
  // GroupProvider has settled the slug, lib/http would rewrite the
  // request to a stale `/g/<old>/search` URL.
  groupSlug?: string | null
}

// useSearch is a per-type hook. The grouped page invokes it once per
// resource (commodities, files, locations, areas); the palette only
// hits commodities + files. 501 Not Implemented is folded into a
// `{ unavailable: true }` page by the api wrapper, so the consumer
// renders a "coming soon" stub instead of a transport error.
export function useSearch<TAttrs>(
  query: string,
  type: SearchableType,
  options: UseSearchOptions = {}
) {
  const minChars = options.minChars ?? 1
  const limit = options.limit ?? 5
  const trimmed = query.trim()
  const groupSlug = options.groupSlug ?? null
  const eligible = options.enabled !== false && trimmed.length >= minChars && groupSlug !== null
  return useQuery<SearchPage<TAttrs>>({
    queryKey: searchKeys.query(groupSlug ?? "", type, trimmed, limit),
    queryFn: ({ signal }) => search<TAttrs>(trimmed, { type, limit, signal }),
    enabled: eligible,
    // Don't retry — search is best-effort and a real 5xx from an
    // overloaded BE shouldn't burn three round-trips before the user
    // sees the error state.
    retry: false,
    // Stale-after-30s lets the grouped page keep a recent query warm
    // while the user navigates away and back. Window focus refetch is
    // disabled outright — re-running a keyword search every time the
    // tab regains focus is too aggressive for a best-effort surface.
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
}
