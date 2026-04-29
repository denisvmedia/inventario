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

// login persists the access + CSRF tokens before resolving so the very next
// /auth/me probe sees them.
export async function login(email: string, password: string): Promise<CurrentUser | undefined> {
  const body = await http.post<LoginResponse>("/auth/login", { email, password })
  if (body.access_token) setAccessToken(body.access_token)
  if (body.csrf_token) setCsrfToken(body.csrf_token)
  return body.user
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
