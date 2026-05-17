// TanStack Query keys for the supply-links feature slice (#1369).
// Scoped by group slug because the http client rewrites
// /commodities/... -> /g/{slug}/commodities/...; without the slug in
// the key, navigating between groups would reuse the wrong cache.
export const supplyLinkKeys = {
  all: ["supply-link"] as const,
  group: (slug: string) => [...supplyLinkKeys.all, slug] as const,
  byCommodity: (slug: string, commodityID: string) =>
    [...supplyLinkKeys.group(slug), "byCommodity", commodityID] as const,
}
