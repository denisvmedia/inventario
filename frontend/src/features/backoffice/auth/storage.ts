// Back-office credential storage (#1785 Phase 6). The tenant auth plane and
// the back-office plane are intentionally segregated: the BE rejects a
// tenant token at /api/v1/backoffice/* and rejects a back-office token at
// /api/v1/admin/* (Phase 3 hardening). Storing the two access tokens under
// distinct localStorage keys keeps that split honest on the client too — a
// browser may be signed into both planes at once (operator who also has a
// tenant identity), and the http clients must read their own key.
//
// CSRF is handled differently here: the back-office refresh endpoint mints
// (and rotates) its own CSRF token via a response header read by the http
// client, just like the tenant flow. We keep CSRF in sessionStorage to
// match the tenant pattern and avoid bleeding the token across browser
// restarts.
const BACKOFFICE_ACCESS_TOKEN_KEY = "backoffice_access_token"
const BACKOFFICE_CSRF_KEY = "backoffice_csrf_token"

let csrfMemory: string | null = null

function safeLocalStorage(): Storage | null {
  if (typeof window === "undefined") return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

function safeSessionStorage(): Storage | null {
  if (typeof window === "undefined") return null
  try {
    return window.sessionStorage
  } catch {
    return null
  }
}

export function getBackofficeAccessToken(): string | null {
  return safeLocalStorage()?.getItem(BACKOFFICE_ACCESS_TOKEN_KEY) ?? null
}

export function setBackofficeAccessToken(token: string): void {
  safeLocalStorage()?.setItem(BACKOFFICE_ACCESS_TOKEN_KEY, token)
}

export function clearBackofficeAccessToken(): void {
  safeLocalStorage()?.removeItem(BACKOFFICE_ACCESS_TOKEN_KEY)
}

export function getBackofficeCsrfToken(): string | null {
  if (csrfMemory) return csrfMemory
  csrfMemory = safeSessionStorage()?.getItem(BACKOFFICE_CSRF_KEY) ?? null
  return csrfMemory
}

export function setBackofficeCsrfToken(token: string): void {
  csrfMemory = token
  safeSessionStorage()?.setItem(BACKOFFICE_CSRF_KEY, token)
}

export function clearBackofficeCsrfToken(): void {
  csrfMemory = null
  safeSessionStorage()?.removeItem(BACKOFFICE_CSRF_KEY)
}

// clearBackofficeAuth wipes both the access token and the CSRF token. The
// httpOnly refresh cookie at /api/v1/backoffice/auth is cleared server-side
// by POST /api/v1/backoffice/auth/logout; calling this without also POSTing
// to /logout leaves the cookie present, which is fine — the next refresh
// attempt will simply fail with the BE rejecting it.
export function clearBackofficeAuth(): void {
  clearBackofficeAccessToken()
  clearBackofficeCsrfToken()
}
