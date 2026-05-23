import { createContext, useContext, useMemo, type ReactNode } from "react"

import { useOptionalAuth } from "@/features/auth/AuthContext"

import { useImpersonationState } from "../hooks"
import type { ImpersonationOperator, ImpersonationState, ImpersonationUser } from "../api"

// The shape every consumer of useImpersonation() reads. `active` is the
// single render gate for the banner; the rest of the quartet is populated
// only while a session is in progress.
interface ImpersonationContextValue {
  // True while the current browser is inside an impersonation session.
  active: boolean
  // The impersonated tenant user — populated only when `active` is true.
  targetUser: ImpersonationUser | null
  // The back-office operator who initiated the session — populated only
  // when active. Phase 5 (#1785) renamed `admin_user` to `operator` on
  // the wire and switched its shape from a tenant user to a back-office
  // operator (id, email, name, role).
  operator: ImpersonationOperator | null
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
  operator: null,
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
function toContextValue(
  state: ImpersonationState | undefined,
  isLoading: boolean
): ImpersonationContextValue {
  if (!state?.active) {
    return { ...INACTIVE, isLoading }
  }
  return {
    active: true,
    targetUser: state.target_user ?? null,
    operator: state.operator ?? null,
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
// hook. This provider ships the read-only `current` probe that gates the
// banner; #1750 shipped the BE `end` primitive, and #1757 wired the FE
// start / "End impersonation" / auto-expiry flows on top of it (see
// `useStartImpersonation` / `useEndImpersonation` in ../hooks.ts).
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
