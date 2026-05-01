// Single source of truth for the active group slug used by the HTTP client to
// rewrite /api/v1/<resource> → /api/v1/g/{slug}/<resource>. The router (issue
// #1404) sets this when the /g/:groupSlug/* route is active and clears it
// when the user is on a non-group route (/profile, /login, /no-group, etc.).
//
// Kept as a module-level slot rather than a React context so non-React code
// (the http client, codegen helpers) can read it without subscribing.
let currentGroupSlug: string | null = null

// Extract the active group slug from the current URL. Used as a fallback for
// the slot when a group-scoped query fires before GroupProvider's mirroring
// effect has had a chance to run — which can happen on the very first render
// of a /g/:slug/* route, since useEffect commits run after the same render's
// child queries register their fetches. Per the GroupProvider docstring, the
// URL is the canonical source of truth; reading it here closes the gap
// without reordering effect timing.
function readSlugFromUrl(): string | null {
  if (typeof window === "undefined") return null
  const match = window.location.pathname.match(/^\/g\/([^/]+)(?:\/|$)/)
  return match ? decodeURIComponent(match[1]) : null
}

export function getCurrentGroupSlug(): string | null {
  if (currentGroupSlug) return currentGroupSlug
  return readSlugFromUrl()
}

export function setCurrentGroupSlug(slug: string | null): void {
  currentGroupSlug = slug
}

// Test-only: reset between cases. Production code should use setCurrentGroupSlug.
export function __resetGroupContextForTests(): void {
  currentGroupSlug = null
}
