// Persistent auth-credential storage. Keys are stable across releases so
// sessions survive client upgrades.
//
// The access token lives in localStorage rather than in a module variable, and
// that is deliberate (#2479, #967 H7). An in-memory token dies with the tab,
// so every reload and every new tab would have to spend a refresh round trip
// before it could render anything.
//
// What makes the trade-off acceptable is where the durable credential is: the
// refresh token is an httpOnly cookie that script cannot read. So script with
// access to localStorage gets an access token whose exposure ends with its
// TTL, not a session it can keep renewing. Moving the access token in-memory
// would not change that — it would only shrink an already-bounded window, at
// the cost of a refresh on every page load.
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

// The identity a stored access token belongs to: the `user_id` claim of the
// JWT, or null when no token is stored. A token whose payload cannot be read
// maps to one shared value, so replacing one such token with another counts
// as the same user. The claim is read without verifying the signature; it
// only tells this tab that the signed-in user changed, and the server checks
// every request on its own.
const UNREADABLE_TOKEN_IDENTITY = "unreadable-token"

function tokenIdentity(token: string | null): string | null {
  if (!token) return null
  const payload = token.split(".")[1]
  if (!payload) return UNREADABLE_TOKEN_IDENTITY
  try {
    const claims: unknown = JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/")))
    if (claims && typeof claims === "object" && "user_id" in claims) {
      const userId = (claims as { user_id: unknown }).user_id
      if (typeof userId === "string" && userId) return userId
    }
  } catch {
    // Not base64 or not JSON: fall through to the shared value.
  }
  return UNREADABLE_TOKEN_IDENTITY
}

type IdentityListener = () => void

const identityListeners = new Set<IdentityListener>()

// Counts the changes of the signed-in user this tab has seen. Work that
// started under one value and finishes under another belongs to a user who is
// no longer signed in (see the mutation cache in lib/query-client.ts).
let identityGeneration = 0

export function authIdentityGeneration(): number {
  return identityGeneration
}

function notifyIdentityChange(): void {
  identityGeneration += 1
  for (const listener of [...identityListeners]) listener()
}

// Another tab wrote the access token. A refresh there keeps the same user and
// is ignored; a sign-in, a sign-out or a switch to another user is reported
// here too. A null key means that tab cleared the whole storage area.
function onStorage(event: StorageEvent): void {
  if (event.storageArea && event.storageArea !== safeLocalStorage()) return
  if (event.key === null) {
    notifyIdentityChange()
    return
  }
  if (event.key !== ACCESS_TOKEN_KEY) return
  if (tokenIdentity(event.oldValue) !== tokenIdentity(event.newValue)) notifyIdentityChange()
}

// Calls `listener` after the signed-in user changes, in this tab or in another
// one: sign-in, sign-out, and a switch from one user to another. Refreshing the
// token of the same user does not call it. Returns the unsubscribe function.
export function onAuthIdentityChange(listener: IdentityListener): () => void {
  if (identityListeners.size === 0 && typeof window !== "undefined") {
    window.addEventListener("storage", onStorage)
  }
  identityListeners.add(listener)
  return () => {
    identityListeners.delete(listener)
    if (identityListeners.size === 0 && typeof window !== "undefined") {
      window.removeEventListener("storage", onStorage)
    }
  }
}

export function getAccessToken(): string | null {
  return safeLocalStorage()?.getItem(ACCESS_TOKEN_KEY) ?? null
}

export function setAccessToken(token: string): void {
  const before = tokenIdentity(getAccessToken())
  safeLocalStorage()?.setItem(ACCESS_TOKEN_KEY, token)
  if (tokenIdentity(token) !== before) notifyIdentityChange()
}

export function clearAccessToken(): void {
  const hadToken = getAccessToken() !== null
  safeLocalStorage()?.removeItem(ACCESS_TOKEN_KEY)
  if (hadToken) notifyIdentityChange()
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
