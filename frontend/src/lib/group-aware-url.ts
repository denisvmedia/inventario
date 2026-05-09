// Append `?g=<slug>` to a URL when there's an active group, leaving the
// path otherwise untouched. The query param is the canonical signal for
// "active group context" on routes whose path doesn't already encode it
// (/profile, /settings, /groups/new, …) — see GroupContext for how it's
// resolved. The path-prefix shape `/g/:slug/*` and `/groups/:id/*` win
// over `?g=`; on those routes the query is redundant and may be omitted.
//
// Returns the original `path` when `slug` is falsy (anonymous user, or
// user-with-zero-groups state) so the link still resolves.
export function withGroupQuery(path: string, slug: string | null | undefined): string {
  if (!slug) return path
  const sep = path.includes("?") ? "&" : "?"
  return `${path}${sep}g=${encodeURIComponent(slug)}`
}
