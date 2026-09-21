import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { RequireBackofficeAuth } from "@/components/routing/RequireBackofficeAuth"
import { BackofficeAuthProvider } from "@/features/backoffice/auth/context"
import { clearBackofficeAuth, setBackofficeAccessToken } from "@/features/backoffice/auth/storage"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { apiUrl, backofficeAuthHandlers } from "@/test/handlers"
import { __resetHttpForTests } from "@/lib/http"

// The guard on /admin/*. Its job is to answer "is there a back-office
// session" in three states, and the expensive mistake is the middle one:
// treating "still resolving" as "logged out" flashes the login page at an
// operator who is signed in, and treating it as "logged in" renders the
// admin surface to someone who may not be.

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} data-search={loc.search} />
}

function renderGuard(initialPath = "/admin/users") {
  return renderWithProviders({
    initialPath,
    routes: (
      <>
        <Route
          path="/admin/*"
          element={
            <BackofficeAuthProvider>
              <RequireBackofficeAuth fallback={<div data-testid="pending" />}>
                <div data-testid="admin-surface" />
              </RequireBackofficeAuth>
            </BackofficeAuthProvider>
          }
        />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

beforeEach(() => {
  window.localStorage.clear()
  clearBackofficeAuth()
  __resetHttpForTests()
})

describe("<RequireBackofficeAuth />", () => {
  it("renders the admin surface for a signed-in operator", async () => {
    setBackofficeAccessToken("op-token")
    server.use(...backofficeAuthHandlers.signedIn())

    renderGuard()

    expect(await screen.findByTestId("admin-surface")).toBeInTheDocument()
  })

  it("bounces to the back-office login with where the operator was going", async () => {
    // No token at all: the plane is definitively logged out, no probe needed.
    renderGuard("/admin/users/u-1")

    await waitFor(() =>
      expect(screen.getByTestId("loc")).toHaveAttribute("data-pathname", "/backoffice/login")
    )
    const search = screen.getByTestId("loc").getAttribute("data-search") ?? ""
    const params = new URLSearchParams(search)
    // Both halves matter: `redirect` is what returns the operator to the page
    // they asked for, `reason` is what makes the login page say why.
    expect(params.get("redirect")).toBe("/admin/users/u-1")
    expect(params.get("reason")).toBe("auth_required")
  })

  it("bounces when the back-office token is rejected", async () => {
    setBackofficeAccessToken("expired-token")
    server.use(
      msw.get(apiUrl("/backoffice/auth/me"), () =>
        HttpResponse.json({ error: "unauthorized" }, { status: 401 })
      ),
      msw.post(apiUrl("/backoffice/auth/refresh"), () =>
        HttpResponse.json({ error: "unauthorized" }, { status: 401 })
      )
    )

    renderGuard()

    await waitFor(() =>
      expect(screen.getByTestId("loc")).toHaveAttribute("data-pathname", "/backoffice/login")
    )
    expect(screen.queryByTestId("admin-surface")).not.toBeInTheDocument()
  })

  it("holds on the fallback while the probe is still out", async () => {
    setBackofficeAccessToken("op-token")
    let release: () => void = () => {}
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    server.use(
      msw.get(apiUrl("/backoffice/auth/me"), async () => {
        await gate
        return HttpResponse.json(backofficeAuthHandlers.fixtureOperator)
      })
    )

    renderGuard()

    // Not the login page: an operator with a token that has not been checked
    // yet must not see a redirect, or every admin page load flashes login.
    expect(await screen.findByTestId("pending")).toBeInTheDocument()
    expect(screen.queryByTestId("loc")).not.toBeInTheDocument()
    expect(screen.queryByTestId("admin-surface")).not.toBeInTheDocument()

    release()
    expect(await screen.findByTestId("admin-surface")).toBeInTheDocument()
  })

  it("holds on the fallback when the probe fails for a reason that is not 401", async () => {
    setBackofficeAccessToken("op-token")
    server.use(
      msw.get(apiUrl("/backoffice/auth/me"), () =>
        HttpResponse.json({ error: "boom" }, { status: 500 })
      )
    )

    renderGuard()

    // A 500 says nothing about the session. Signing the operator out on a
    // backend blip would be a worse answer than waiting.
    expect(await screen.findByTestId("pending")).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByTestId("loc")).not.toBeInTheDocument())
    expect(screen.queryByTestId("admin-surface")).not.toBeInTheDocument()
  })
})
