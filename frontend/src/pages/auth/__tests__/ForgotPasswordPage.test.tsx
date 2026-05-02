import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { ForgotPasswordPage } from "@/pages/auth/ForgotPasswordPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function renderForgot() {
  return renderWithProviders({
    initialPath: "/forgot-password",
    routes: (
      <Route
        path="/forgot-password"
        element={
          <AuthProvider>
            <ForgotPasswordPage />
          </AuthProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<ForgotPasswordPage />", () => {
  it("validates the email field before submission", async () => {
    const user = userEvent.setup()
    renderForgot()
    await user.click(screen.getByTestId("submit-button"))
    expect(await screen.findByTestId("email-error")).toBeInTheDocument()
  })

  it("renders the success state regardless of whether the email exists", async () => {
    server.use(
      msw.post(api("/forgot-password"), () =>
        HttpResponse.json({
          message: "If that email exists, you'll get a reset link.",
        })
      )
    )
    const user = userEvent.setup()
    renderForgot()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.click(screen.getByTestId("submit-button"))
    expect(await screen.findByTestId("forgot-success")).toHaveTextContent(
      /you'll get a reset link/i
    )
  })

  it("surfaces server errors inline on 500", async () => {
    server.use(
      msw.post(api("/forgot-password"), () =>
        HttpResponse.json({ error: "Mailer down" }, { status: 500 })
      )
    )
    const user = userEvent.setup()
    renderForgot()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.click(screen.getByTestId("submit-button"))
    expect(await screen.findByTestId("server-error")).toHaveTextContent(/mailer down/i)
  })

  it("surfaces resend errors inline on the success state", async () => {
    // First call succeeds; second call (resend) fails. The success branch
    // must render an inline alert instead of swallowing the error.
    let calls = 0
    server.use(
      msw.post(api("/forgot-password"), () => {
        calls++
        if (calls === 1) {
          return HttpResponse.json({ message: "Reset link sent." })
        }
        return HttpResponse.json({ error: "Rate limited" }, { status: 429 })
      })
    )
    const user = userEvent.setup()
    renderForgot()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.click(screen.getByTestId("submit-button"))
    const success = await screen.findByTestId("forgot-success")
    await user.click(within(success).getByRole("button", { name: /resend/i }))
    expect(await screen.findByTestId("resend-error")).toHaveTextContent(/rate limited/i)
  })
})
