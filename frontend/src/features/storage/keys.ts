// TanStack Query keys for the storage slice. Scoped by group slug
// because the HTTP client rewrites `/storage-usage` -> `/g/{slug}/storage-usage`;
// without the slug in the key, navigating between groups would reuse
// the wrong cache.
export const storageKeys = {
  all: ["storage"] as const,
  group: (slug: string) => [...storageKeys.all, slug] as const,
  usage: (slug: string) => [...storageKeys.group(slug), "usage"] as const,
}
