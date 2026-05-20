import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import { RequireSystemAdmin } from "@/components/routing/RequireSystemAdmin"
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

function AdminContent() {
  return <div data-testid="admin-content">admin</div>
}

function buildRoutes() {
  return (
    <Route
      path="/admin"
      element={
        <AuthProvider>
          <RequireSystemAdmin>
            <AdminContent />
          </RequireSystemAdmin>
        </AuthProvider>
      }
    />
  )
}

describe("RequireSystemAdmin", () => {
  it("renders the admin child for a system administrator", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u1",
          email: "admin@example.com",
          name: "Admin",
          is_system_admin: true,
        })
      )
    )
    renderWithProviders({ initialPath: "/admin", routes: buildRoutes() })
    await waitFor(() => expect(screen.getByTestId("admin-content")).toBeInTheDocument())
    expect(screen.queryByTestId("admin-forbidden")).toBeNull()
  })

  it("shows the 403 page for a signed-in non-admin user (no redirect, no crash)", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u2",
          email: "user@example.com",
          name: "User",
          is_system_admin: false,
        })
      )
    )
    renderWithProviders({ initialPath: "/admin", routes: buildRoutes() })
    await waitFor(() => expect(screen.getByTestId("admin-forbidden")).toBeInTheDocument())
    expect(screen.queryByTestId("admin-content")).toBeNull()
  })

  it("treats a user with the flag absent as a non-admin", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u3", email: "legacy@example.com", name: "Legacy" })
      )
    )
    renderWithProviders({ initialPath: "/admin", routes: buildRoutes() })
    await waitFor(() => expect(screen.getByTestId("admin-forbidden")).toBeInTheDocument())
    expect(screen.queryByTestId("admin-content")).toBeNull()
  })
})
