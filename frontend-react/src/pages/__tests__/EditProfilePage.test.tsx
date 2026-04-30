import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { EditProfilePage } from "@/pages/EditProfilePage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
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

function renderEdit() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/profile/edit",
    routes: (
      <>
        <Route
          path="/profile/edit"
          element={
            <AuthProvider>
              <GroupProvider>
                <ConfirmProvider>
                  <EditProfilePage />
                </ConfirmProvider>
              </GroupProvider>
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

const baseUserHandlers = [
  msw.get(api("/auth/me"), () =>
    HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
  ),
  msw.get(api("/groups"), () => HttpResponse.json({ data: [] })),
]

describe("<EditProfilePage />", () => {
  it("validates name is required and ≤255 chars", async () => {
    server.use(...baseUserHandlers)
    const user = userEvent.setup()
    renderEdit()
    // Wait for the form's reset effect to run after the auth probe
    // resolves — otherwise user.clear() races the reset and the field
    // is repopulated with the user's name after we cleared it.
    const nameInput = (await screen.findByTestId("profile-name-input")) as HTMLInputElement
    await waitFor(() => expect(nameInput).toHaveValue("Alex"))
    await user.clear(nameInput)
    await user.click(screen.getByTestId("profile-save"))
    expect(await screen.findByTestId("profile-name-error")).toBeInTheDocument()
  })

  it("submits name + default_group_id and navigates back to /profile", async () => {
    let captured: { name: string; default_group_id: string | null } | null = null
    server.use(
      ...baseUserHandlers,
      msw.put(api("/auth/me"), async ({ request }) => {
        captured = (await request.json()) as typeof captured
        return HttpResponse.json({
          id: "u1",
          email: "alex@example.com",
          name: captured?.name,
          default_group_id: captured?.default_group_id ?? undefined,
        })
      })
    )
    const user = userEvent.setup()
    renderEdit()
    const nameInput = (await screen.findByTestId("profile-name-input")) as HTMLInputElement
    await waitFor(() => expect(nameInput).toHaveValue("Alex"))
    await user.clear(nameInput)
    await user.type(nameInput, "Alex 2")
    await user.click(screen.getByTestId("profile-save"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/profile")
    )
    expect(captured).toEqual({ name: "Alex 2", default_group_id: null })
  })

  it("password form rejects mismatched confirmation", async () => {
    server.use(...baseUserHandlers)
    const user = userEvent.setup()
    renderEdit()
    await user.type(await screen.findByTestId("current-password"), "old-pw-1")
    await user.type(screen.getByTestId("new-password"), "newer-pw-1")
    await user.type(screen.getByTestId("confirm-password"), "different1")
    await user.click(screen.getByTestId("change-password-submit"))
    expect(await screen.findByTestId("confirm-password-error")).toHaveTextContent(/match/i)
  })

  it("password form rejects when new === current", async () => {
    server.use(...baseUserHandlers)
    const user = userEvent.setup()
    renderEdit()
    await user.type(await screen.findByTestId("current-password"), "samepass1")
    await user.type(screen.getByTestId("new-password"), "samepass1")
    await user.type(screen.getByTestId("confirm-password"), "samepass1")
    await user.click(screen.getByTestId("change-password-submit"))
    expect(await screen.findByTestId("new-password-error")).toHaveTextContent(/differ/i)
  })

  it("posts a successful password change and triggers logout flow", async () => {
    // Tried Vitest fake timers here per Copilot review feedback to skip
    // the page's 1500ms post-success delay; both `vi.useFakeTimers()` up
    // front and a "switch after form interactions" variant deadlocked
    // because userEvent + RTL's waitFor + msw rely on real microtask
    // scheduling. Real timers + a 3s waitFor budget is the working
    // compromise: 1.5s page delay + ~200ms test overhead, well inside
    // the per-test 5s budget.
    let logoutCalls = 0
    server.use(
      ...baseUserHandlers,
      msw.post(api("/auth/change-password"), () =>
        HttpResponse.json({ message: "Password changed" })
      ),
      msw.post(api("/auth/logout"), () => {
        logoutCalls++
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderEdit()
    await user.type(await screen.findByTestId("current-password"), "old-pw-1")
    await user.type(screen.getByTestId("new-password"), "new-secure-pw-1")
    await user.type(screen.getByTestId("confirm-password"), "new-secure-pw-1")
    await user.click(screen.getByTestId("change-password-submit"))
    await waitFor(() => expect(screen.getByTestId("password-change-success")).toBeInTheDocument())
    await waitFor(() => expect(logoutCalls).toBe(1), { timeout: 3000 })
  })

  it("surfaces 'incorrect current password' on 422", async () => {
    server.use(
      ...baseUserHandlers,
      msw.post(api("/auth/change-password"), () =>
        HttpResponse.json({ error: "wrong" }, { status: 422 })
      )
    )
    const user = userEvent.setup()
    renderEdit()
    await user.type(await screen.findByTestId("current-password"), "old-pw-1")
    await user.type(screen.getByTestId("new-password"), "new-secure-pw-1")
    await user.type(screen.getByTestId("confirm-password"), "new-secure-pw-1")
    await user.click(screen.getByTestId("change-password-submit"))
    expect(await screen.findByTestId("password-server-error")).toHaveTextContent(
      /current password is incorrect/i
    )
  })
})
