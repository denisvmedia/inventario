import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { LoginPage } from "@/pages/auth/LoginPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, getAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearPendingInvite, savePendingInvite } from "@/features/auth/inviteHandoff"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} data-search={loc.search} />
}

function renderLogin(initial = "/login") {
  return renderWithProviders({
    initialPath: initial,
    routes: (
      <>
        <Route
          path="/login"
          element={
            <AuthProvider>
              <LoginPage />
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
  clearPendingInvite()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<LoginPage />", () => {
  it("validates required fields before submitting", async () => {
    const user = userEvent.setup()
    renderLogin()
    await user.click(screen.getByTestId("login-button"))
    expect(await screen.findByTestId("email-error")).toBeInTheDocument()
    expect(screen.getByTestId("password-error")).toBeInTheDocument()
  })

  it("renders the session message when ?reason=session_expired is present", () => {
    renderLogin("/login?reason=session_expired")
    expect(screen.getByTestId("session-message")).toHaveTextContent(/session has expired/i)
  })

  it("submits credentials, stores the token, and navigates on success", async () => {
    server.use(
      msw.post(api("/auth/login"), async ({ request }) => {
        const body = (await request.json()) as { email: string; password: string }
        expect(body).toEqual({ email: "alex@example.com", password: "secret-pw" })
        return HttpResponse.json({
          access_token: "tok",
          csrf_token: "csrf",
          user: { id: "u1", email: "alex@example.com", name: "Alex" },
        })
      })
    )
    const user = userEvent.setup()
    renderLogin("/login?redirect=/g/household")
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("login-button"))

    await waitFor(() => expect(getAccessToken()).toBe("tok"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household")
    )
  })

  it("surfaces the server error inline when the API returns 401", async () => {
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({ error: "Invalid credentials" }, { status: 401 })
      )
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "wrong")
    await user.click(screen.getByTestId("login-button"))
    expect(await screen.findByTestId("server-error")).toHaveTextContent(/invalid credentials/i)
  })

  it("auto-accepts a pending invite after successful login", async () => {
    savePendingInvite({ token: "inv-tok", groupName: "Household" })
    let acceptCalls = 0
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({
          access_token: "tok",
          csrf_token: "csrf",
          user: { id: "u1", email: "alex@example.com", name: "Alex" },
        })
      ),
      msw.post(api("/invites/inv-tok/accept"), () => {
        acceptCalls++
        return HttpResponse.json({ data: { id: "m1", attributes: { group_id: "g1" } } })
      })
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret")
    await user.click(screen.getByTestId("login-button"))
    await waitFor(() => expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/"))
    expect(acceptCalls).toBe(1)
  })

  it("has no axe violations on the form", async () => {
    const { container } = renderLogin()
    expect(await axe(container)).toHaveNoViolations()
  })

  it("falls back to / when ?redirect= points off the app (open-redirect guard)", async () => {
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({
          access_token: "tok",
          csrf_token: "csrf",
          user: { id: "u1", email: "alex@example.com", name: "Alex" },
        })
      )
    )
    const user = userEvent.setup()
    renderLogin("/login?redirect=//evil.example/foo")
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret")
    await user.click(screen.getByTestId("login-button"))
    await waitFor(() => expect(getAccessToken()).toBe("tok"))
    await waitFor(() => expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/"))
  })
})
