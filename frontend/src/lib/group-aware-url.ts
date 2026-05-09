// Set `?g=<slug>` on a URL when there's an active group, leaving the path
// (and any existing search/hash) otherwise untouched. The query param is
// the canonical signal for "active group context" on routes whose path
// doesn't already encode it (/profile, /settings, /groups/new, …) — see
// GroupContext for how it's resolved. The path-prefix shape `/g/:slug/*`
// and `/groups/:id/*` win over `?g=`; on those routes the query is
// redundant and may be omitted.
//
// Returns the original `path` when `slug` is falsy (anonymous user, or
// user-with-zero-groups state) so the link still resolves.
//
// Hash-aware (`?` must precede `#`) and de-duplicating: an existing `g=`
// is replaced rather than concatenated, an existing `#fragment` stays at
// the tail, and other query params (including ordering) are preserved.
export function withGroupQuery(path: string, slug: string | null | undefined): string {
  if (!slug) return path
  // Slice off optional `#fragment` first so it can be reattached at the
  // end — the URL spec puts query before hash, and a naive concat against
  // a fragmented path produces e.g. `/help#shortcuts?g=…` which the
  // server / router never parses as a query.
  const hashIndex = path.indexOf("#")
  const hash = hashIndex >= 0 ? path.slice(hashIndex) : ""
  const beforeHash = hashIndex >= 0 ? path.slice(0, hashIndex) : path
  const queryIndex = beforeHash.indexOf("?")
  const pathname = queryIndex >= 0 ? beforeHash.slice(0, queryIndex) : beforeHash
  const queryString = queryIndex >= 0 ? beforeHash.slice(queryIndex + 1) : ""
  // URLSearchParams.set() replaces an existing `g=…` instead of
  // appending, which is what we want — repeated calls with different
  // slugs converge on a single canonical value.
  const params = new URLSearchParams(queryString)
  params.set("g", slug)
  return `${pathname}?${params.toString()}${hash}`
}
