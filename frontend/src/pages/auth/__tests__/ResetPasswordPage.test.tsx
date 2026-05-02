import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { ResetPasswordPage } from "@/pages/auth/ResetPasswordPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function renderReset(path: string) {
  return renderWithProviders({
    initialPath: path,
    routes: (
      <Route
        path="/reset-password"
        element={
          <AuthProvider>
            <ResetPasswordPage />
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

describe("<ResetPasswordPage />", () => {
  it("shows a missing-token notice when ?token is absent", () => {
    renderReset("/reset-password")
    expect(screen.getByTestId("reset-missing-token")).toBeInTheDocument()
  })

  it("validates min length and matching confirmation", async () => {
    const user = userEvent.setup()
    renderReset("/reset-password?token=tok-1")
    await user.type(screen.getByTestId("password"), "short")
    await user.type(screen.getByTestId("confirm-password"), "different")
    await user.click(screen.getByTestId("submit-button"))
    expect(await screen.findByTestId("password-error")).toBeInTheDocument()
  })

  it("surfaces password mismatch error on the confirm field", async () => {
    const user = userEvent.setup()
    renderReset("/reset-password?token=tok-1")
    await user.type(screen.getByTestId("password"), "longenough")
    await user.type(screen.getByTestId("confirm-password"), "different1")
    await user.click(screen.getByTestId("submit-button"))
    expect(await screen.findByTestId("confirm-password-error")).toHaveTextContent(/match/i)
  })

  it("posts the new password and renders the success state", async () => {
    server.use(
      msw.post(api("/reset-password"), async ({ request }) => {
        const body = (await request.json()) as { token: string; new_password: string }
        expect(body.token).toBe("tok-1")
        expect(body.new_password).toBe("longenough123")
        return HttpResponse.json({ message: "Password updated." })
      })
    )
    const user = userEvent.setup()
    renderReset("/reset-password?token=tok-1")
    await user.type(screen.getByTestId("password"), "longenough123")
    await user.type(screen.getByTestId("confirm-password"), "longenough123")
    await user.click(screen.getByTestId("submit-button"))
    await waitFor(() =>
      expect(screen.getByTestId("reset-success")).toHaveTextContent(/password updated/i)
    )
  })
})
