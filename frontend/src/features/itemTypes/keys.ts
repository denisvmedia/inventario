// TanStack Query keys for the item types slice. Scoped by group slug,
// because the http client rewrites /item-types to /g/{slug}/item-types.
export const itemTypeKeys = {
  all: ["itemType"] as const,
  group: (slug: string) => [...itemTypeKeys.all, slug] as const,
  list: (slug: string) => [...itemTypeKeys.group(slug), "list"] as const,
}
