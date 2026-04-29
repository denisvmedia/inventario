import { createContext, useContext, useEffect, useMemo, type ReactNode } from "react"
import { useNavigate } from "react-router-dom"

import { getAccessToken } from "@/lib/auth-storage"
import {
  __resetNavigationForTests,
  setNavigateToLogin as setHttpNavigateToLogin,
} from "@/lib/navigation"

import { useCurrentUser, useLogout } from "./hooks"
import type { CurrentUser } from "./api"

interface AuthContextValue {
  // The currently signed-in user. `undefined` while the initial probe is in
  // flight; `null` once the probe resolves with a 401 (no session).
  user: CurrentUser | undefined | null
  // True after the initial /auth/me probe has settled (success OR error) so
  // route guards can stop showing the boot spinner without flipping a tab to
  // /login on a transient blip.
  isInitialized: boolean
  // Convenience flag — the most common consumer just wants a yes/no.
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
  const { data: user, isFetched, isError } = useCurrentUser()
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
    // Without an access token there's nothing to probe — the query is
    // disabled and we can flip `isInitialized` immediately so the guard
    // layer doesn't sit on the boot fallback. With a token, `isFetched`
    // is the canonical "the query has resolved at least once" flag,
    // flipping true on both success and error so a transient /auth/me
    // failure doesn't pin the spinner.
    const hasToken = !!getAccessToken()
    const isInitialized = !hasToken || isFetched
    const resolvedUser: CurrentUser | undefined | null = isError
      ? null
      : (user ?? (isInitialized ? null : undefined))
    return {
      user: resolvedUser,
      isInitialized,
      isAuthenticated: !!resolvedUser,
      logout: () => logoutMutation.mutateAsync(),
    }
  }, [user, isFetched, isError, logoutMutation])

  return <Context.Provider value={value}>{children}</Context.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(Context)
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>")
  return ctx
}
