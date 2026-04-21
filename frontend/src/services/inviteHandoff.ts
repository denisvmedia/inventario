// Shared sessionStorage bridge for the invite → register/login flow.
// When an unauthenticated user lands on /invite/<token>, the invite-accept
// view stashes the token here before sending them to /register or /login;
// the destination views read the token back and auto-accept after auth.
// Scope is intentionally sessionStorage (not localStorage): the token should
// not outlive a browser session, and private-tab / closed-window clears it.

const STORAGE_KEY = 'inventario_pending_invite'

export interface PendingInvite {
  token: string
  groupName?: string
}

function canAccessStorage(): boolean {
  try {
    return typeof window !== 'undefined' && !!window.sessionStorage
  } catch {
    return false
  }
}

export function savePendingInvite(invite: PendingInvite): void {
  if (!canAccessStorage()) return
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(invite))
  } catch (err) {
    // Quota errors shouldn't break the flow — fall through silently.
    console.warn('[inviteHandoff] failed to persist invite:', err)
  }
}

export function peekPendingInvite(): PendingInvite | null {
  if (!canAccessStorage()) return null
  const raw = sessionStorage.getItem(STORAGE_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as PendingInvite
    if (!parsed || typeof parsed.token !== 'string' || !parsed.token) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

export function consumePendingInvite(): PendingInvite | null {
  const peek = peekPendingInvite()
  clearPendingInvite()
  return peek
}

export function clearPendingInvite(): void {
  if (!canAccessStorage()) return
  sessionStorage.removeItem(STORAGE_KEY)
}
