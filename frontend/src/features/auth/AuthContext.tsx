import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react"
import { useNavigate } from "react-router-dom"
import { useQueryClient } from "@tanstack/react-query"

import { getAccessToken } from "@/lib/auth-storage"
import { HttpError } from "@/lib/http"
import {
  __resetNavigationForTests,
  setNavigateToLogin as setHttpNavigateToLogin,
  setNavigateToMaintenance as setHttpNavigateToMaintenance,
} from "@/lib/navigation"

import { tryBootRefresh } from "./bootRefresh"
import { useCurrentUser, useLogout } from "./hooks"
import { authKeys } from "./keys"
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
//
// Boot-time refresh (#1394): the OAuth callback 302s the browser back to the
// app with NO access token in localStorage — only the httpOnly refresh cookie
// the BE just minted. Without this hook, the route guard would see "no token"
// and bounce to /login, undoing the sign-in. So before reporting
// `isInitialized=true` for the no-token case, AuthProvider speculatively
// POSTs /auth/refresh once. On success, the token lands in storage and the
// normal /auth/me probe takes over; on failure, the user falls through to
// /login as today. The one-shot guard lives in features/auth/bootRefresh.ts.
export function AuthProvider({ children }: AuthProviderProps) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data: user, isFetched, error } = useCurrentUser()
  const logoutMutation = useLogout()

  // Boot-refresh state machine. Three values:
  //   - "pending"  — request in flight (or about to be).
  //   - "settled"  — request returned (success OR failure); normal logic
  //                  takes over.
  //   - "skipped"  — there was already a token at mount time, no refresh
  //                  needed; we never block initialization on the helper.
  // Initialised lazily so the very first render already reflects the
  // correct branch (skipping the refresh when a token is present).
  const [bootRefreshState, setBootRefreshState] = useState<"pending" | "settled" | "skipped">(() =>
    getAccessToken() ? "skipped" : "pending"
  )

  useEffect(() => {
    if (bootRefreshState !== "pending") return
    let cancelled = false
    void tryBootRefresh().then((token) => {
      if (cancelled) return
      if (token) {
        // The /auth/me query was created with `enabled: !!getAccessToken()`,
        // which evaluated to `false` at mount time (no token yet). Invalidate
        // the key now so TanStack Query re-evaluates `enabled` and fires the
        // probe with the freshly stored Bearer token.
        queryClient.invalidateQueries({ queryKey: authKeys.currentUser() })
      }
      setBootRefreshState("settled")
    })
    return () => {
      cancelled = true
    }
  }, [bootRefreshState, queryClient])

  // Replace the default window.location-based navigateToLogin with one that
  // uses react-router's navigate(). The default stays in place outside the
  // provider tree (e.g. during boot before <AuthProvider> mounts), and the
  // effect's cleanup restores it when the provider unmounts so tests don't
  // leak navigators between cases.
  useEffect(() => {
    setHttpNavigateToLogin((currentPath, reason, plane) => {
      const params = new URLSearchParams({ redirect: currentPath })
      if (reason) params.set("reason", reason)
      // `plane` (#1785 Phase 6) selects which login surface to bounce to:
      // a back-office 401 (a refresh on /admin/* or /backoffice/* that
      // failed) routes the operator to /backoffice/login; everything
      // else stays on the tenant /login. Defaulting to "tenant" keeps
      // the legacy single-plane callers unchanged.
      const target = plane === "backoffice" ? "/backoffice/login" : "/login"
      navigate(`${target}?${params.toString()}`)
    })
    // The http client (#1403) bounces here on a 503 from the API; the
    // Retry-After + X-Maintenance-Status headers carry the operator's
    // ETA + per-component status (api/database/storage) so the page can
    // render the mock-faithful status card. URL params keep the page
    // refresh-safe (#1542).
    setHttpNavigateToMaintenance(({ retryAfter, componentStatus }) => {
      const params = new URLSearchParams()
      if (retryAfter) params.set("retry_after", retryAfter)
      if (componentStatus) params.set("status", componentStatus)
      const qs = params.toString()
      navigate(qs ? `/maintenance?${qs}` : "/maintenance")
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

    // While the boot-refresh request is in flight we have NO definitive
    // answer yet — the refresh may yet hand us a token that turns this
    // session into an authenticated one. Hold the boot fallback rather
    // than briefly classifying the user as logged out (which would race
    // ProtectedRoute to /login).
    const bootRefreshPending = bootRefreshState === "pending"

    // Without a token there is nothing to probe — settle synchronously.
    // With a token, the probe is settled on success or on a 401; transient
    // errors keep us in the "still trying" state.
    const isInitialized = !bootRefreshPending && (!hasToken || (isFetched && !transientError))

    let resolvedUser: CurrentUser | undefined | null
    if (bootRefreshPending) {
      resolvedUser = undefined
    } else if (!hasToken) {
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
  }, [user, isFetched, error, logoutMutation, bootRefreshState])

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
