import { createContext, useContext, useEffect, useMemo, type ReactNode } from "react"
import { useNavigate } from "react-router-dom"

import { getAccessToken } from "@/lib/auth-storage"
import { HttpError } from "@/lib/http"
import {
  __resetNavigationForTests,
  setNavigateToLogin as setHttpNavigateToLogin,
} from "@/lib/navigation"

import { useCurrentUser, useLogout } from "./hooks"
import type { CurrentUser } from "./api"

interface AuthContextValue {
  // The currently signed-in user. Tri-state:
  //   - object  → signed in (the only "isAuthenticated=true" branch).
  //   - null    → definitely not signed in (no token, or /auth/me returned 401).
  //   - undefined → unknown — initial probe hasn't settled OR the probe errored
  //     with a non-401 status (e.g. transient 5xx). Guards should keep showing
  //     the boot fallback rather than bouncing the user to /login on a blip.
  user: CurrentUser | undefined | null
  // True once we have a definitive answer for `user` (either the probe settled
  // OR there is no token to probe with). Stays false on transient backend
  // errors so the boot fallback renders rather than the login page.
  isInitialized: boolean
  // Convenience flag — only true when `user` is an actual user object.
  isAuthenticated: boolean
  // Imperative handles for logout; login lands in #1407 (Auth pages).
  logout: () => Promise<void>
}

const Context = createContext<AuthContextValue | undefined>(undefined)

interface AuthProviderProps {
  children: ReactNode
}

// AuthProvider runs the boot-time /auth/me probe and feeds the rest of the
// app a single source of truth for who is signed in. It also installs a
// router-aware redirect into lib/navigation so the http client's 401-handler
// (#1403) stays an SPA navigation rather than a full-page reload.
export function AuthProvider({ children }: AuthProviderProps) {
  const navigate = useNavigate()
  const { data: user, isFetched, error } = useCurrentUser()
  const logoutMutation = useLogout()

  // Replace the default window.location-based navigateToLogin with one that
  // uses react-router's navigate(). The default stays in place outside the
  // provider tree (e.g. during boot before <AuthProvider> mounts), and the
  // effect's cleanup restores it when the provider unmounts so tests don't
  // leak navigators between cases.
  useEffect(() => {
    setHttpNavigateToLogin((currentPath, reason) => {
      const params = new URLSearchParams({ redirect: currentPath })
      if (reason) params.set("reason", reason)
      navigate(`/login?${params.toString()}`)
    })
    return () => __resetNavigationForTests()
  }, [navigate])

  const value = useMemo<AuthContextValue>(() => {
    const hasToken = !!getAccessToken()
    // Classify the probe error. A 401 is a definitive "not signed in"; the
    // http client (#1403) has already tried to refresh and failed by the time
    // this surfaces. Anything else (network error, 5xx) is transient — we
    // hold the boot fallback rather than claim the user is logged out.
    const authError = error instanceof HttpError && error.status === 401
    const transientError = !!error && !authError

    // Without a token there is nothing to probe — settle synchronously.
    // With a token, the probe is settled on success or on a 401; transient
    // errors keep us in the "still trying" state.
    const isInitialized = !hasToken || (isFetched && !transientError)

    let resolvedUser: CurrentUser | undefined | null
    if (!hasToken) {
      resolvedUser = null
    } else if (authError) {
      resolvedUser = null
    } else if (transientError) {
      resolvedUser = undefined
    } else if (user) {
      resolvedUser = user
    } else if (isFetched) {
      // Probe succeeded but returned no body — backend bug. Treat as logged out
      // so the user lands on /login rather than spinning forever.
      resolvedUser = null
    } else {
      resolvedUser = undefined
    }

    return {
      user: resolvedUser,
      isInitialized,
      isAuthenticated: resolvedUser !== null && resolvedUser !== undefined,
      logout: () => logoutMutation.mutateAsync(),
    }
  }, [user, isFetched, error, logoutMutation])

  return <Context.Provider value={value}>{children}</Context.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(Context)
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>")
  return ctx
}

// Same intent as useOptionalCurrentGroup in features/group/GroupContext: chrome
// components mounted in both authenticated shells and bare boot states need to
// degrade silently when no provider is wrapped.
export function useOptionalAuth(): AuthContextValue | undefined {
  return useContext(Context)
}
