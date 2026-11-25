import { commodityKeys } from "@/features/commodities/keys"

import type { InboxListOptions } from "./api"

function listKeySuffix(opts: InboxListOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.reason) params.set("reason", opts.reason)
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  return params.toString()
}

// TanStack Query keys of the inbox. They sit under the commodity group key:
// creating, editing or deleting an item can add an entry or clear one, and
// every item mutation already invalidates that key. Draft mutations
// invalidate the inbox through useInvalidateDrafts.
export const inboxKeys = {
  all: (slug: string) => [...commodityKeys.group(slug), "inbox"] as const,
  list: (slug: string, opts?: InboxListOptions) =>
    [...inboxKeys.all(slug), "list", listKeySuffix(opts)] as const,
  count: (slug: string) => [...inboxKeys.all(slug), "count"] as const,
}
