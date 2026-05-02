import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import { ProtectedRoute } from "@/components/routing/ProtectedRoute"
import { AuthProvider } from "@/features/auth/AuthContext"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

function Protected() {
  return <div data-testid="protected">protected</div>
}

// Login stub also echoes useLocation so the redirect test can read the
// search string back without poking at MemoryRouter internals.
function LoginStub() {
  const loc = useLocation()
  return (
    <div data-testid="login-stub" data-search={loc.search}>
      login
    </div>
  )
}

function buildRoutes() {
  return (
    <>
      <Route
        path="/private"
        element={
          <AuthProvider>
            <ProtectedRoute>
              <Protected />
            </ProtectedRoute>
          </AuthProvider>
        }
      />
      <Route path="/login" element={<LoginStub />} />
    </>
  )
}

describe("ProtectedRoute", () => {
  it("redirects unauthenticated users to /login with a return-to redirect and reason query", async () => {
    server.use(msw.get(api("/auth/me"), () => HttpResponse.json(null, { status: 401 })))
    renderWithProviders({ initialPath: "/private?foo=1", routes: buildRoutes() })
    await waitFor(() => expect(screen.getByTestId("login-stub")).toBeInTheDocument())
    expect(screen.queryByTestId("protected")).toBeNull()
    // The full original path+search must round-trip through the redirect
    // query so #1407's login flow can return the user where they tried
    // to go. A reason code is attached so the login page can show the right
    // copy ("Please sign in" vs "Your session expired").
    const search = screen.getByTestId("login-stub").getAttribute("data-search") ?? ""
    const params = new URLSearchParams(search)
    expect(params.get("redirect")).toBe("/private?foo=1")
    expect(params.get("reason")).toBe("auth_required")
  })

  it("renders the protected child when /auth/me succeeds", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "denis@example.com", name: "Denis" })
      )
    )
    renderWithProviders({ initialPath: "/private", routes: buildRoutes() })
    await waitFor(() => expect(screen.getByTestId("protected")).toBeInTheDocument())
    expect(screen.queryByTestId("login-stub")).toBeNull()
  })

  it("holds the fallback (no redirect to /login) on a transient backend error", async () => {
    setAccessToken("good-token")
    // 503 is a non-auth error. The user has a token; we must not bounce
    // them to /login on a backend blip. http.ts only triggers refresh on
    // 401, so 503 surfaces directly to React Query as an error.
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ error: "service unavailable" }, { status: 503 })
      )
    )
    renderWithProviders({ initialPath: "/private", routes: buildRoutes() })
    // Give React Query a moment to settle the error state. The login stub
    // must NOT appear; the protected child must NOT appear either — fallback
    // (null) is what's rendered.
    await new Promise((r) => setTimeout(r, 50))
    expect(screen.queryByTestId("login-stub")).toBeNull()
    expect(screen.queryByTestId("protected")).toBeNull()
  })
})
