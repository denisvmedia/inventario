import type { ListExportsOptions } from "./api"

function listKeySuffix(opts: ListExportsOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.includeDeleted) params.set("include_deleted", "true")
  return params.toString()
}

// TanStack Query keys for the exports / restores slice. Scoped by group
// slug because the http client rewrites /exports -> /g/{slug}/exports;
// without the slug in the key, navigating between groups would reuse
// the wrong cache.
export const exportKeys = {
  all: ["export"] as const,
  group: (slug: string) => [...exportKeys.all, slug] as const,
  list: (slug: string, opts?: ListExportsOptions) =>
    [...exportKeys.group(slug), "list", listKeySuffix(opts)] as const,
  detail: (slug: string, id: string) => [...exportKeys.group(slug), "detail", id] as const,
  restoreList: (slug: string, exportId: string) =>
    [...exportKeys.group(slug), "detail", exportId, "restores"] as const,
  restoreDetail: (slug: string, exportId: string, restoreId: string) =>
    [...exportKeys.group(slug), "detail", exportId, "restores", restoreId] as const,
}
