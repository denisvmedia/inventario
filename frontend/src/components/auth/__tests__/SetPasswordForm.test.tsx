import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { SetPasswordForm } from "@/components/auth/SetPasswordForm"
import { AuthProvider } from "@/features/auth/AuthContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function renderForm() {
  return renderWithProviders({
    initialPath: "/profile/edit",
    routes: (
      <Route
        path="/profile/edit"
        element={
          <AuthProvider>
            <SetPasswordForm />
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
  setAccessToken("good-token")
  // Seed an authenticated user so AuthProvider can render its children.
  server.use(
    msw.get(api("/auth/me"), () =>
      HttpResponse.json({ id: "u1", email: "denis@example.com", name: "Denis" })
    )
  )
})

describe("<SetPasswordForm />", () => {
  it("renders the Set Password card with the OAuth-only subtitle", async () => {
    renderForm()
    expect(await screen.findByTestId("set-password-form")).toBeInTheDocument()
    expect(screen.getByText(/Set a password/i)).toBeInTheDocument()
  })

  it("submits with an empty current_password so the BE takes the OAuth-only branch", async () => {
    let bodySeen: Record<string, string> | null = null
    server.use(
      msw.post(api("/auth/change-password"), async ({ request }) => {
        bodySeen = (await request.json()) as Record<string, string>
        return HttpResponse.json({ message: "ok" })
      })
    )
    renderForm()
    const user = userEvent.setup()
    await user.type(await screen.findByTestId("set-new-password"), "super-strong-pw")
    await user.type(screen.getByTestId("set-confirm-password"), "super-strong-pw")
    await user.click(screen.getByTestId("set-password-submit"))
    await waitFor(() => expect(bodySeen).not.toBeNull())
    expect(bodySeen).toEqual({ current_password: "", new_password: "super-strong-pw" })
  })

  it("shows the inline confirm-mismatch error before submitting", async () => {
    renderForm()
    const user = userEvent.setup()
    await user.type(await screen.findByTestId("set-new-password"), "super-strong-pw")
    await user.type(screen.getByTestId("set-confirm-password"), "different-pw")
    await user.click(screen.getByTestId("set-password-submit"))
    expect(await screen.findByTestId("set-confirm-password-error")).toBeInTheDocument()
  })

  it("surfaces the BE error inline (e.g. the user actually had a password on file)", async () => {
    server.use(
      msw.post(api("/auth/change-password"), () =>
        HttpResponse.text("Current and new passwords are required", { status: 400 })
      )
    )
    renderForm()
    const user = userEvent.setup()
    await user.type(await screen.findByTestId("set-new-password"), "super-strong-pw")
    await user.type(screen.getByTestId("set-confirm-password"), "super-strong-pw")
    await user.click(screen.getByTestId("set-password-submit"))
    expect(await screen.findByTestId("set-password-server-error")).toBeInTheDocument()
  })
})
