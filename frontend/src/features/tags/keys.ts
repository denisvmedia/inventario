import type { ListTagsOptions, TagKind } from "./api"

function listKeySuffix(opts: ListTagsOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("limit", String(opts.perPage))
  if (opts.search?.trim()) params.set("search", opts.search.trim())
  if (opts.sort) params.set("sort", opts.sort)
  if (opts.order) params.set("order", opts.order)
  if (opts.includeUsage) params.set("include", "usage")
  if (opts.kind) params.set("kind", opts.kind)
  return params.toString()
}

// TanStack Query keys for the tags slice. Scoped by group slug because
// the http client rewrites /tags -> /g/{slug}/tags; without the slug in
// the key, navigating between groups would reuse the wrong cache.
export const tagKeys = {
  all: ["tag"] as const,
  group: (slug: string) => [...tagKeys.all, slug] as const,
  list: (slug: string, opts?: ListTagsOptions) =>
    [...tagKeys.group(slug), "list", listKeySuffix(opts)] as const,
  stats: (slug: string) => [...tagKeys.group(slug), "stats"] as const,
  detail: (slug: string, id: string) => [...tagKeys.group(slug), "detail", id] as const,
  // autocomplete key includes kind so the per-input cache buckets a
  // commodity-tag query separately from a file-tag one even when q +
  // limit happen to be identical.
  autocomplete: (slug: string, q: string, limit: number, kind: TagKind | undefined) =>
    [...tagKeys.group(slug), "autocomplete", q, limit, kind ?? "none"] as const,
}
