// Single source of truth for the active group slug used by the HTTP client to
// rewrite /api/v1/<resource> → /api/v1/g/{slug}/<resource>. The router (issue
// #1404) sets this when the /g/:groupSlug/* route is active and clears it
// when the user is on a non-group route (/profile, /login, /no-group, etc.).
//
// Kept as a module-level slot rather than a React context so non-React code
// (the http client, codegen helpers) can read it without subscribing.
let currentGroupSlug: string | null = null

export function getCurrentGroupSlug(): string | null {
  return currentGroupSlug
}

export function setCurrentGroupSlug(slug: string | null): void {
  currentGroupSlug = slug
}

// Test-only: reset between cases. Production code should use setCurrentGroupSlug.
export function __resetGroupContextForTests(): void {
  currentGroupSlug = null
}
