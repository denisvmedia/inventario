import { commodityKeys } from "@/features/commodities/keys"

// TanStack Query keys of the saved views slice. They sit under the
// commodity group key: a change to items moves the counts, and a tag rename
// (which invalidates that key) rewrites the queries of the views naming it,
// so both refetch with the rest of the item data.
export const savedViewKeys = {
  all: (slug: string) => [...commodityKeys.group(slug), "saved-views"] as const,
  list: (slug: string) => [...savedViewKeys.all(slug), "list"] as const,
  counts: (slug: string) => [...savedViewKeys.all(slug), "counts"] as const,
}
