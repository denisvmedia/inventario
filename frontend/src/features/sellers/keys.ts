// TanStack Query keys for the sellers slice. Scoped by group slug, because
// the http client rewrites /sellers to /g/{slug}/sellers.
export const sellerKeys = {
  all: ["seller"] as const,
  group: (slug: string) => [...sellerKeys.all, slug] as const,
  list: (slug: string) => [...sellerKeys.group(slug), "list"] as const,
}
