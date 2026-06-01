import { beforeEach, describe, expect, it } from "vitest"
import { delay, http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { RootGate } from "@/app/RootGate"
import { AuthProvider } from "@/features/auth/AuthContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} />
}

function renderRootGate() {
  return renderWithProviders({
    initialPath: "/",
    routes: (
      <>
        <Route
          path="/"
          element={
            <AuthProvider>
              <RootGate />
            </AuthProvider>
          }
        />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  // The landing page (anon branch) reads /feature-flags on mount via
  // useFeatureFlag; provide a default so MSW doesn't error.
  server.use(
    msw.get(api("/feature-flags"), () =>
      HttpResponse.json({ currency_migration: false, magic_link_login: false, public_scan: false })
    )
  )
})

describe("<RootGate />", () => {
  it("renders the landing page for an anonymous visitor", async () => {
    // No token + 401 /auth/me → user === null → LandingPage.
    server.use(msw.get(api("/auth/me"), () => HttpResponse.json(null, { status: 401 })))
    renderRootGate()
    expect(await screen.findByTestId("landing-page")).toBeInTheDocument()
  })

  it("resolves an authed user to their group dashboard via RootRedirect", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u1",
          email: "denis@example.com",
          name: "Denis",
          default_group_id: "g1",
        })
      ),
      msw.get(api("/groups"), () =>
        HttpResponse.json({
          data: [
            {
              id: "g1",
              type: "groups",
              attributes: { id: "g1", slug: "household", name: "Household" },
            },
          ],
        })
      )
    )
    renderRootGate()
    // RootRedirect (wrapped in GroupProvider) bounces to /g/<slug>.
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household")
    )
    // The landing page must NOT have flashed.
    expect(screen.queryByTestId("landing-page")).not.toBeInTheDocument()
  })

  it("renders nothing while the auth probe is still in flight (boot guard)", async () => {
    setAccessToken("good-token")
    // Hold /auth/me open so the probe never settles within the window —
    // with a token present, isInitialized stays false and the gate's boot
    // guard must render neither the landing page nor a redirect.
    server.use(
      msw.get(api("/auth/me"), async () => {
        await delay(10_000)
        return HttpResponse.json({ id: "u1", email: "denis@example.com", name: "Denis" })
      })
    )
    renderRootGate()
    await new Promise((r) => setTimeout(r, 50))
    expect(screen.queryByTestId("landing-page")).not.toBeInTheDocument()
    expect(screen.queryByTestId("loc")).not.toBeInTheDocument()
  })
})
