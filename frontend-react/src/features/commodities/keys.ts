// TanStack Query keys for the commodities slice. Scoped by group slug
// because the http client rewrites /commodities -> /g/{slug}/commodities
// — without the slug in the key, navigating from /g/household to
// /g/office would reuse cached household data while the http call goes
// to office, and the mismatch would only resolve on the next refetch.
// Embedding the slug means the cache treats the two groups as separate
// entries the way the URL does.
//
// `group(slug)` is the parent prefix; `list(slug)` and `values(slug)`
// extend it. Pass the slug from `useCurrentGroup()`; for tests with no
// group context, callers can pass an empty string.
export const commodityKeys = {
  all: ["commodity"] as const,
  group: (slug: string) => [...commodityKeys.all, slug] as const,
  list: (slug: string) => [...commodityKeys.group(slug), "list"] as const,
  values: (slug: string) => [...commodityKeys.group(slug), "values"] as const,
}
