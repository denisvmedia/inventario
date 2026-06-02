import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { RegisterPage } from "@/pages/auth/RegisterPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearPendingInvite } from "@/features/auth/inviteHandoff"
import { clearPendingFirstItem, savePendingFirstItem } from "@/features/auth/firstItemHandoff"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function renderRegister() {
  return renderWithProviders({
    initialPath: "/register",
    routes: (
      <Route
        path="/register"
        element={
          <AuthProvider>
            <RegisterPage />
          </AuthProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  clearPendingInvite()
  clearPendingFirstItem()
  __resetGroupContextForTests()
  __resetHttpForTests()
  // OAuthRow (#1394) probes /auth/oauth/providers on mount. Default to an
  // empty list so the row hides itself; per-test handlers can override.
  server.use(msw.get(api("/auth/oauth/providers"), () => HttpResponse.json({ providers: [] })))
})

describe("<RegisterPage />", () => {
  it("requires terms acceptance before submitting", async () => {
    const user = userEvent.setup()
    renderRegister()
    await user.type(screen.getByTestId("name"), "Alex")
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("register-button"))
    expect(await screen.findByTestId("terms-error")).toBeInTheDocument()
  })

  it("renders the success state with the server message", async () => {
    server.use(
      msw.post(api("/register"), async ({ request }) => {
        const body = (await request.json()) as { email: string }
        expect(body.email).toBe("alex@example.com")
        return HttpResponse.json({
          message: "We've sent you a verification link.",
        })
      })
    )
    const user = userEvent.setup()
    renderRegister()
    await user.type(screen.getByTestId("name"), "Alex")
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("terms"))
    await user.click(screen.getByTestId("register-button"))
    expect(await screen.findByTestId("register-success")).toHaveTextContent(/verification link/i)
  })

  it("surfaces the server error when the API returns 400", async () => {
    server.use(
      msw.post(api("/register"), () =>
        HttpResponse.json(
          { errors: [{ detail: "Registration is currently closed" }] },
          { status: 400 }
        )
      )
    )
    const user = userEvent.setup()
    renderRegister()
    await user.type(screen.getByTestId("name"), "Alex")
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("terms"))
    await user.click(screen.getByTestId("register-button"))
    await waitFor(() =>
      expect(screen.getByTestId("server-error")).toHaveTextContent(
        /registration is currently closed/i
      )
    )
  })

  it("shows the first-item drawer + pill when a draft is pending, and registration still completes (#1988)", async () => {
    savePendingFirstItem({ draftKey: "commodity-draft:anon:create", currency: "USD", savedAt: 1 })
    server.use(
      msw.post(api("/register"), () =>
        HttpResponse.json({ message: "We've sent you a verification link." })
      )
    )
    // pointerEventsCheck:0: the drawer is modal (vaul sets pointer-events:none
    // on the page behind it), and under jsdom it never animates away — we're
    // asserting the affordances render and registration still completes, not
    // the modal's exit animation.
    const user = userEvent.setup({ pointerEventsCheck: 0 })
    renderRegister()
    // Both reassurance affordances appear for a pending anonymous draft.
    expect(await screen.findByTestId("pending-first-item-drawer")).toBeInTheDocument()
    expect(screen.getByTestId("resume-first-item-pill")).toBeInTheDocument()
    // Dismiss the drawer ("Got it"), then registration completes underneath —
    // the modal doesn't permanently gate sign-up.
    await user.click(screen.getByTestId("pending-first-item-drawer-ok"))
    await user.type(screen.getByTestId("name"), "Alex")
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("terms"))
    await user.click(screen.getByTestId("register-button"))
    // Success view replaces the form (and with it the drawer/pill).
    expect(await screen.findByTestId("register-success")).toBeInTheDocument()
    expect(screen.queryByTestId("pending-first-item-drawer")).not.toBeInTheDocument()
  })

  it("omits the first-item drawer and pill when no draft is pending", async () => {
    renderRegister()
    await screen.findByTestId("register-page")
    expect(screen.queryByTestId("pending-first-item-drawer")).not.toBeInTheDocument()
    expect(screen.queryByTestId("resume-first-item-pill")).not.toBeInTheDocument()
  })
})
