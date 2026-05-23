import { http, HttpResponse } from "msw"

import type { Schema } from "@/types"

import { apiUrl } from "."

type BackofficeUser = Schema<"apiserver.BackofficeProfile">

export interface BackofficeSignedInOptions {
  // Override the operator profile returned by /backoffice/auth/me. Defaults
  // to a stable fixture used across admin tests.
  user?: Partial<BackofficeUser>
}

export const fixtureOperator: BackofficeUser = {
  id: "op-1",
  email: "operator@example.com",
  name: "Operator",
  role: "platform_admin",
  mfa_enforced: true,
} as BackofficeUser

// signedIn returns the canonical "back-office operator is logged in"
// handler set: /backoffice/auth/me resolves with the fixture (or override),
// /backoffice/auth/refresh quietly issues a refreshed token, and logout
// is a 204. Pair with `setBackofficeAccessToken("…")` in beforeEach so the
// boot probe fires.
export function signedIn(opts: BackofficeSignedInOptions = {}) {
  const user = { ...fixtureOperator, ...opts.user }
  return [
    http.get(apiUrl("/backoffice/auth/me"), () => HttpResponse.json(user)),
    http.post(apiUrl("/backoffice/auth/refresh"), () =>
      HttpResponse.json({
        access_token: "refreshed-bo-token",
        token_type: "Bearer",
        expires_in: 600,
      })
    ),
    http.post(apiUrl("/backoffice/auth/logout"), () => new HttpResponse(null, { status: 204 })),
  ]
}
