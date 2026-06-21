import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { SessionsPage } from "@/pages/SessionsPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

function renderPage() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/profile/sessions",
    routes: (
      <Route
        path="/profile/sessions"
        element={
          <AuthProvider>
            <GroupProvider>
              <ConfirmProvider>
                <SessionsPage />
              </ConfirmProvider>
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

const meHandler = msw.get(api("/auth/me"), () =>
  HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
)
const groupsHandler = msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))

const baseTokens = [
  {
    id: "rt-current",
    created_at: "2026-05-13T08:00:00Z",
    last_used_at: "2026-05-14T07:55:00Z",
    expires_at: "2026-06-13T08:00:00Z",
    ip_address: "203.0.113.0/24",
    user_agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/120.0",
    is_current: true,
  },
  {
    id: "rt-other",
    created_at: "2026-05-10T12:00:00Z",
    last_used_at: "2026-05-12T09:00:00Z",
    expires_at: "2026-06-10T12:00:00Z",
    ip_address: "198.51.100.0/24",
    user_agent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Safari/605.1.15",
    is_current: false,
  },
]

describe("<SessionsPage />", () => {
  it("renders one card per session and tags the current device", async () => {
    server.use(
      meHandler,
      groupsHandler,
      msw.get(api("/users/me/sessions"), () => HttpResponse.json({ sessions: baseTokens }))
    )
    renderPage()
    const list = await screen.findByTestId("sessions-list")
    const cards = within(list).getAllByTestId("session-card")
    expect(cards).toHaveLength(2)
    expect(within(cards[0]).getByTestId("session-current-pill")).toBeInTheDocument()
    // Current session has no revoke button; the other one does.
    expect(within(cards[0]).queryByTestId("session-revoke-btn")).not.toBeInTheDocument()
    expect(within(cards[1]).getByTestId("session-revoke-btn")).toBeInTheDocument()
  })

  it("revokes a single session after confirmation and refreshes the list", async () => {
    let listCalls = 0
    let revokeId = ""
    server.use(
      meHandler,
      groupsHandler,
      msw.get(api("/users/me/sessions"), () => {
        listCalls += 1
        // First call returns both sessions; subsequent calls reflect
        // the revocation by dropping the second token.
        return HttpResponse.json({
          sessions: listCalls === 1 ? baseTokens : [baseTokens[0]],
        })
      }),
      msw.delete(api("/users/me/sessions/:id"), ({ params }) => {
        revokeId = String(params.id)
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderPage()
    const list = await screen.findByTestId("sessions-list")
    const cards = within(list).getAllByTestId("session-card")
    await user.click(within(cards[1]).getByTestId("session-revoke-btn"))
    await user.click(await screen.findByTestId("sessions-confirm-revoke-btn"))
    await waitFor(() => expect(revokeId).toBe("rt-other"))
    await waitFor(() => {
      const stillThere = within(screen.getByTestId("sessions-list")).queryAllByTestId(
        "session-card"
      )
      expect(stillThere).toHaveLength(1)
    })
  })

  it("revoke-all-other-sessions button fires DELETE /users/me/sessions with the current keep_id", async () => {
    // The refresh cookie is path-scoped to /api/v1/auth so the BE
    // can't recover the current session from it on this route; the FE
    // must pass the id of the row flagged `is_current: true` via
    // `?keep_id=` or the BE wipes every session (the bug that broke
    // the sessions-and-login-history e2e cleanup branch).
    let capturedQuery = ""
    server.use(
      meHandler,
      groupsHandler,
      msw.get(api("/users/me/sessions"), () => HttpResponse.json({ sessions: baseTokens })),
      msw.delete(api("/users/me/sessions"), ({ request }) => {
        capturedQuery = new URL(request.url).search
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("sessions-list")
    await user.click(screen.getByTestId("sessions-revoke-all-btn"))
    await user.click(await screen.findByTestId("sessions-confirm-revoke-all-btn"))
    await waitFor(() => expect(capturedQuery).toBe("?keep_id=rt-current"))
  })

  it("hides the revoke-all CTA when no session is flagged current (impersonation)", async () => {
    // During impersonation the imp session has no rti claim and no tenant
    // refresh cookie, so the BE flags no row `is_current`. The CTA would
    // resolve a keep-id of undefined and wipe EVERY session, so it must be
    // hidden even though "other" sessions exist (#2126).
    const noCurrent = baseTokens.map((t) => ({ ...t, is_current: false }))
    server.use(
      meHandler,
      groupsHandler,
      msw.get(api("/users/me/sessions"), () => HttpResponse.json({ sessions: noCurrent }))
    )
    renderPage()
    await screen.findByTestId("sessions-list")
    expect(screen.queryByTestId("sessions-revoke-all-btn")).not.toBeInTheDocument()
  })
})
