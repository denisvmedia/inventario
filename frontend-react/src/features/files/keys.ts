import type { ListFilesOptions } from "./api"

// Stringify list-query options into a stable key suffix. URLSearchParams
// preserves multi-value keys; we sort `tags` before serialising so two
// options objects with the same filter set but different array order
// produce identical keys (TanStack Query re-uses the cache instead of
// issuing a duplicate request).
function listKeySuffix(opts: ListFilesOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("limit", String(opts.perPage))
  if (opts.category) params.set("category", opts.category)
  if (opts.type) params.set("type", opts.type)
  if (opts.search?.trim()) params.set("search", opts.search.trim())
  for (const t of [...(opts.tags ?? [])].sort()) params.append("tags", t)
  return params.toString()
}

function countsKeySuffix(
  opts: Omit<ListFilesOptions, "category" | "page" | "perPage"> | undefined
): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.type) params.set("type", opts.type)
  if (opts.search?.trim()) params.set("search", opts.search.trim())
  for (const t of [...(opts.tags ?? [])].sort()) params.append("tags", t)
  return params.toString()
}

// TanStack Query keys for the files slice. Scoped by group slug because
// the http client rewrites /files -> /g/{slug}/files; without the slug
// in the key, navigating between groups would reuse the wrong cache.
export const fileKeys = {
  all: ["file"] as const,
  group: (slug: string) => [...fileKeys.all, slug] as const,
  list: (slug: string, opts?: ListFilesOptions) =>
    [...fileKeys.group(slug), "list", listKeySuffix(opts)] as const,
  categoryCounts: (slug: string, opts?: Omit<ListFilesOptions, "category" | "page" | "perPage">) =>
    [...fileKeys.group(slug), "category-counts", countsKeySuffix(opts)] as const,
  detail: (slug: string, id: string) => [...fileKeys.group(slug), "detail", id] as const,
}
