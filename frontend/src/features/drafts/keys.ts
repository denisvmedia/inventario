import type { DraftListOptions } from "./api"

function listKeySuffix(opts: DraftListOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  return params.toString()
}

// TanStack Query keys for server drafts. Scoped by group slug because the
// http client rewrites /drafts -> /g/{slug}/drafts.
export const draftKeys = {
  all: ["drafts"] as const,
  group: (slug: string) => [...draftKeys.all, slug] as const,
  list: (slug: string, opts?: DraftListOptions) =>
    [...draftKeys.group(slug), "list", listKeySuffix(opts)] as const,
  detail: (slug: string, id: string) => [...draftKeys.group(slug), "detail", id] as const,
  files: (slug: string, id: string) => [...draftKeys.group(slug), "files", id] as const,
}
