// The inbox's items to review as a list context (lib/list-context.ts): an
// item opened from the inbox carries `?from=/g/<slug>/inbox?reason=item_draft`
// so its page can walk the items that wait for review, one after another, and
// return to the inbox. The walk reads the inbox pages through the same query
// keys as the inbox page, so both share the cached pages.
//
// The walk always follows the Items to review list, also when the item was
// opened from the All tab: the All list starts with the drafts, which are not
// items and cannot be opened on an item page.
import { listPageQuery, type ListPageSource } from "@/hooks/useListNavigation"
import { listContextPage, listContextParams, type ListContext } from "@/lib/list-context"

import { listInbox, type InboxListOptions } from "./api"
import { inboxKeys } from "./keys"

/** Entries per inbox page; the inbox page and the walk use the same size. */
export const INBOX_PAGE_SIZE = 50

const REVIEW_REASON = "item_draft"

export function inboxPath(slug: string): string {
  return `/g/${encodeURIComponent(slug)}/inbox`
}

/** The Items to review list on a page (1-based), as a list context. */
export function inboxReviewContext(slug: string, page = 1): ListContext {
  const params = new URLSearchParams({ reason: REVIEW_REASON })
  if (page > 1) params.set("page", String(page))
  return { pathname: inboxPath(slug), search: params.toString() }
}

/** True when the context is the group's Items to review list. */
export function isInboxReviewContext(ctx: ListContext, slug: string): boolean {
  return (
    !!slug &&
    ctx.pathname === inboxPath(slug) &&
    listContextParams(ctx).get("reason") === REVIEW_REASON
  )
}

/** The options of one page of the Items to review list. */
export function inboxReviewOptions(page: number): InboxListOptions {
  return { reason: REVIEW_REASON, page, perPage: INBOX_PAGE_SIZE }
}

/** The Items to review list behind a context, as a source for useListNavigation. */
export function inboxReviewSource(ctx: ListContext, slug: string): ListPageSource {
  return {
    pageSize: INBOX_PAGE_SIZE,
    page: listContextPage(ctx),
    pageQuery: (page) => {
      const opts = inboxReviewOptions(page)
      return listPageQuery({
        queryKey: inboxKeys.list(slug, opts),
        queryFn: ({ signal }) => listInbox(opts, signal),
        select: (data) => ({ ids: data.entries.map((e) => e.id), total: data.total }),
      })
    },
  }
}

/**
 * The page of the Items to review list that holds the entry at `offset` (0-based)
 * of the All list. The server orders the inbox by each entry's first reason,
 * drafts first, so the items to review follow the `drafts` draft entries in
 * the same order.
 */
export function reviewPageForAllOffset(offset: number, drafts: number): number {
  return Math.floor(Math.max(0, offset - drafts) / INBOX_PAGE_SIZE) + 1
}
