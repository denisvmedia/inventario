import { createContext, useContext, useMemo, type ReactNode } from "react"

import { HttpError } from "@/lib/http"

import { useBackofficeLogout, useBackofficeMe } from "./hooks"
import { getBackofficeAccessToken } from "./storage"
import type { BackofficeUser } from "./api"

interface BackofficeAuthContextValue {
  // The currently signed-in back-office operator. Tri-state mirrors
  // AuthContext: object | null | undefined.
  user: BackofficeUser | undefined | null
  // True once the probe has settled (or there is no token to probe).
  isInitialized: boolean
  // Convenience flag — only true when `user` is an actual operator.
  isAuthenticated: boolean
  // Imperative logout for the chrome (operator card "Sign out" button).
  logout: () => Promise<void>
}

const Context = createContext<BackofficeAuthContextValue | undefined>(undefined)

interface BackofficeAuthProviderProps {
  children: ReactNode
}

// BackofficeAuthProvider mirrors AuthProvider for the back-office plane
// (#1785 Phase 6). Lives inside the /backoffice/* and /admin/* subtrees
// only — the tenant shell never mounts this, and the back-office shell
// never mounts the tenant AuthProvider. Both can coexist when a browser
// is signed into both planes at once.
//
// Unlike AuthProvider, this one does NOT install a navigation handler
// for 401s — the global handler installed by AuthProvider already does
// that, and it knows about the `plane` parameter (#1785 Phase 6) to
// route back-office 401s to /backoffice/login. Mounting two installers
// would last-writer-wins, which is the bug we're avoiding.
export function BackofficeAuthProvider({ children }: BackofficeAuthProviderProps) {
  const { data: user, isFetched, error } = useBackofficeMe()
  const logoutMutation = useBackofficeLogout()

  const value = useMemo<BackofficeAuthContextValue>(() => {
    const hasToken = !!getBackofficeAccessToken()
    const authError = error instanceof HttpError && error.status === 401
    const transientError = !!error && !authError

    const isInitialized = !hasToken || (isFetched && !transientError)

    let resolvedUser: BackofficeUser | undefined | null
    if (!hasToken) {
      resolvedUser = null
    } else if (authError) {
      resolvedUser = null
    } else if (transientError) {
      resolvedUser = undefined
    } else if (user) {
      resolvedUser = user
    } else if (isFetched) {
      // 200 with no body — backend bug. Treat as logged out so the
      // operator lands on /backoffice/login rather than spinning.
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

export function useBackofficeAuth(): BackofficeAuthContextValue {
  const ctx = useContext(Context)
  if (!ctx) throw new Error("useBackofficeAuth must be used inside <BackofficeAuthProvider>")
  return ctx
}

// Optional variant for chrome components that may render outside the
// provider (mirrors useOptionalAuth). Returns undefined when no
// provider is mounted instead of throwing.
export function useOptionalBackofficeAuth(): BackofficeAuthContextValue | undefined {
  return useContext(Context)
}
