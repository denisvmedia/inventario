import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { InviteAcceptPage } from "@/pages/auth/InviteAcceptPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearPendingInvite, peekPendingInvite } from "@/features/auth/inviteHandoff"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} />
}

function renderInvite(initial: string, opts?: { authenticated?: boolean }) {
  if (opts?.authenticated) setAccessToken("good-token")
  return renderWithProviders({
    initialPath: initial,
    routes: (
      <>
        <Route
          path="/invite/:token"
          element={
            <AuthProvider>
              <InviteAcceptPage />
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

describe("<InviteAcceptPage />", () => {
  it("renders the invalid state when the invite lookup fails", async () => {
    server.use(
      msw.get(api("/invites/bad-tok"), () =>
        HttpResponse.json({ error: "not found" }, { status: 404 })
      )
    )
    renderInvite("/invite/bad-tok")
    await waitFor(() => expect(screen.getByTestId("invite-invalid")).toBeInTheDocument())
  })

  it("renders the expired state when the invite is expired", async () => {
    server.use(
      msw.get(api("/invites/exp-tok"), () =>
        HttpResponse.json({
          data: {
            type: "invite_info",
            attributes: { group_name: "Household", expired: true, used: false },
          },
        })
      )
    )
    renderInvite("/invite/exp-tok")
    await waitFor(() => expect(screen.getByTestId("invite-expired")).toBeInTheDocument())
  })

  it("for an unauthenticated user, stores the invite in sessionStorage and renders sign-in CTAs", async () => {
    server.use(
      msw.get(api("/invites/inv-tok"), () =>
        HttpResponse.json({
          data: {
            type: "invite_info",
            attributes: { group_name: "Household", expired: false, used: false },
          },
        })
      )
    )
    renderInvite("/invite/inv-tok")
    await waitFor(() => expect(screen.getByTestId("invite-page")).toBeInTheDocument())
    expect(screen.getByTestId("invite-login-link")).toBeInTheDocument()
    expect(screen.getByTestId("invite-register-link")).toBeInTheDocument()
    await waitFor(() =>
      expect(peekPendingInvite()).toEqual({ token: "inv-tok", groupName: "Household" })
    )
  })

  it("for an authenticated user, accepts the invite and redirects home", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
      ),
      msw.get(api("/invites/inv-tok"), () =>
        HttpResponse.json({
          data: {
            type: "invite_info",
            attributes: { group_name: "Household", expired: false, used: false },
          },
        })
      ),
      msw.post(api("/invites/inv-tok/accept"), () =>
        HttpResponse.json({ data: { id: "m1", attributes: { group_id: "g1" } } })
      )
    )
    const user = userEvent.setup()
    renderInvite("/invite/inv-tok", { authenticated: true })
    const acceptBtn = await screen.findByTestId("invite-accept-btn")
    await user.click(acceptBtn)
    await waitFor(() => expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/"))
  })
})
