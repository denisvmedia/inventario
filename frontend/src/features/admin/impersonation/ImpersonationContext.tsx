import { createContext, useContext, useMemo, type ReactNode } from "react"

import { useOptionalAuth } from "@/features/auth/AuthContext"

import { useImpersonationState } from "../hooks"
import type { ImpersonationState, ImpersonationUser } from "../api"

// The shape every consumer of useImpersonation() reads. `active` is the
// single render gate for the banner; the rest of the quartet is populated
// only while a session is in progress.
interface ImpersonationContextValue {
  // True while the current browser is inside an impersonation session.
  active: boolean
  // The impersonated user — populated only when `active` is true.
  targetUser: ImpersonationUser | null
  // The operator who initiated the session — populated only when active.
  adminUser: ImpersonationUser | null
  // ISO timestamps bounding the session; null when inactive.
  startedAt: string | null
  expiresAt: string | null
  // True while the boot probe is still resolving the state for the first
  // time. The banner stays hidden during this window.
  isLoading: boolean
}

const INACTIVE: ImpersonationContextValue = {
  active: false,
  targetUser: null,
  adminUser: null,
  startedAt: null,
  expiresAt: null,
  isLoading: false,
}

const Context = createContext<ImpersonationContextValue | undefined>(undefined)

interface ImpersonationProviderProps {
  children: ReactNode
}

// Normalizes the raw BE payload into the context value. The BE returns
// `{ active: false }` with no other fields when no session is running, so
// every field below is defensively defaulted.
function toContextValue(state: ImpersonationState | undefined, isLoading: boolean): ImpersonationContextValue {
  if (!state?.active) {
    return { ...INACTIVE, isLoading }
  }
  return {
    active: true,
    targetUser: state.target_user ?? null,
    adminUser: state.admin_user ?? null,
    startedAt: state.started_at ?? null,
    expiresAt: state.expires_at ?? null,
    isLoading,
  }
}

// ImpersonationProvider tracks whether the current browser is inside an
// impersonation session via GET /admin/impersonation/current and feeds the
// ImpersonationBanner. Mounted inside the authenticated Shell tree, below
// AuthProvider — it only probes once the user is signed in.
//
// Mirrors the AuthProvider / GroupProvider pattern: a query hook drives a
// memoized context value, consumers read it through a `useImpersonation()`
// hook. The "End impersonation" action is wired in a later sub-issue
// (#1750 ships the BE primitive); this issue ships the read-only state.
export function ImpersonationProvider({ children }: ImpersonationProviderProps) {
  const auth = useOptionalAuth()
  const isAuthenticated = auth?.isAuthenticated ?? false
  // Only probe once authenticated — the endpoint 401s otherwise, and an
  // unauthenticated browser is never inside an impersonation session.
  const { data, isLoading } = useImpersonationState({ enabled: isAuthenticated })

  const value = useMemo<ImpersonationContextValue>(
    () => (isAuthenticated ? toContextValue(data, isLoading) : INACTIVE),
    [isAuthenticated, data, isLoading]
  )

  return <Context.Provider value={value}>{children}</Context.Provider>
}

// Reads the impersonation context. Throws outside the provider — that's a
// programming error, not a user-facing one.
export function useImpersonation(): ImpersonationContextValue {
  const ctx = useContext(Context)
  if (!ctx) throw new Error("useImpersonation must be used inside <ImpersonationProvider>")
  return ctx
}

// Optional variant for chrome components that may render outside the
// provider (e.g. on bare test surfaces). Same motivation as
// useOptionalAuth / useOptionalCurrentGroup — degrade silently.
export function useOptionalImpersonation(): ImpersonationContextValue | undefined {
  return useContext(Context)
}
