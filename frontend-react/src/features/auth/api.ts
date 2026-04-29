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

export function getCurrentUser(signal?: AbortSignal): Promise<CurrentUser> {
  return http.get<CurrentUser>("/auth/me", { signal, authCheck: "user-initiated" })
}

// login persists the access + CSRF tokens before resolving so the very next
// /auth/me probe sees them. The Auth pages issue (#1407) is what actually
// renders a form on top of this; here it's exposed for AuthContext + tests.
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
