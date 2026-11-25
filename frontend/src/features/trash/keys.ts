import type { TrashListOptions } from "./api"

function listKeySuffix(opts: TrashListOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  return params.toString()
}

// TanStack Query keys for the Trash. Scoped by group slug because the http
// client rewrites /trash -> /g/{slug}/trash.
export const trashKeys = {
  all: ["trash"] as const,
  group: (slug: string) => [...trashKeys.all, slug] as const,
  commodities: (slug: string, opts?: TrashListOptions) =>
    [...trashKeys.group(slug), "commodities", listKeySuffix(opts)] as const,
  files: (slug: string, opts?: TrashListOptions) =>
    [...trashKeys.group(slug), "files", listKeySuffix(opts)] as const,
}
