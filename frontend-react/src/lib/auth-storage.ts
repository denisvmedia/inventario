// Persistent auth-credential storage shared with the legacy Vue frontend
// during the dual-bundle migration window (#1397). Storage keys MUST match
// frontend/src/services/api.ts so a session started in one bundle survives
// when the user switches via INVENTARIO_FRONTEND.
const ACCESS_TOKEN_KEY = "inventario_token"
const USER_KEY = "inventario_user"
const CSRF_KEY = "inventario_csrf_token"

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

export function clearAuth(): void {
  clearAccessToken()
  clearCsrfToken()
  safeLocalStorage()?.removeItem(USER_KEY)
}
