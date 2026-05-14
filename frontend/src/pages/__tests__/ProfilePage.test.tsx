import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
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

function renderProfile(initialPath = "/profile") {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
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

// Stub the commodities + values endpoints used by `useDashboardData` so the
// 4-stat snapshot tile renders concrete numbers instead of the still-loading
// dash placeholder. The values payload mirrors the real BE shape
// (`data.attributes.global_total` + per-location/area breakdown lists with
// `{id,name,value}`) so the stub doesn't accidentally mask a contract
// regression — even though the page only reads `global_total` today.
function mockDashboardEndpoints(slug: string) {
  return [
    msw.get(api(`/g/${slug}/commodities`), () =>
      HttpResponse.json({
        data: [],
        meta: { commodities: 0, total: 0, page: 1, per_page: 100, total_pages: 1 },
      })
    ),
    msw.get(api(`/g/${slug}/commodities/values`), () =>
      HttpResponse.json({
        data: {
          attributes: {
            global_total: 0,
            location_totals: [],
            area_totals: [],
          },
        },
      })
    ),
  ]
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

  it("renders the initials avatar from the user's name", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex Johnson" })
      ),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))
    )
    renderProfile()
    // The avatar is rendered as a presentational `aria-hidden="true"` tile
    // (the heading carries the accessible name). Look it up via the inner
    // text content so this test exercises the actual rendering path the
    // user sees, not just the API contract.
    await waitFor(() => {
      const card = screen.getByTestId("profile-page")
      expect(within(card).getByText("AJ")).toBeInTheDocument()
    })
  })

  it("renders the 4-up stat snapshot grid", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
      ),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))
    )
    renderProfile()
    await waitFor(() => expect(screen.getByTestId("profile-stats")).toBeInTheDocument())
    expect(screen.getByTestId("profile-stat-items")).toBeInTheDocument()
    expect(screen.getByTestId("profile-stat-active-warranties")).toBeInTheDocument()
    expect(screen.getByTestId("profile-stat-expiring-warranties")).toBeInTheDocument()
    expect(screen.getByTestId("profile-stat-est-value")).toBeInTheDocument()
  })

  describe("Groups tab", () => {
    it("renders the empty state when the user has no groups", async () => {
      server.use(
        msw.get(api("/auth/me"), () =>
          HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
        ),
        msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))
      )
      renderProfile()
      await waitFor(() => expect(screen.getByTestId("profile-groups-empty")).toBeInTheDocument())
      expect(screen.queryByTestId("profile-groups-list")).not.toBeInTheDocument()
    })

    it("renders the error state when /groups fails on first load", async () => {
      server.use(
        msw.get(api("/auth/me"), () =>
          HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
        ),
        msw.get(api("/groups"), () => new HttpResponse(null, { status: 500 }))
      )
      renderProfile()
      // The error tile must surface — without an explicit error branch the
      // tab would pin the loading skeleton forever, because React Query
      // keeps `data` undefined when the first fetch errors.
      await waitFor(() => expect(screen.getByTestId("profile-groups-error")).toBeInTheDocument())
      expect(screen.queryByTestId("profile-groups-loading")).not.toBeInTheDocument()
      expect(screen.queryByTestId("profile-groups-empty")).not.toBeInTheDocument()
    })

    it("renders a single group tile with the user's role", async () => {
      server.use(
        msw.get(api("/auth/me"), () =>
          HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
        ),
        msw.get(api("/groups"), () =>
          HttpResponse.json({
            data: [
              {
                id: "g1",
                type: "groups",
                attributes: {
                  id: "g1",
                  slug: "household",
                  name: "Household",
                  members_count: 3,
                  current_user_role: "owner",
                },
              },
            ],
          })
        )
      )
      renderProfile()
      await waitFor(() => {
        const tiles = screen.getAllByTestId("profile-group-tile")
        expect(tiles).toHaveLength(1)
      })
      const tile = screen.getByTestId("profile-group-tile")
      expect(within(tile).getByTestId("profile-group-tile-name")).toHaveTextContent("Household")
      expect(within(tile).getByTestId("profile-group-tile-role")).toHaveTextContent(/Owner/i)
      expect(tile).toHaveAttribute("href", "/g/household")
      expect(tile).toHaveAttribute("data-group-slug", "household")
    })

    it("renders multiple group tiles with their own role badges", async () => {
      server.use(
        msw.get(api("/auth/me"), () =>
          HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
        ),
        msw.get(api("/groups"), () =>
          HttpResponse.json({
            data: [
              {
                id: "g1",
                type: "groups",
                attributes: {
                  id: "g1",
                  slug: "household",
                  name: "Household",
                  members_count: 3,
                  current_user_role: "owner",
                },
              },
              {
                id: "g2",
                type: "groups",
                attributes: {
                  id: "g2",
                  slug: "family",
                  name: "Family",
                  members_count: 5,
                  current_user_role: "user",
                },
              },
              {
                id: "g3",
                type: "groups",
                attributes: {
                  id: "g3",
                  slug: "office",
                  name: "Office",
                  members_count: 12,
                  current_user_role: "admin",
                },
              },
            ],
          })
        )
      )
      renderProfile()
      await waitFor(() => {
        const tiles = screen.getAllByTestId("profile-group-tile")
        expect(tiles).toHaveLength(3)
      })
      const tiles = screen.getAllByTestId("profile-group-tile")
      const roles = tiles.map((tile) =>
        tile.querySelector('[data-testid="profile-group-tile-role"]')?.getAttribute("data-role")
      )
      expect(roles).toEqual(["owner", "user", "admin"])
      // Each tile is a Link → /g/{slug} so cmd/ctrl-click still works.
      expect(tiles[0]).toHaveAttribute("href", "/g/household")
      expect(tiles[1]).toHaveAttribute("href", "/g/family")
      expect(tiles[2]).toHaveAttribute("href", "/g/office")
    })

    it("renders a +N overflow row when groups exceed the tile cap", async () => {
      const groups = Array.from({ length: 9 }, (_, i) => ({
        id: `g${i}`,
        type: "groups",
        attributes: {
          id: `g${i}`,
          slug: `group-${i}`,
          name: `Group ${i}`,
          members_count: 1,
          current_user_role: "user",
        },
      }))
      server.use(
        msw.get(api("/auth/me"), () =>
          HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
        ),
        msw.get(api("/groups"), () => HttpResponse.json({ data: groups }))
      )
      renderProfile()
      await waitFor(() => {
        expect(screen.getAllByTestId("profile-group-tile")).toHaveLength(6)
      })
      // 9 total - 6 visible = 3 overflow.
      expect(screen.getByTestId("profile-groups-overflow")).toHaveTextContent(/\+3/)
    })
  })

  describe("Activity tab", () => {
    it("renders the empty state and switches in via the tab control", async () => {
      server.use(
        msw.get(api("/auth/me"), () =>
          HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
        ),
        msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))
      )
      renderProfile()
      await waitFor(() => expect(screen.getByTestId("profile-tab-activity")).toBeInTheDocument())
      const user = userEvent.setup()
      await user.click(screen.getByTestId("profile-tab-activity"))
      await waitFor(() => expect(screen.getByTestId("profile-activity-empty")).toBeInTheDocument())
    })
  })

  describe("Stat snapshot", () => {
    it("renders item count and est. value once the active group resolves", async () => {
      server.use(
        msw.get(api("/auth/me"), () =>
          HttpResponse.json({
            id: "u1",
            email: "alex@example.com",
            name: "Alex",
            default_group_id: "g1",
          })
        ),
        msw.get(api("/groups"), () =>
          HttpResponse.json({
            data: [
              {
                id: "g1",
                type: "groups",
                attributes: {
                  id: "g1",
                  slug: "household",
                  name: "Household",
                  group_currency: "USD",
                  members_count: 1,
                  current_user_role: "owner",
                },
              },
            ],
          })
        ),
        ...mockDashboardEndpoints("household")
      )
      renderProfile("/profile?g=household")
      await waitFor(() => {
        expect(screen.getByTestId("profile-stat-items-value")).toHaveTextContent("0")
      })
      expect(screen.getByTestId("profile-stat-est-value-value").textContent).toMatch(/\$/)
    })
  })
})
