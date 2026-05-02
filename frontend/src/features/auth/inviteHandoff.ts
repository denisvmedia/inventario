// Shared sessionStorage bridge for the invite → register/login flow.
// When an unauthenticated user lands on /invite/<token>, the invite-accept
// view stashes the token here before navigating to /register or /login;
// the destination views read it back and auto-accept after auth.
//
// Scope is intentionally sessionStorage (not localStorage): the token must
// not outlive the browser session, and a closed tab clears it.
//
// Storage key matches frontend/src/services/inviteHandoff.ts so a flow
// started in the legacy bundle survives a switch to the new bundle and vice
// versa during the dual-bundle migration window (#1397).

const STORAGE_KEY = "inventario_pending_invite"

export interface PendingInvite {
  token: string
  groupName?: string
}

function safeSession(): Storage | null {
  if (typeof window === "undefined") return null
  try {
    return window.sessionStorage
  } catch {
    return null
  }
}

export function savePendingInvite(invite: PendingInvite): void {
  const s = safeSession()
  if (!s) return
  try {
    s.setItem(STORAGE_KEY, JSON.stringify(invite))
  } catch {
    // Quota / serialization errors must not break the auth flow.
  }
}

export function peekPendingInvite(): PendingInvite | null {
  const s = safeSession()
  if (!s) return null
  const raw = s.getItem(STORAGE_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as PendingInvite
    if (!parsed || typeof parsed.token !== "string" || !parsed.token) return null
    return parsed
  } catch {
    return null
  }
}

export function clearPendingInvite(): void {
  safeSession()?.removeItem(STORAGE_KEY)
}

export function consumePendingInvite(): PendingInvite | null {
  const peek = peekPendingInvite()
  clearPendingInvite()
  return peek
}
