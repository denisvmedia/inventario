import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"

import { LoginHistoryPage } from "@/pages/LoginHistoryPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
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
    initialPath: "/profile/login-history",
    routes: (
      <Route
        path="/profile/login-history"
        element={
          <AuthProvider>
            <GroupProvider>
              <LoginHistoryPage />
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

describe("<LoginHistoryPage />", () => {
  it("renders one row per event with the correct outcome", async () => {
    server.use(
      meHandler,
      groupsHandler,
      msw.get(api("/users/me/login-history"), () =>
        HttpResponse.json({
          events: [
            {
              id: "ev-ok",
              created_at: "2026-05-14T07:55:00Z",
              email: "alex@example.com",
              outcome: "ok",
              method: "password",
              ip_address: "203.0.113.0/24",
              user_agent: "Mozilla/5.0 (Macintosh) Chrome/120.0",
            },
            {
              id: "ev-bad",
              created_at: "2026-05-14T07:50:00Z",
              email: "alex@example.com",
              outcome: "bad_password",
              method: "password",
              ip_address: "198.51.100.0/24",
              user_agent: "Mozilla/5.0 (iPhone) Safari/605",
            },
          ],
          failed_last_7d: 2,
        })
      )
    )
    renderPage()
    const list = await screen.findByTestId("login-history-list")
    const rows = within(list).getAllByTestId("login-history-row")
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveAttribute("data-outcome", "ok")
    expect(rows[1]).toHaveAttribute("data-outcome", "bad_password")
    // failed_last_7d=2 is below the threshold (3) — banner stays hidden.
    expect(screen.queryByTestId("login-history-failed-banner")).not.toBeInTheDocument()
  })

  it("shows the failed-attempts banner when failed_last_7d > 3", async () => {
    server.use(
      meHandler,
      groupsHandler,
      msw.get(api("/users/me/login-history"), () =>
        HttpResponse.json({ events: [], failed_last_7d: 5 })
      )
    )
    renderPage()
    await waitFor(() =>
      expect(screen.getByTestId("login-history-failed-banner")).toBeInTheDocument()
    )
  })
})
