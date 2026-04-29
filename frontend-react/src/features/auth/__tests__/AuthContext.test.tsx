import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, act } from "@testing-library/react"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import { AuthProvider, useAuth } from "@/features/auth/AuthContext"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
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
  it("starts initialized=true and authenticated=false when no access token is present", async () => {
    renderWithProviders({ initialPath: "/", routes })
    // No /auth/me probe because the token is missing — we settle synchronously.
    await waitFor(() =>
      expect(screen.getByTestId("probe").getAttribute("data-initialized")).toBe("true")
    )
    expect(screen.getByTestId("probe").getAttribute("data-authenticated")).toBe("false")
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
})
