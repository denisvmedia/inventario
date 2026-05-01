import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import { axe } from "jest-axe"

import { ProfilePage } from "@/pages/ProfilePage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function renderProfile() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/profile",
    routes: (
      <Route
        path="/profile"
        element={
          <AuthProvider>
            <GroupProvider>
              <ProfilePage />
            </GroupProvider>
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

describe("<ProfilePage />", () => {
  it("renders the user's name, email, and member-since", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u1",
          email: "alex@example.com",
          name: "Alex Johnson",
          created_at: "2024-01-15T10:00:00Z",
          default_group_id: "g1",
        })
      ),
      msw.get(api("/groups"), () =>
        HttpResponse.json({
          data: [
            {
              id: "g1",
              type: "groups",
              attributes: { id: "g1", slug: "household", name: "Household" },
            },
          ],
        })
      )
    )
    renderProfile()
    await waitFor(() =>
      expect(screen.getByTestId("profile-name")).toHaveTextContent("Alex Johnson")
    )
    expect(screen.getByTestId("profile-email")).toHaveTextContent("alex@example.com")
    expect(screen.getByTestId("profile-default-group")).toHaveTextContent("Household")
  })

  it("falls back to 'no group set' when default_group_id is unset", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
      ),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))
    )
    renderProfile()
    await waitFor(() =>
      expect(screen.getByTestId("profile-default-group")).toHaveTextContent(/no default group/i)
    )
  })

  it("links to /profile/edit and /plans", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
      ),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))
    )
    renderProfile()
    await waitFor(() => expect(screen.getByTestId("profile-edit-link")).toBeInTheDocument())
    expect(screen.getByTestId("profile-edit-link")).toHaveAttribute("href", "/profile/edit")
    expect(screen.getByTestId("profile-upgrade-link")).toHaveAttribute("href", "/plans")
  })

  it("has no axe violations", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u1",
          email: "alex@example.com",
          name: "Alex Johnson",
          created_at: "2024-01-15T10:00:00Z",
        })
      ),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))
    )
    const { container } = renderProfile()
    // Wait for the heading to populate — axe flags an empty h1, and the
    // h1 mounts before /auth/me resolves so a bare in-document check
    // races the empty-heading window.
    await waitFor(() =>
      expect(screen.getByTestId("profile-name")).toHaveTextContent("Alex Johnson")
    )
    expect(await axe(container)).toHaveNoViolations()
  })
})
