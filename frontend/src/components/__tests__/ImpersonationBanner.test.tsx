import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import { ImpersonationBanner } from "@/components/ImpersonationBanner"
import { AuthProvider } from "@/features/auth/AuthContext"
import { ImpersonationProvider } from "@/features/admin/impersonation/ImpersonationContext"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import {
  clearAuth,
  getImpersonationReturn,
  setAccessToken,
  setImpersonationReturn,
} from "@/lib/auth-storage"
import { __resetNavigationForTests, setHardRedirect } from "@/lib/navigation"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  __resetNavigationForTests()
})

afterEach(() => {
  __resetNavigationForTests()
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

  it("stays hidden when the endpoint 401s for a tenant-only user", async () => {
    setAccessToken("good-token")
    let authMeCalls = 0
    let impersonationCalls = 0
    server.use(
      msw.get(api("/auth/me"), () => {
        authMeCalls++
        return HttpResponse.json({ id: "u9", email: "plain@example.com", name: "Plain" })
      }),
      // A plain tenant user 401s on /admin/impersonation/current — the
      // endpoint is gated by RequireBackofficeAuthOrImpersonating (#1785
      // Phase 5), which rejects bare tenant tokens with 401, not 403.
      // The api layer translates that into `{ active: false }` and
      // crucially does NOT let the http client treat the 401 as a
      // back-office session expiry (it would otherwise refresh-bounce the
      // tenant user to /backoffice/login since the path matches
      // isBackofficePath).
      msw.get(api("/admin/impersonation/current"), () => {
        impersonationCalls++
        return HttpResponse.json({ errors: [] }, { status: 401 })
      })
    )
    const mockSetHardRedirect = vi.fn()
    setHardRedirect(mockSetHardRedirect)
    renderBanner()
    // Positive completion signal: wait until both probes have actually been
    // hit (the 401 included) before asserting the banner stays absent.
    await waitFor(() => {
      expect(authMeCalls).toBeGreaterThan(0)
      expect(impersonationCalls).toBeGreaterThan(0)
    })
    expect(screen.queryByTestId("impersonation-banner")).toBeNull()
    expect(mockSetHardRedirect).not.toHaveBeenCalled()
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
      // Defensive: if any deployment of the gate ever returns 403 (e.g. a
      // future tightening of the middleware), the api layer still treats
      // it as "not impersonating" so the banner stays hidden.
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

  it("renders the banner with the target name and an enabled End button when active", async () => {
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
          operator: { id: "u1", name: "Admin", email: "admin@example.com", role: "platform_admin" },
          started_at: new Date().toISOString(),
          expires_at: expiresAt,
        })
      )
    )
    renderBanner()
    await waitFor(() => expect(screen.getByTestId("impersonation-banner")).toBeInTheDocument())
    expect(screen.getByText(/Target User/)).toBeInTheDocument()
    expect(screen.getByTestId("impersonation-end")).toBeEnabled()
  })

  // Seeds an active impersonation session — used by the End-button cases.
  function seedActiveSession() {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "t1", email: "target@example.com", name: "Target User" })
      ),
      msw.get(api("/admin/impersonation/current"), () =>
        HttpResponse.json({
          active: true,
          target_user: { id: "t1", name: "Target User", email: "target@example.com" },
          operator: { id: "u1", name: "Admin", email: "admin@example.com", role: "platform_admin" },
          started_at: new Date().toISOString(),
          expires_at: new Date(Date.now() + 20 * 60 * 1000).toISOString(),
        })
      )
    )
  }

  it("ends the session: POST /end → clears the return slot → redirects to the target user", async () => {
    setAccessToken("good-token")
    setImpersonationReturn({ targetUserId: "t1" })
    let endCalls = 0
    seedActiveSession()
    server.use(
      msw.post(api("/admin/impersonation/end"), () => {
        endCalls++
        return HttpResponse.json({ access_token: "admin-token", csrf_token: "admin-csrf" })
      })
    )
    const redirect = vi.fn()
    setHardRedirect(redirect)

    renderBanner()
    await waitFor(() => expect(screen.getByTestId("impersonation-end")).toBeEnabled())
    await userEvent.click(screen.getByTestId("impersonation-end"))

    await waitFor(() => expect(endCalls).toBe(1))
    await waitFor(() => expect(redirect).toHaveBeenCalledWith("/admin/users/t1"))
    expect(getImpersonationReturn()).toBeNull()
  })

  it("on End failure: clears auth and redirects to /backoffice/login with the session_expired reason", async () => {
    setAccessToken("good-token")
    setImpersonationReturn({ targetUserId: "t1" })
    seedActiveSession()
    server.use(
      msw.post(api("/admin/impersonation/end"), () =>
        HttpResponse.json({ errors: [{ code: "admin.impersonate.not_active" }] }, { status: 422 })
      )
    )
    const redirect = vi.fn()
    setHardRedirect(redirect)

    renderBanner()
    await waitFor(() => expect(screen.getByTestId("impersonation-end")).toBeEnabled())
    await userEvent.click(screen.getByTestId("impersonation-end"))

    // The hook-level onError carries the `reason` param so the back-office
    // login page renders the "session expired" notice — consistent with the
    // auto-expiry path. Phase 5/6 (#1785) moved end onto the back-office
    // plane, so the recovery surface is /backoffice/login, not /login.
    await waitFor(() =>
      expect(redirect).toHaveBeenCalledWith("/backoffice/login?reason=session_expired")
    )
    expect(getImpersonationReturn()).toBeNull()
  })
})
