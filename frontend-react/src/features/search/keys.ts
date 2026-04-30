import type { SearchableType } from "./api"

export const searchKeys = {
  all: ["search"] as const,
  // Per (type, query, limit) cache slot. Splitting by type lets the
  // grouped page keep one section in flight while another resolves —
  // they're independent network calls.
  query: (type: SearchableType, q: string, limit: number) =>
    [...searchKeys.all, type, q, limit] as const,
}
