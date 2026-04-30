// TanStack Query keys for the areas slice. Scoped by group slug to
// keep the cache aligned with the http client's /g/{slug}/ rewrite.
export const areaKeys = {
  all: ["area"] as const,
  group: (slug: string) => [...areaKeys.all, slug] as const,
  list: (slug: string) => [...areaKeys.group(slug), "list"] as const,
  detail: (slug: string, id: string) => [...areaKeys.group(slug), "detail", id] as const,
}
