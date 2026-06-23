// Pure functions that talk to the backend. Hooks (`./hooks.ts`) wrap these
// in TanStack Query — keeping fetch logic separate from React makes them
// trivial to test with MSW and reuse outside React (e.g. boot probes).
import { http } from "@/lib/http"
import { clearAuth, setAccessToken, setCsrfToken } from "@/lib/auth-storage"
import type { Schema } from "@/types"

export type CurrentUser = Schema<"models.User">

interface LoginResponse {
  access_token?: string
  csrf_token?: string
  user?: CurrentUser
}

interface MessageResponse {
  message?: string
}

export interface RegisterRequest {
  email: string
  password: string
  name: string
  // When set, registration succeeds even if open registration is closed and
  // the email-verification step is skipped. The token is NOT consumed here —
  // the caller must POST /invites/{token}/accept after logging in (#1285).
  invite_token?: string
}

export function getCurrentUser(signal?: AbortSignal): Promise<CurrentUser> {
  return http.get<CurrentUser>("/auth/me", { signal, authCheck: "user-initiated" })
}

// LoginOutcome surfaces the two-step shape POST /auth/login now produces.
//
//   - kind: "ok"           — credentials accepted, tokens already stored
//                            in localStorage. The page navigates onward.
//   - kind: "mfa_required" — credentials accepted, but the user has TOTP
//                            enabled. mfaToken is a short-lived challenge
//                            token that authorises POST /auth/login/mfa.
//
// The discriminated union keeps the page free of "did login return a user
// or did it return a challenge?" branching scattered through the JSX —
// callers `switch` on `kind` once.
export type LoginOutcome =
  | { kind: "ok"; user: CurrentUser | undefined }
  | {
      kind: "mfa_required"
      mfaToken: string
      expiresIn: number
      email: string
    }

interface LoginMFAChallengeBody {
  mfa_required: true
  mfa_token: string
  expires_in: number
  email: string
}

// login persists the access + CSRF tokens before resolving so the very next
// /auth/me probe sees them.
//
// Tenancy is HOST-BASED: the request body carries only { email, password } —
// no tenantSlug. The backend's HostTenantResolver derives the tenant from the
// request host (subdomain) before this handler runs, so the frontend never
// sends or asks for a tenant identifier. See devdocs/frontend/auth.md
// ("Login input model — host-based tenancy") and go/apiserver/tenant_context.go.
//
// When MFA is enabled the backend returns 200 with `mfa_required: true`
// instead of issuing tokens. We disambiguate via the field rather than
// HTTP status because credentials were correct — it's the *step* that
// is incomplete.
// finalizeLoginResponse maps the shared /auth/login-shaped body into a
// LoginOutcome. Both password login and magic-link verify (#magic-link)
// resolve to the exact same wire shape — either a LoginResponse or an
// `mfa_required` challenge — so they reuse this one helper. On the
// non-MFA path it persists the access + CSRF tokens before resolving so
// the very next /auth/me probe sees them.
function finalizeLoginResponse(body: LoginResponse & Partial<LoginMFAChallengeBody>): LoginOutcome {
  if (body.mfa_required && body.mfa_token) {
    return {
      kind: "mfa_required",
      mfaToken: body.mfa_token,
      expiresIn: body.expires_in ?? 0,
      email: body.email ?? "",
    }
  }
  if (body.access_token) setAccessToken(body.access_token)
  if (body.csrf_token) setCsrfToken(body.csrf_token)
  return { kind: "ok", user: body.user }
}

export async function login(email: string, password: string): Promise<LoginOutcome> {
  const body = await http.post<LoginResponse & Partial<LoginMFAChallengeBody>>("/auth/login", {
    email,
    password,
  })
  return finalizeLoginResponse(body)
}

// requestMagicLink asks the backend to email a one-time sign-in link. The
// response is always a neutral 200 message regardless of whether the email
// maps to an active account (anti-enumeration) — mirrors forgotPassword, so
// callers treat the resolution as success copy, never an "email exists" probe.
export async function requestMagicLink(email: string): Promise<string> {
  const body = await http.post<MessageResponse>("/auth/magic-link/request", { email })
  return body.message ?? ""
}

// verifyMagicLink exchanges the token from the emailed link for a session.
// The backend returns the EXACT same shape as POST /auth/login — either a
// LoginResponse (tokens issued) or an `mfa_required` challenge — so it reuses
// finalizeLoginResponse: a non-MFA success logs the user in (tokens stored),
// an `mfa_required` response is surfaced to the existing MFAChallenge UI. An
// invalid / expired / replayed token comes back as a non-2xx → HttpError,
// which MagicLinkPage maps to the "request a new link" state.
export async function verifyMagicLink(token: string): Promise<LoginOutcome> {
  const body = await http.post<LoginResponse & Partial<LoginMFAChallengeBody>>(
    "/auth/magic-link/verify",
    { token }
  )
  return finalizeLoginResponse(body)
}

// completeLoginMFA exchanges the mfa_token + a current TOTP code (or an
// unused backup code) for a session. Sets the access/CSRF tokens on success.
export interface CompleteLoginMFARequest {
  mfaToken: string
  totpCode?: string
  backupCode?: string
}

export async function completeLoginMFA(
  req: CompleteLoginMFARequest
): Promise<CurrentUser | undefined> {
  const body = await http.post<LoginResponse>("/auth/login/mfa", {
    mfa_token: req.mfaToken,
    totp_code: req.totpCode,
    backup_code: req.backupCode,
  })
  if (body.access_token) setAccessToken(body.access_token)
  if (body.csrf_token) setCsrfToken(body.csrf_token)
  return body.user
}

// --- MFA management ------------------------------------------------------
//
// These call the authenticated /auth/mfa/* endpoints. Each function is a
// thin wrapper around http so hooks.ts can compose them with TanStack
// Query without duplicating the URLs.

// MFAState mirrors the backend enum (apiserver.MFAState). Single
// discriminator instead of the original (enabled, enrollment_in_progress)
// pair — encodes "no row | row pending verify | row active" without
// the (true, true) impossible combination.
export type MFAState = "none" | "pending" | "active"

export interface MFAStatus {
  state: MFAState
  enabledAt?: string | null
  lastUsedAt?: string | null
  backupCodesRemaining: number
}

interface MFAStatusBody {
  state?: MFAState
  enabled_at?: string
  last_used_at?: string
  backup_codes_remaining?: number
}

function adaptStatus(body: MFAStatusBody): MFAStatus {
  return {
    state: body.state ?? "none",
    enabledAt: body.enabled_at ?? null,
    lastUsedAt: body.last_used_at ?? null,
    backupCodesRemaining: body.backup_codes_remaining ?? 0,
  }
}

export async function getMFAStatus(signal?: AbortSignal): Promise<MFAStatus> {
  const body = await http.get<MFAStatusBody>("/auth/mfa/status", {
    signal,
    authCheck: "user-initiated",
  })
  return adaptStatus(body)
}

export interface MFASetupBody {
  secret: string
  qrCodeURL: string
}

export async function startMFASetup(): Promise<MFASetupBody> {
  const body = await http.post<{ secret?: string; qr_code_url?: string }>("/auth/mfa/setup", {})
  return { secret: body.secret ?? "", qrCodeURL: body.qr_code_url ?? "" }
}

export async function verifyMFASetup(code: string): Promise<string[]> {
  const body = await http.post<{ backup_codes?: string[] }>("/auth/mfa/verify", { code })
  return body.backup_codes ?? []
}

export interface DisableMFARequest {
  password: string
  totpCode?: string
  backupCode?: string
}

export async function disableMFA(req: DisableMFARequest): Promise<void> {
  await http.post<MessageResponse>("/auth/mfa/disable", {
    password: req.password,
    totp_code: req.totpCode,
    backup_code: req.backupCode,
  })
}

export async function regenerateMFABackupCodes(code: string): Promise<string[]> {
  const body = await http.post<{ backup_codes?: string[] }>("/auth/mfa/regenerate-backup-codes", {
    code,
  })
  return body.backup_codes ?? []
}

export async function logout(): Promise<void> {
  try {
    await http.post<void>("/auth/logout")
  } finally {
    // Whether or not the server acknowledged, the user has clicked Logout —
    // wiping local credentials guarantees the UI can't keep using them.
    clearAuth()
  }
}

// register hits the unauthenticated /register endpoint. The server always
// returns 200 with a generic message regardless of whether the email is
// already taken (anti-enumeration), so callers should treat `message` as
// success copy and never as an "email exists" probe.
export async function register(req: RegisterRequest): Promise<string> {
  const body = await http.post<MessageResponse>("/register", req)
  return body.message ?? ""
}

// verifyEmail completes the sign-up flow with the token from the magic link.
// The server returns either 200 with a message or a non-2xx — surfaced as
// HttpError, which the page maps to "expired" / "invalid" copy.
export async function verifyEmail(token: string): Promise<string> {
  const body = await http.get<MessageResponse>(`/verify-email?token=${encodeURIComponent(token)}`)
  return body.message ?? ""
}

export async function resendVerification(email: string): Promise<string> {
  const body = await http.post<MessageResponse>("/resend-verification", { email })
  return body.message ?? ""
}

// forgotPassword always resolves with a generic success message — the
// backend returns the same body whether the email is known or not.
export async function forgotPassword(email: string): Promise<string> {
  const body = await http.post<MessageResponse>("/forgot-password", { email })
  return body.message ?? ""
}

export async function resetPassword(token: string, newPassword: string): Promise<string> {
  const body = await http.post<MessageResponse>("/reset-password", {
    token,
    new_password: newPassword,
  })
  return body.message ?? ""
}

// updateProfile patches the authenticated user's profile. The backend
// accepts only `name` and `default_group_id` — email is read-only here
// (changing it requires the verification flow which lives elsewhere).
//
// `default_group_id` semantics (#1263): undefined → leave unchanged,
// null → clear the preference, string → set to that group UUID. The
// backend validates the membership.
export interface UpdateProfileRequest {
  name: string
  default_group_id?: string | null
}

export async function updateProfile(req: UpdateProfileRequest): Promise<CurrentUser> {
  return http.put<CurrentUser>("/auth/me", req)
}

export interface ChangePasswordRequest {
  current_password: string
  new_password: string
}

// changePassword posts the credentials. On success the backend invalidates
// every session — the caller is responsible for following up with logout()
// + redirect to /login so the UI doesn't keep using a now-revoked token.
export async function changePassword(req: ChangePasswordRequest): Promise<string> {
  const body = await http.post<MessageResponse>("/auth/change-password", req)
  return body.message ?? ""
}

// deleteAccount permanently erases the authenticated user (#2147). The BE
// re-verifies `password` via user.CheckPassword (skipped for OAuth-only
// users with an empty hash) and replies 204 No Content on success, after
// which it has already torn down every active session (refresh tokens
// revoked, access tokens blacklisted). Failures arrive as a jsonapi.Errors
// envelope with a dotted code (auth.delete.invalid_password /
// auth.delete.last_owner / auth.delete.owns_content) the caller branches on.
//
// We use `http.request` rather than `http.del` because the BE re-auth check
// reads the password from the DELETE body, and `http.del` strips the body
// (mirrors features/group/api.ts deleteGroup). The CSRF header is added by
// the http layer for every mutating method, so the X-CSRF-Token the BE
// requires under requireAuth rides along automatically.
export async function deleteAccount(password: string): Promise<void> {
  await http.request<void>("/auth/me", { method: "DELETE", body: { password } })
}
