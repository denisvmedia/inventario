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
  __resetGroupContextForTests()
  __resetHttpForTests()
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
})
