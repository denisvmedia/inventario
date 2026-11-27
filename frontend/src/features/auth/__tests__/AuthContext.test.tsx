import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, act } from "@testing-library/react"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import type { QueryClient } from "@tanstack/react-query"

import { AuthProvider, useAuth } from "@/features/auth/AuthContext"
import { useLogin } from "@/features/auth/hooks"
import { authKeys } from "@/features/auth/keys"
import { groupKeys } from "@/features/group/keys"
import { ProtectedRoute } from "@/components/routing/ProtectedRoute"
import { __resetBootRefreshForTests } from "@/features/auth/bootRefresh"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests, http } from "@/lib/http"
import { clearAuth, getAccessToken, setAccessToken } from "@/lib/auth-storage"
import { fakeAccessToken } from "@/test/handlers/auth"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  __resetBootRefreshForTests()
})

function Probe() {
  const auth = useAuth()
  return (
    <div
      data-testid="probe"
      data-initialized={String(auth.isInitialized)}
      data-authenticated={String(auth.isAuthenticated)}
      data-email={auth.user?.email ?? ""}
    >
      <button type="button" onClick={() => void auth.logout()}>
        sign out
      </button>
    </div>
  )
}

function LoginEcho() {
  const loc = useLocation()
  return (
    <div data-testid="login-stub" data-search={loc.search}>
      login
    </div>
  )
}

const routes = (
  <>
    <Route
      path="/"
      element={
        <AuthProvider>
          <Probe />
        </AuthProvider>
      }
    />
    <Route path="/login" element={<LoginEcho />} />
  </>
)

describe("useAuth", () => {
  it("starts initialized=true and authenticated=false when no access token is present and boot-refresh fails", async () => {
    // Boot-refresh (#1394) speculatively probes /auth/refresh on mount. With no
    // refresh cookie the BE returns 401 and AuthContext settles into the
    // unauthenticated branch.
    server.use(msw.post(api("/auth/refresh"), () => HttpResponse.json(null, { status: 401 })))
    renderWithProviders({ initialPath: "/", routes })
    await waitFor(() =>
      expect(screen.getByTestId("probe").getAttribute("data-initialized")).toBe("true")
    )
    expect(screen.getByTestId("probe").getAttribute("data-authenticated")).toBe("false")
  })

  // #1394 — OAuth callback 302s the browser back into the SPA with NO access
  // token in localStorage but a valid httpOnly refresh cookie. The boot-time
  // /auth/refresh has to succeed and seed the token so the app renders the
  // authenticated shell instead of bouncing the user to /login.
  it("populates the access token from the refresh cookie on boot and renders the shell", async () => {
    let refreshCalls = 0
    server.use(
      msw.post(api("/auth/refresh"), () => {
        refreshCalls++
        return HttpResponse.json({ access_token: "boot-tok", csrf_token: "boot-csrf" })
      }),
      msw.get(api("/auth/me"), ({ request }) => {
        expect(request.headers.get("authorization")).toBe("Bearer boot-tok")
        return HttpResponse.json({ id: "u1", email: "oauth@example.com", name: "OAuth User" })
      })
    )
    renderWithProviders({ initialPath: "/", routes })
    await waitFor(() =>
      expect(screen.getByTestId("probe").getAttribute("data-authenticated")).toBe("true")
    )
    expect(screen.getByTestId("probe").getAttribute("data-email")).toBe("oauth@example.com")
    expect(getAccessToken()).toBe("boot-tok")
    // One-shot guard: AuthProvider re-renders (incl. StrictMode dev double
    // mounts) must not refire the request.
    expect(refreshCalls).toBe(1)
  })

  it("exposes the signed-in user once /auth/me resolves", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "denis@example.com", name: "Denis" })
      )
    )
    renderWithProviders({ initialPath: "/", routes })
    await waitFor(() =>
      expect(screen.getByTestId("probe").getAttribute("data-authenticated")).toBe("true")
    )
    expect(screen.getByTestId("probe").getAttribute("data-email")).toBe("denis@example.com")
  })

  it("flips authenticated=false after a successful logout", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "denis@example.com", name: "Denis" })
      ),
      msw.post(api("/auth/logout"), () => new HttpResponse(null, { status: 204 }))
    )
    renderWithProviders({ initialPath: "/", routes })
    await waitFor(() =>
      expect(screen.getByTestId("probe").getAttribute("data-authenticated")).toBe("true")
    )
    await act(async () => {
      screen.getByRole("button", { name: /sign out/i }).click()
    })
    await waitFor(() =>
      expect(screen.getByTestId("probe").getAttribute("data-authenticated")).toBe("false")
    )
  })

  it("redirects to /login with a session_expired reason when /auth/me 401s and refresh fails", async () => {
    setAccessToken("bad-token")
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json(null, { status: 401 })),
      msw.post(api("/auth/refresh"), () => HttpResponse.json(null, { status: 401 }))
    )
    renderWithProviders({ initialPath: "/", routes })
    await waitFor(() => expect(screen.getByTestId("login-stub")).toBeInTheDocument())
    const search = screen.getByTestId("login-stub").getAttribute("data-search") ?? ""
    const params = new URLSearchParams(search)
    expect(params.get("reason")).toBe("session_expired")
    expect(params.get("redirect")).toBe("/")
  })

  // #1038 — a non-401 probe failure (transient 5xx / network blip) is NOT a
  // logout. AuthContext must hold the boot fallback: user stays `undefined`,
  // isInitialized stays false, and the user is NOT bounced to /login. (Uses
  // 500, not 503 — a 503 bounces the whole shell to /maintenance.)
  it("holds the boot fallback on a transient /auth/me error instead of logging out", async () => {
    let meCalls = 0
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () => {
        meCalls++
        return HttpResponse.json({ error: "boom" }, { status: 500 })
      })
    )
    renderWithProviders({ initialPath: "/", routes })
    await waitFor(() => expect(meCalls).toBeGreaterThan(0))
    await waitFor(() => {
      const probe = screen.getByTestId("probe")
      expect(probe.getAttribute("data-initialized")).toBe("false")
      expect(probe.getAttribute("data-authenticated")).toBe("false")
    })
    // Transient error must not redirect — the probe is still mounted.
    expect(screen.queryByTestId("login-stub")).not.toBeInTheDocument()
  })

  // #1038 — logging out clears stored credentials and, on a guarded route,
  // drops the user to `null` so ProtectedRoute redirects to /login.
  it("clears credentials and redirects a guarded route to /login on logout", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "denis@example.com", name: "Denis" })
      ),
      msw.post(api("/auth/logout"), () => new HttpResponse(null, { status: 204 }))
    )
    const guardedRoutes = (
      <>
        <Route
          path="/"
          element={
            <AuthProvider>
              <ProtectedRoute fallback={<div data-testid="boot" />}>
                <Probe />
              </ProtectedRoute>
            </AuthProvider>
          }
        />
        <Route path="/login" element={<LoginEcho />} />
      </>
    )
    renderWithProviders({ initialPath: "/", routes: guardedRoutes })
    await waitFor(() =>
      expect(screen.getByTestId("probe").getAttribute("data-authenticated")).toBe("true")
    )
    await act(async () => {
      screen.getByRole("button", { name: /sign out/i }).click()
    })
    await waitFor(() => expect(screen.getByTestId("login-stub")).toBeInTheDocument())
    expect(getAccessToken()).toBeNull()
  })
})

// A change of the signed-in user must leave nothing in the query cache that
// the previous user fetched: most keys (["group", "list"] among them) do not
// name the user, and a fresh entry would be shown to the next user as is.
describe("query cache across a change of the signed-in user", () => {
  const tokenA = fakeAccessToken("user-a")
  const tokenB = fakeAccessToken("user-b")
  const userA = { id: "user-a", email: "a@example.com", name: "Alice" }
  const userB = { id: "user-b", email: "b@example.com", name: "Bob" }
  const groupsOfA = [{ id: "g-a", slug: "alice-home", name: "Alice's home" }]

  function SessionProbe() {
    const auth = useAuth()
    const login = useLogin()
    return (
      <div
        data-testid="session"
        data-authenticated={String(auth.isAuthenticated)}
        data-email={auth.user?.email ?? ""}
      >
        <button type="button" onClick={() => void auth.logout()}>
          sign out
        </button>
        <button
          type="button"
          onClick={() => login.mutate({ email: userB.email, password: "secret" })}
        >
          sign in as B
        </button>
      </div>
    )
  }

  const sessionRoutes = (
    <>
      <Route
        path="/"
        element={
          <AuthProvider>
            <SessionProbe />
          </AuthProvider>
        }
      />
      <Route path="/login" element={<LoginEcho />} />
    </>
  )

  function installSessionHandlers() {
    server.use(
      msw.get(api("/auth/me"), ({ request }) => {
        const auth = request.headers.get("authorization")
        if (auth === `Bearer ${tokenA}`) return HttpResponse.json(userA)
        if (auth === `Bearer ${tokenB}`) return HttpResponse.json(userB)
        return HttpResponse.json(null, { status: 401 })
      }),
      msw.post(api("/auth/logout"), () => new HttpResponse(null, { status: 204 })),
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({ access_token: tokenB, csrf_token: "csrf-b", user: userB })
      )
    )
  }

  // Fills the cache the way user A's session would: a list whose key does
  // not name the user, and a finished mutation.
  function fillCacheAsA(queryClient: QueryClient) {
    queryClient.setQueryData(groupKeys.list(), groupsOfA)
    queryClient.getMutationCache().build(queryClient, { mutationKey: ["note", "save"] })
  }

  function cachedQueryData(queryClient: QueryClient): string {
    return JSON.stringify(
      queryClient
        .getQueryCache()
        .getAll()
        .map((q) => q.state.data)
        .filter((d) => d !== undefined)
    )
  }

  async function signedInAsA() {
    setAccessToken(tokenA)
    installSessionHandlers()
    const rendered = renderWithProviders({ initialPath: "/", routes: sessionRoutes })
    await waitFor(() =>
      expect(screen.getByTestId("session").getAttribute("data-email")).toBe(userA.email)
    )
    fillCacheAsA(rendered.queryClient)
    return rendered
  }

  function otherTabWritesToken(oldValue: string | null, newValue: string | null) {
    if (newValue === null) window.localStorage.removeItem("inventario_token")
    else window.localStorage.setItem("inventario_token", newValue)
    window.dispatchEvent(
      new StorageEvent("storage", {
        key: "inventario_token",
        oldValue,
        newValue,
        storageArea: window.localStorage,
      })
    )
  }

  it("drops A's queries and mutations on sign-out and shows B only B's data", async () => {
    const { queryClient } = await signedInAsA()

    await act(async () => {
      screen.getByRole("button", { name: "sign out" }).click()
    })
    await waitFor(() =>
      expect(screen.getByTestId("session").getAttribute("data-authenticated")).toBe("false")
    )
    expect(queryClient.getQueryData(groupKeys.list())).toBeUndefined()
    expect(cachedQueryData(queryClient)).toBe("[]")
    expect(queryClient.getMutationCache().find({ mutationKey: ["note", "save"] })).toBeUndefined()

    await act(async () => {
      screen.getByRole("button", { name: "sign in as B" }).click()
    })
    await waitFor(() =>
      expect(screen.getByTestId("session").getAttribute("data-email")).toBe(userB.email)
    )
    expect(queryClient.getQueryData(authKeys.currentUser())).toMatchObject({ id: userB.id })
    expect(queryClient.getQueryData(groupKeys.list())).toBeUndefined()
    const remaining = cachedQueryData(queryClient)
    expect(remaining).not.toContain(userA.email)
    expect(remaining).not.toContain("alice-home")
    expect(queryClient.getMutationCache().find({ mutationKey: ["note", "save"] })).toBeUndefined()
  })

  it("clears the cache when a failed refresh signs the user out", async () => {
    const { queryClient } = await signedInAsA()
    server.use(
      msw.get(api("/groups"), () => HttpResponse.json(null, { status: 401 })),
      msw.post(api("/auth/refresh"), () => HttpResponse.json(null, { status: 401 }))
    )

    await act(async () => {
      await http.get("/groups").catch(() => undefined)
    })

    expect(queryClient.getQueryData(groupKeys.list())).toBeUndefined()
    expect(cachedQueryData(queryClient)).not.toContain(userA.email)
    await waitFor(() => expect(screen.getByTestId("login-stub")).toBeInTheDocument())
  })

  it("clears the cache when another tab signs out", async () => {
    const { queryClient } = await signedInAsA()

    act(() => otherTabWritesToken(tokenA, null))

    expect(queryClient.getQueryData(groupKeys.list())).toBeUndefined()
    await waitFor(() =>
      expect(screen.getByTestId("session").getAttribute("data-authenticated")).toBe("false")
    )
  })

  it("clears the cache when another tab signs in as a different user", async () => {
    const { queryClient } = await signedInAsA()

    act(() => otherTabWritesToken(tokenA, tokenB))

    expect(queryClient.getQueryData(groupKeys.list())).toBeUndefined()
    await waitFor(() =>
      expect(screen.getByTestId("session").getAttribute("data-email")).toBe(userB.email)
    )
    expect(cachedQueryData(queryClient)).not.toContain(userA.email)
  })

  it("keeps the cache when another tab refreshes the same user's token", async () => {
    const { queryClient } = await signedInAsA()

    act(() => otherTabWritesToken(tokenA, fakeAccessToken("user-a", "refreshed")))

    expect(queryClient.getQueryData(groupKeys.list())).toEqual(groupsOfA)
    expect(screen.getByTestId("session").getAttribute("data-email")).toBe(userA.email)
  })
})
