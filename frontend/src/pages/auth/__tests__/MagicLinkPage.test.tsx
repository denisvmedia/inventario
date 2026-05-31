import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { MagicLinkPage } from "@/pages/auth/MagicLinkPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, getAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

// Mirrors the LoginPage suite: a catch-all route renders the current
// pathname so we can assert the auto-verify-on-mount redirect lands on "/"
// (MagicLinkPage navigates via useNavigate, not <Navigate>, so we read the
// resolved location rather than spying on the router).
function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} data-search={loc.search} />
}

function renderMagicLink(path: string) {
  return renderWithProviders({
    initialPath: path,
    routes: (
      <>
        <Route
          path="/magic-link"
          element={
            <AuthProvider>
              <MagicLinkPage />
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
})

describe("<MagicLinkPage />", () => {
  it("renders the error state and the /login affordance when ?token is absent", async () => {
    let verifyCalls = 0
    server.use(
      msw.post(api("/auth/magic-link/verify"), () => {
        verifyCalls++
        return HttpResponse.json({ access_token: "tok" })
      })
    )
    renderMagicLink("/magic-link")
    expect(screen.getByTestId("magic-link-error")).toBeInTheDocument()
    // The "request a new link" CTA points back at /login.
    const links = screen.getAllByRole("link")
    expect(links.some((a) => a.getAttribute("href") === "/login")).toBe(true)
    // No token → the verify endpoint is never hit.
    expect(verifyCalls).toBe(0)
  })

  it("verifies the token and redirects to / on a non-MFA success", async () => {
    server.use(
      msw.post(api("/auth/magic-link/verify"), async ({ request }) => {
        const body = (await request.json()) as { token: string }
        expect(body.token).toBe("magic-1")
        return HttpResponse.json({
          access_token: "tok",
          csrf_token: "csrf",
          user: { id: "u1", email: "alex@example.com", name: "Alex" },
        })
      })
    )
    renderMagicLink("/magic-link?token=magic-1")
    await waitFor(() => expect(getAccessToken()).toBe("tok"))
    await waitFor(() => expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/"))
  })

  it("hands off to the MFA challenge when the server requires a code", async () => {
    server.use(
      msw.post(api("/auth/magic-link/verify"), () =>
        HttpResponse.json({
          mfa_required: true,
          mfa_token: "challenge-jwt",
          expires_in: 300,
          email: "alex@example.com",
        })
      )
    )
    renderMagicLink("/magic-link?token=magic-1")
    expect(await screen.findByTestId("mfa-challenge")).toBeInTheDocument()
    // No tokens stored yet — the session waits on the second step.
    expect(getAccessToken()).toBeNull()
    expect(screen.queryByTestId("magic-link-verifying")).not.toBeInTheDocument()
  })

  it("renders the error state with the request-a-new-link CTA when the token is invalid or expired", async () => {
    server.use(
      msw.post(api("/auth/magic-link/verify"), () =>
        HttpResponse.json({ error: "invalid or expired token" }, { status: 400 })
      )
    )
    renderMagicLink("/magic-link?token=stale")
    await waitFor(() => expect(screen.getByTestId("magic-link-error")).toBeInTheDocument())
    const links = screen.getAllByRole("link")
    expect(links.some((a) => a.getAttribute("href") === "/login")).toBe(true)
    expect(getAccessToken()).toBeNull()
  })
})
