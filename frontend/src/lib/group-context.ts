// Single source of truth for the active group slug used by the HTTP client to
// rewrite /api/v1/<resource> → /api/v1/g/{slug}/<resource>. The router (issue
// #1404) sets this when the /g/:groupSlug/* route is active and clears it
// when the user is on a non-group route (/profile, /login, /no-group, etc.).
//
// Kept as a module-level slot rather than a React context so non-React code
// (the http client, codegen helpers) can read it without subscribing.
let currentGroupSlug: string | null = null

// Extract the active group slug from the current URL. Per the GroupProvider
// docstring the URL is the canonical source of truth; the module-level slot
// is just a non-React mirror that gets written from a useEffect.
//
// Two URL shapes carry the slug:
//   - `/g/<slug>/...` — canonical for inventory pages (path wins always).
//   - `?g=<slug>`     — path-clean routes (/profile, /settings) pin the
//                       active group via this query param (#1612 / #1613).
//
// Reading both here closes the same effect-timing race for the query-param
// shape that path-shape already enjoyed: setSearchParams runs synchronously
// (`history.replaceState`) so window.location.search is up-to-date the
// instant /profile auto-fills `?g=`; the slug-mirror useEffect, by contrast,
// lands one tick later — long after the post-render React-Query refetch that
// `enabled` flipping to true just scheduled. Without this fallback that
// first fetch would 404 against `/commodities` (no `/g/{slug}/` prefix),
// leaving the snapshot stuck on the "—" placeholder until the user clicks
// somewhere else (see #1679).
function readSlugFromUrl(): string | null {
  if (typeof window === "undefined") return null
  const path = window.location.pathname
  const m = path.match(/^\/g\/([^/]+)(?:\/|$)/)
  if (m) return decodeURIComponent(m[1])
  const search = new URLSearchParams(window.location.search)
  const queryParamSlug = search.get("g")
  return queryParamSlug && queryParamSlug !== "" ? queryParamSlug : null
}

// URL wins over the slot whenever the path carries a /g/:slug/* prefix. Two
// effect-timing races are closed by this preference:
//
//  1. First render of /g/:slug/* — useEffect commits run after the same
//     render's child queries register their fetches, so a query firing in
//     render N may read a slot that hasn't been mirrored yet.
//
//  2. URL-shape reconciliation — when GroupProvider navigate()s away from a
//     slug the user isn't a member of (URL /g/<wrong>/* → replace to
//     /g/<right>/*), `replaceState` updates window.location synchronously,
//     but the slot still holds the OLD slug from the previous mirror
//     effect. A query that fires in the same effect tick (e.g. React-Query
//     refetch triggered by `enabled` flipping false→true once `currentGroup`
//     resolves) reads the stale slot and 404s against the wrong group.
//     Preferring the URL closes the gap because window.location is already
//     authoritative by the time the query fires.
//
// On routes whose path doesn't carry a slug (/profile, /settings,
// /groups/:id/*) the URL fallback returns null and the slot wins — that's
// the only case where the slot is load-bearing (it carries the active group
// pinned via `?g=<slug>` or via `currentGroup.slug` for id-keyed routes).
export function getCurrentGroupSlug(): string | null {
  const fromUrl = readSlugFromUrl()
  if (fromUrl) return fromUrl
  return currentGroupSlug
}

export function setCurrentGroupSlug(slug: string | null): void {
  currentGroupSlug = slug
}

// Test-only: reset between cases. Production code should use setCurrentGroupSlug.
export function __resetGroupContextForTests(): void {
  currentGroupSlug = null
}
