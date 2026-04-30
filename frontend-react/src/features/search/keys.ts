import type { SearchableType } from "./api"

export const searchKeys = {
  all: ["search"] as const,
  // Per (groupSlug, type, query, limit) cache slot. groupSlug is part of
  // the key because lib/http rewrites `/search` → `/g/<slug>/search`, so
  // two groups produce different network calls; without slug-scoping the
  // cache they would collide and one group could read another's results.
  // Splitting by type lets the grouped page keep one section in flight
  // while another resolves — they're independent network calls.
  query: (groupSlug: string, type: SearchableType, q: string, limit: number) =>
    [...searchKeys.all, groupSlug, type, q, limit] as const,
}
