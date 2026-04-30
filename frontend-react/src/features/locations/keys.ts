// TanStack Query keys for the locations slice. Scoped by group slug
// for the same reason as commodities (#1408): the http client rewrites
// /locations -> /g/{slug}/locations, so without the slug embedded in
// the key, switching groups would reuse a stale cache while the network
// fetched fresh data.
export const locationKeys = {
  all: ["location"] as const,
  group: (slug: string) => [...locationKeys.all, slug] as const,
  list: (slug: string) => [...locationKeys.group(slug), "list"] as const,
  detail: (slug: string, id: string) => [...locationKeys.group(slug), "detail", id] as const,
}
