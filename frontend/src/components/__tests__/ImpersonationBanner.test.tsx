import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { screen, waitFor } from "@testing-library/react"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import { ImpersonationBanner } from "@/components/ImpersonationBanner"
import { AuthProvider } from "@/features/auth/AuthContext"
import { ImpersonationProvider } from "@/features/admin/impersonation/ImpersonationContext"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

// Mounts the banner under a real AuthProvider + ImpersonationProvider so
// the visibility test exercises the same context wiring as production.
function renderBanner() {
  return renderWithProviders({
    initialPath: "/",
    children: (
      <AuthProvider>
        <ImpersonationProvider>
          <ImpersonationBanner />
        </ImpersonationProvider>
      </AuthProvider>
    ),
  })
}

describe("ImpersonationBanner", () => {
  it("renders nothing when no impersonation session is active", async () => {
    setAccessToken("good-token")
    let authMeCalls = 0
    let impersonationCalls = 0
    server.use(
      msw.get(api("/auth/me"), () => {
        authMeCalls++
        return HttpResponse.json({ id: "u1", email: "admin@example.com", name: "Admin" })
      }),
      msw.get(api("/admin/impersonation/current"), () => {
        impersonationCalls++
        return HttpResponse.json({ active: false })
      })
    )
    renderBanner()
    // Positive completion signal: wait until both probes have actually been
    // hit before asserting the banner is absent — the absent state is also
    // true before the queries resolve, so a bare waitFor would pass early.
    await waitFor(() => {
      expect(authMeCalls).toBeGreaterThan(0)
      expect(impersonationCalls).toBeGreaterThan(0)
    })
    expect(screen.queryByTestId("impersonation-banner")).toBeNull()
  })

  it("stays hidden when the endpoint 403s for a non-admin user", async () => {
    setAccessToken("good-token")
    let authMeCalls = 0
    let impersonationCalls = 0
    server.use(
      msw.get(api("/auth/me"), () => {
        authMeCalls++
        return HttpResponse.json({ id: "u9", email: "plain@example.com", name: "Plain" })
      }),
      // A plain user 403s on /admin/impersonation/current — the api layer
      // translates that into `{ active: false }`, so the banner hides.
      msw.get(api("/admin/impersonation/current"), () => {
        impersonationCalls++
        return HttpResponse.json({ errors: [] }, { status: 403 })
      })
    )
    renderBanner()
    // Positive completion signal: wait until both probes have actually been
    // hit (the 403 included) before asserting the banner stays absent.
    await waitFor(() => {
      expect(authMeCalls).toBeGreaterThan(0)
      expect(impersonationCalls).toBeGreaterThan(0)
    })
    expect(screen.queryByTestId("impersonation-banner")).toBeNull()
  })

  it("renders the banner with the target name and an End button when active", async () => {
    setAccessToken("good-token")
    const expiresAt = new Date(Date.now() + 20 * 60 * 1000).toISOString()
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "admin@example.com", name: "Admin" })
      ),
      msw.get(api("/admin/impersonation/current"), () =>
        HttpResponse.json({
          active: true,
          target_user: { id: "t1", name: "Target User", email: "target@example.com" },
          admin_user: { id: "u1", name: "Admin", email: "admin@example.com" },
          started_at: new Date().toISOString(),
          expires_at: expiresAt,
        })
      )
    )
    renderBanner()
    await waitFor(() => expect(screen.getByTestId("impersonation-banner")).toBeInTheDocument())
    expect(screen.getByText(/Target User/)).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /end impersonation/i })).toBeInTheDocument()
  })
})
