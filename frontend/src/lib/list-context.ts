// List context: how a record surface knows the list it was opened from.
//
// A page that shows one record (a file's review page, an item page) and was
// opened from a list carries that list's location in its own URL, as
// `?from=<list path>?<list query>`. The record surface reads it back to:
//
//   - return to the same list with the same search, filters, sort, layout and
//     page (Close, Cancel, Save);
//   - walk the list's records in the list's order (Previous, Next, Save and
//     next), when it knows how to query that list. See useListNavigation.
//
// The value lives in the URL rather than in history state so that a reload, a
// copied link or a link opened in a new tab keeps it. A record surface with no
// `from`, or with one it cannot walk, still works: it returns to its own
// fallback and hides Previous / Next.
//
// `from` is navigated to with the in-app router, so it is accepted only as a
// path that starts with a single "/" inside the current group's URL space
// (`/g/<slug>`). Anything else reads as "no list". A list's own `from` is kept
// as part of its query, so a chain of record surfaces unwinds one step at a
// time.
//
// Moving to another record of the list updates the list's `page` parameter to
// the page that holds that record, so returning lands on the page the user
// ended on, not the one they started from.

export const LIST_CONTEXT_PARAM = "from"

// Every paginated list in the app names its page parameter `page`; 1 is the
// default and is left out of the URL.
const PAGE_PARAM = "page"

export interface ListContext {
  /** In-app path of the list, without the query string, e.g. `/g/home/files`. */
  readonly pathname: string
  /** The list's query string, without the leading "?". */
  readonly search: string
}

/** The list context for a location, e.g. the list page's own `useLocation()`. */
export function listContextFromLocation(location: {
  pathname: string
  search: string
}): ListContext {
  return { pathname: location.pathname, search: stripQuestionMark(location.search) }
}

/**
 * Reads `from` out of a record surface's query string. `scope` is the URL
 * space the list must live in, normally `/g/<encoded slug>`; a value outside
 * it is ignored.
 */
export function readListContext(params: URLSearchParams, scope: string): ListContext | null {
  const raw = params.get(LIST_CONTEXT_PARAM)
  return raw ? parseListContext(raw, scope) : null
}

/** Parses a `from` value; null when it is not an in-app path under `scope`. */
export function parseListContext(raw: string, scope: string): ListContext | null {
  const value = raw.trim()
  // A single leading "/" only: "//host" and "/\host" are protocol-relative
  // URLs to the browser and would leave the app.
  if (value[0] !== "/" || value[1] === "/" || value[1] === "\\") return null
  const withoutHash = value.split("#", 1)[0]
  const q = withoutHash.indexOf("?")
  const pathname = q === -1 ? withoutHash : withoutHash.slice(0, q)
  const search = q === -1 ? "" : withoutHash.slice(q + 1)
  if (pathname.includes("\\") || pathname.split("/").includes("..")) return null
  const root = scope.endsWith("/") ? scope.slice(0, -1) : scope
  if (!root || (pathname !== root && !pathname.startsWith(`${root}/`))) return null
  return { pathname, search }
}

/** The URL to navigate to for returning to the list. */
export function listContextHref(ctx: ListContext): string {
  return ctx.search ? `${ctx.pathname}?${ctx.search}` : ctx.pathname
}

/** The list's query parameters, as a fresh object the caller may change. */
export function listContextParams(ctx: ListContext): URLSearchParams {
  return new URLSearchParams(ctx.search)
}

/**
 * Adds `from` to a record surface's URL. `to` may already carry a query
 * string; an existing `from` is replaced. A null context returns `to` as is.
 */
export function withListContext(to: string, ctx: ListContext | null): string {
  if (!ctx) return to
  const hashAt = to.indexOf("#")
  const hash = hashAt === -1 ? "" : to.slice(hashAt)
  const beforeHash = hashAt === -1 ? to : to.slice(0, hashAt)
  const q = beforeHash.indexOf("?")
  const path = q === -1 ? beforeHash : beforeHash.slice(0, q)
  const params = new URLSearchParams(q === -1 ? "" : beforeHash.slice(q + 1))
  params.set(LIST_CONTEXT_PARAM, listContextHref(ctx))
  return `${path}?${params.toString()}${hash}`
}

/**
 * The list page the context points at; 1 when absent or not a positive
 * number. Parsed with parseInt like the list pages parse it, so both agree on
 * a value such as "2x".
 */
export function listContextPage(ctx: ListContext): number {
  const raw = listContextParams(ctx).get(PAGE_PARAM)
  const page = raw ? Number.parseInt(raw, 10) : 1
  return Number.isFinite(page) && page >= 1 ? page : 1
}

/** The same list on another page. Page 1 drops the parameter, as the lists do. */
export function withListContextPage(ctx: ListContext, page: number): ListContext {
  // Leave an unchanged context byte for byte as the list wrote it.
  if (listContextPage(ctx) === Math.max(1, page)) return ctx
  const params = listContextParams(ctx)
  if (page > 1) params.set(PAGE_PARAM, String(page))
  else params.delete(PAGE_PARAM)
  return { pathname: ctx.pathname, search: params.toString() }
}

function stripQuestionMark(search: string): string {
  return search.startsWith("?") ? search.slice(1) : search
}
