import { http, HttpResponse } from "msw"

import type { Schema } from "@/types"

import { apiUrl } from "."

type User = Schema<"models.User">

export interface SignedInOptions {
  // Override the user object returned by /auth/me. Defaults to a stable
  // fixture used across most tests.
  user?: Partial<User>
}

export const fixtureUser: User = {
  id: "u1",
  email: "denis@example.com",
  name: "Denis",
} as User

// signedIn returns the canonical "user is logged in" handler set: /auth/me
// resolves with a real user, refresh and logout are both quiet 204s. Tests
// that need to flip the user (different default_group_id, different role)
// pass `user`; tests that need a different shape build their own handlers
// with `apiUrl()` directly.
export function signedIn(opts: SignedInOptions = {}) {
  const user = { ...fixtureUser, ...opts.user }
  return [
    http.get(apiUrl("/auth/me"), () => HttpResponse.json(user)),
    http.post(apiUrl("/auth/refresh"), () =>
      HttpResponse.json({ access_token: "refreshed-token", csrf_token: "refreshed-csrf" })
    ),
    http.post(apiUrl("/auth/logout"), () => new HttpResponse(null, { status: 204 })),
  ]
}

// signedOut models a brand-new tab: /auth/me 401s, refresh also 401s. The
// http client's 401-handler should redirect the user to /login, which the
// router's tests assert.
export function signedOut() {
  return [
    http.get(apiUrl("/auth/me"), () => HttpResponse.json(null, { status: 401 })),
    http.post(apiUrl("/auth/refresh"), () => HttpResponse.json(null, { status: 401 })),
  ]
}

// transientServerError is what we use to assert "we don't bounce the user
// to /login on a 5xx blip" — see ProtectedRoute and AuthContext tests.
export function transientServerError() {
  return [
    http.get(apiUrl("/auth/me"), () =>
      HttpResponse.json({ error: "service unavailable" }, { status: 503 })
    ),
  ]
}
