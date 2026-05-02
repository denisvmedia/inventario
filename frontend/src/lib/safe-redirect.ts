// Sanitizes a `?redirect=` query value into a safe in-app path.
//
// Threat model: the redirect param is supposed to be an internal path written
// by route guards (e.g. `/g/household/items`), but a user-crafted URL could
// point it at `https://evil.example/`, `//evil.example`, or `\\evil.example`.
// Passing those into `<Navigate to={…}>` or `navigate(…)` would happily send
// the user off-domain on react-router 7.
//
// The rule: only accept values that begin with a single `/` and are not
// followed by another `/` or `\`. Everything else falls back to `/`.

const FALLBACK = "/"

export function sanitizeRedirectPath(raw: string | null | undefined): string {
  if (!raw) return FALLBACK
  // Drop any whitespace-only / empty values.
  const value = raw.trim()
  if (!value) return FALLBACK
  // Must start with a single `/` — refuses absolute URLs ("https://…",
  // "//host/…"), protocol-relative paths, and Windows-style "\\host".
  if (value[0] !== "/") return FALLBACK
  if (value[1] === "/" || value[1] === "\\") return FALLBACK
  return value
}
