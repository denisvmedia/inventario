// Pure functions that talk to the back-office auth plane (#1785). Mirrors
// `features/auth/api.ts` but routes to /backoffice/auth/* and persists
// credentials through the back-office storage. Hooks (`./context.tsx`)
// wrap these in TanStack Query.
//
// The two planes are intentionally separate: a tenant token is rejected at
// /api/v1/admin/* and a back-office token is rejected at the tenant
// surfaces (Phase 3 hardening). The plane-aware http client (lib/http.ts)
// reads the right token / CSRF / refresh endpoint based on the path
// prefix, so this module never has to thread plane metadata explicitly.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

import { clearBackofficeAuth, setBackofficeAccessToken, setBackofficeCsrfToken } from "./storage"

// The profile shape returned by /backoffice/auth/login and /backoffice/auth/me.
// Generated from apiserver.BackofficeProfile — id/email/name/role + the
// `mfa_enforced` flag the back-office UI surfaces in operator chrome.
export type BackofficeUser = Schema<"apiserver.BackofficeProfile">

// The success envelope from /backoffice/auth/login(/mfa) and
// /backoffice/auth/refresh.
type BackofficeLoginResponseBody = Schema<"apiserver.BackofficeLoginResponse">

// The 200 MFA-challenge / 501 enrollment-missing envelope from
// /backoffice/auth/login.
type BackofficeMFARequiredBody = Schema<"apiserver.BackofficeMFARequiredResponse">

// Stable code returned by the BE in the 501 enrollment-missing body. The
// FE branches on this to surface the "run inventario backoffice mfa setup"
// copy instead of a generic error. Kept as a constant so call sites read
// `BACKOFFICE_MFA_NOT_IMPLEMENTED` rather than the magic string.
export const BACKOFFICE_MFA_NOT_IMPLEMENTED = "backoffice.mfa_not_implemented"

// LoginOutcome discriminates the four post-login states the FE has to
// handle. `mfaRequired` and `mfaNotEnrolled` are both 200/501 successes
// from the BE's perspective (credentials were correct); they just gate the
// next FE step. `ok` is the terminal "tokens stored, navigate onward".
export type BackofficeLoginOutcome =
  | { kind: "ok"; user: BackofficeUser | undefined }
  | {
      kind: "mfaRequired"
      mfaToken: string
      expiresIn: number
      email: string
    }
  | { kind: "mfaNotEnrolled"; email: string }

// login submits credentials. The BE returns one of three success shapes:
//   - 200 { access_token, ... }                  → kind: "ok"
//   - 200 { mfa_required, mfa_token, ... }       → kind: "mfaRequired"
//   - 501 { mfa_required, code: not_implemented} → kind: "mfaNotEnrolled"
// A 401/422/etc surfaces as a thrown HttpError the caller maps to a
// banner. The MFA-required envelope is returned with HTTP 200 by the BE,
// so we have to inspect the body to distinguish — the http client throws
// only on non-2xx, so this happens BEFORE the catch.
//
// The 501 branch is special: the http client throws an HttpError(501)
// whose body matches BackofficeMFARequiredBody with `code` set to
// BACKOFFICE_MFA_NOT_IMPLEMENTED. We catch + translate it here so the
// caller's discriminated union covers every outcome without leaking the
// status-code special case into the page.
export async function backofficeLogin(
  email: string,
  password: string
): Promise<BackofficeLoginOutcome> {
  try {
    const body = await http.post<BackofficeLoginResponseBody & Partial<BackofficeMFARequiredBody>>(
      "/backoffice/auth/login",
      { email, password }
    )
    if (body.mfa_required && body.mfa_token) {
      return {
        kind: "mfaRequired",
        mfaToken: body.mfa_token,
        expiresIn: body.expires_in ?? 0,
        email: body.email ?? email,
      }
    }
    if (body.access_token) setBackofficeAccessToken(body.access_token)
    return { kind: "ok", user: body.user }
  } catch (err) {
    // Surface the 501 enrollment-missing branch as a discriminated outcome
    // rather than a thrown error — the caller wants a typed nudge to the
    // CLI, not a generic error toast.
    if (isMFANotEnrolled(err)) {
      const data = (err as { data: BackofficeMFARequiredBody }).data
      return { kind: "mfaNotEnrolled", email: data.email ?? email }
    }
    throw err
  }
}

interface HttpErrorLike {
  status: number
  data: unknown
}

function isMFANotEnrolled(err: unknown): boolean {
  if (!err || typeof err !== "object") return false
  const candidate = err as HttpErrorLike
  if (candidate.status !== 501) return false
  if (!candidate.data || typeof candidate.data !== "object") return false
  const body = candidate.data as BackofficeMFARequiredBody
  return body.code === BACKOFFICE_MFA_NOT_IMPLEMENTED
}

export interface BackofficeCompleteLoginMFARequest {
  mfaToken: string
  totpCode?: string
  backupCode?: string
}

// completeMFA exchanges the mfa_token from step-1 + a current TOTP code
// (or an unused backup code) for a session. Sets the back-office access
// token on success and returns the operator profile.
export async function backofficeCompleteMFA(
  req: BackofficeCompleteLoginMFARequest
): Promise<BackofficeUser | undefined> {
  const body = await http.post<BackofficeLoginResponseBody>("/backoffice/auth/login/mfa", {
    mfa_token: req.mfaToken,
    totp_code: req.totpCode,
    backup_code: req.backupCode,
  })
  if (body.access_token) setBackofficeAccessToken(body.access_token)
  return body.user
}

export function getBackofficeMe(signal?: AbortSignal): Promise<BackofficeUser> {
  return http.get<BackofficeUser>("/backoffice/auth/me", {
    signal,
    authCheck: "user-initiated",
  })
}

export async function backofficeLogout(): Promise<void> {
  try {
    await http.post<void>("/backoffice/auth/logout")
  } finally {
    // Whether or not the server acknowledged, the operator has clicked
    // Sign out — wipe local credentials so the UI can't keep using
    // them. Mirrors the tenant `clearAuth()` pattern.
    clearBackofficeAuth()
  }
}

// backofficeRefresh is exported for tests and explicit boot probes;
// production callers normally rely on the http client's 401 dispatcher.
// On success the new tokens are written through storage (same side
// effects as the auto-refresh path inside lib/http.ts).
interface BackofficeRefreshBody {
  access_token?: string
  csrf_token?: string
}

export async function backofficeRefresh(): Promise<string> {
  const body = await http.post<BackofficeRefreshBody>(
    "/backoffice/auth/refresh",
    {},
    { skipAuthRefresh: true }
  )
  if (!body.access_token) {
    throw new Error("Back-office refresh returned no access_token")
  }
  setBackofficeAccessToken(body.access_token)
  if (body.csrf_token) setBackofficeCsrfToken(body.csrf_token)
  return body.access_token
}
