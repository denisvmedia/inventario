// Persistent auth-credential storage. Keys are stable across releases so
// sessions survive client upgrades.
const ACCESS_TOKEN_KEY = "inventario_token"
const USER_KEY = "inventario_user"
const CSRF_KEY = "inventario_csrf_token"
// Holds the impersonated user's id while an admin impersonation session is
// active (#1757). The backend keeps the admin's "return slot" server-side
// and POST /admin/impersonation/end hands the admin's fresh tokens back in
// the response body — so the FE never persists the admin's token. All it
// needs is the impersonated user's id, so ending the session (manually or
// on auto-expiry) can navigate back to /admin/users/{thatId}.
const IMPERSONATION_KEY = "inventario_impersonation"

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

export function getAccessToken(): string | null {
  return safeLocalStorage()?.getItem(ACCESS_TOKEN_KEY) ?? null
}

export function setAccessToken(token: string): void {
  safeLocalStorage()?.setItem(ACCESS_TOKEN_KEY, token)
}

export function clearAccessToken(): void {
  safeLocalStorage()?.removeItem(ACCESS_TOKEN_KEY)
}

export function getCsrfToken(): string | null {
  if (csrfMemory) return csrfMemory
  csrfMemory = safeSessionStorage()?.getItem(CSRF_KEY) ?? null
  return csrfMemory
}

export function setCsrfToken(token: string): void {
  csrfMemory = token
  safeSessionStorage()?.setItem(CSRF_KEY, token)
}

export function clearCsrfToken(): void {
  csrfMemory = null
  safeSessionStorage()?.removeItem(CSRF_KEY)
}

// The impersonated user's id, persisted so the End / auto-expiry flows can
// route back to that user's admin detail page. Returns null when no session
// is recorded or the stored value is malformed.
export interface ImpersonationReturn {
  targetUserId: string
}

export function getImpersonationReturn(): ImpersonationReturn | null {
  const raw = safeLocalStorage()?.getItem(IMPERSONATION_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Partial<ImpersonationReturn>
    if (parsed && typeof parsed.targetUserId === "string" && parsed.targetUserId) {
      return { targetUserId: parsed.targetUserId }
    }
    return null
  } catch {
    return null
  }
}

export function setImpersonationReturn(value: ImpersonationReturn): void {
  safeLocalStorage()?.setItem(IMPERSONATION_KEY, JSON.stringify(value))
}

export function clearImpersonationReturn(): void {
  safeLocalStorage()?.removeItem(IMPERSONATION_KEY)
}

export function clearAuth(): void {
  clearAccessToken()
  clearCsrfToken()
  clearImpersonationReturn()
  safeLocalStorage()?.removeItem(USER_KEY)
}
