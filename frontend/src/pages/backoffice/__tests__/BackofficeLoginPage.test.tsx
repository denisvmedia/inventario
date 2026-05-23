import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { BackofficeLoginPage } from "@/pages/backoffice/BackofficeLoginPage"
import { BackofficeAuthProvider } from "@/features/backoffice/auth/context"
import { clearBackofficeAuth, getBackofficeAccessToken } from "@/features/backoffice/auth/storage"
import { __resetHttpForTests } from "@/lib/http"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} data-search={loc.search} />
}

function renderLogin(initial = "/backoffice/login") {
  return renderWithProviders({
    initialPath: initial,
    routes: (
      <>
        <Route
          path="/backoffice/login"
          element={
            <BackofficeAuthProvider>
              <BackofficeLoginPage />
            </BackofficeAuthProvider>
          }
        />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

beforeEach(() => {
  clearBackofficeAuth()
  __resetHttpForTests()
})

describe("<BackofficeLoginPage />", () => {
  it("submits credentials, stores the back-office token, and navigates to /admin/tenants", async () => {
    server.use(
      msw.post(api("/backoffice/auth/login"), async ({ request }) => {
        const body = (await request.json()) as { email: string; password: string }
        expect(body).toEqual({ email: "operator@example.com", password: "secret-pw" })
        return HttpResponse.json({
          access_token: "bo-tok",
          token_type: "Bearer",
          expires_in: 600,
          user: { id: "op-1", email: "operator@example.com", name: "Operator" },
        })
      })
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("backoffice-email"), "operator@example.com")
    await user.type(screen.getByTestId("backoffice-password"), "secret-pw")
    await user.click(screen.getByTestId("backoffice-login-button"))

    await waitFor(() => expect(getBackofficeAccessToken()).toBe("bo-tok"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/admin/tenants")
    )
  })

  it("swaps the form for the MFA challenge when the server returns mfa_required", async () => {
    server.use(
      msw.post(api("/backoffice/auth/login"), () =>
        HttpResponse.json({
          mfa_required: true,
          mfa_token: "challenge-jwt",
          expires_in: 300,
          email: "operator@example.com",
        })
      )
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("backoffice-email"), "operator@example.com")
    await user.type(screen.getByTestId("backoffice-password"), "secret-pw")
    await user.click(screen.getByTestId("backoffice-login-button"))

    expect(await screen.findByTestId("backoffice-mfa-challenge")).toBeInTheDocument()
    expect(screen.queryByTestId("backoffice-login-page")).not.toBeInTheDocument()
    // No tokens yet — step-2 owns persistence.
    expect(getBackofficeAccessToken()).toBeNull()
  })

  it("renders the enrollment-missing alert with the CLI command on a 501", async () => {
    server.use(
      msw.post(api("/backoffice/auth/login"), () =>
        HttpResponse.json(
          {
            mfa_required: true,
            code: "backoffice.mfa_not_implemented",
            email: "operator@example.com",
            mfa_token: "",
            expires_in: 0,
          },
          { status: 501 }
        )
      )
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("backoffice-email"), "operator@example.com")
    await user.type(screen.getByTestId("backoffice-password"), "secret-pw")
    await user.click(screen.getByTestId("backoffice-login-button"))

    const banner = await screen.findByTestId("backoffice-mfa-not-enrolled")
    // The CLI command is rendered verbatim with the email substituted so the
    // operator can copy + paste it onto the server.
    expect(banner).toHaveTextContent("inventario backoffice mfa setup --email operator@example.com")
    // The form is still mounted — the operator may want to try a different
    // email after they enrol, without leaving the page.
    expect(screen.getByTestId("backoffice-login-page")).toBeInTheDocument()
    expect(getBackofficeAccessToken()).toBeNull()
  })

  it("surfaces the server error inline on a 401", async () => {
    server.use(
      msw.post(api("/backoffice/auth/login"), () =>
        HttpResponse.json({ error: "Invalid credentials" }, { status: 401 })
      )
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("backoffice-email"), "operator@example.com")
    await user.type(screen.getByTestId("backoffice-password"), "wrong")
    await user.click(screen.getByTestId("backoffice-login-button"))

    expect(await screen.findByTestId("backoffice-server-error")).toHaveTextContent(
      /invalid credentials/i
    )
  })
})
