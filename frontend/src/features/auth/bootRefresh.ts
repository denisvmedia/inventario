// Boot-time speculative refresh — closes the gap between the OAuth callback
// landing the browser back in the app (no access token in localStorage, only
// the httpOnly refresh cookie) and the route guard's "no token → /login"
// reflex.
//
// Flow:
//   1. App boots; AuthProvider mounts.
//   2. If there is no access token in localStorage, we speculatively POST
//      /auth/refresh. The browser attaches the refresh cookie if it exists.
//   3. Success (200 + access_token) — write the token + CSRF and let the
//      normal /auth/me probe proceed. The user stays on the OAuth-landed
//      route instead of bouncing to /login.
//   4. Failure (401, network error, no cookie) — leave storage empty; the
//      normal guard bounces to /login as today.
//
// One-shot guard: AuthProvider re-mounts (e.g. StrictMode dev double-render)
// must not refire the request. The module-level `attempted` flag is the
// canonical guard; the in-flight promise is reused if a second caller hits
// the helper while the first request is still pending. `__resetBootRefreshForTests`
// resets the guard for unit tests.
//
// Why not call lib/http's `refreshAccessToken` directly: that helper lives
// behind a private symbol in http.ts and shares a single-flight slot used by
// the 401 → retry dance. Reusing it would couple the boot path to the
// retry-side state machine; a small standalone fetch keeps the contract
// crystal clear (one POST, no retries, no navigation side effects).
import { setAccessToken, setCsrfToken } from "@/lib/auth-storage"

const REFRESH_PATH = "/api/v1/auth/refresh"

let attempted = false
let inFlight: Promise<string | null> | null = null

interface RefreshBody {
  access_token?: string
  csrf_token?: string
}

// tryBootRefresh attempts a single /auth/refresh and resolves with the
// access token on success, `null` on any failure (HTTP non-2xx, network
// error, empty body). It NEVER throws — the boot path treats refresh
// failures as "no session yet", which is the same state as a fresh tab.
//
// Subsequent calls within the same module lifetime return the same
// memoised promise so AuthProvider re-renders / StrictMode double-mounts
// don't multiply the request.
export function tryBootRefresh(): Promise<string | null> {
  if (inFlight) return inFlight
  if (attempted) return Promise.resolve(null)
  attempted = true
  inFlight = (async () => {
    try {
      const response = await fetch(REFRESH_PATH, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: "{}",
        cache: "no-store",
      })
      if (!response.ok) return null
      const body = (await response.json().catch(() => null)) as RefreshBody | null
      if (!body || !body.access_token) return null
      setAccessToken(body.access_token)
      if (body.csrf_token) setCsrfToken(body.csrf_token)
      return body.access_token
    } catch {
      // Network error / aborted / parse failure — treat as "no session".
      return null
    } finally {
      inFlight = null
    }
  })()
  return inFlight
}

// hasAttemptedBootRefresh reports whether the one-shot guard has already
// fired. AuthContext reads this to decide whether to render the boot
// fallback or proceed to the no-token branch.
export function hasAttemptedBootRefresh(): boolean {
  return attempted
}

// Test-only reset — vitest's beforeEach clears the guard so each case
// starts from a clean slate.
export function __resetBootRefreshForTests(): void {
  attempted = false
  inFlight = null
}

// Test-only: pre-stamp the one-shot guard as "already attempted" so a test
// that mounts <AuthProvider> without an access token does not fire the
// speculative /auth/refresh request and trip MSW's `onUnhandledRequest:
// "error"`. Tests that DO exercise the boot-refresh path call
// `__resetBootRefreshForTests` to flip this back to "fresh".
export function markBootRefreshAttemptedForTests(): void {
  attempted = true
}
