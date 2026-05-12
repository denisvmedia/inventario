import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { GroupSettingsPage } from "@/pages/groups/GroupSettingsPage"
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

function renderSettings(initial = "/groups/g1/settings") {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: initial,
    routes: (
      <>
        <Route
          path="/groups/:groupId/settings"
          element={
            <AuthProvider>
              <GroupProvider>
                <ConfirmProvider>
                  <GroupSettingsPage />
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

const baseHandlers = [
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
            group_currency: "USD",
            icon: "🏠",
            created_at: "2026-04-01T00:00:00Z",
          },
        },
      ],
    })
  ),
  msw.get(api("/groups/g1"), () =>
    HttpResponse.json({
      data: {
        id: "g1",
        type: "groups",
        attributes: {
          id: "g1",
          slug: "household",
          name: "Household",
          group_currency: "USD",
          icon: "🏠",
          created_at: "2026-04-01T00:00:00Z",
        },
      },
    })
  ),
]

const adminMembership = msw.get(api("/groups/g1/members"), () =>
  HttpResponse.json({
    data: [
      {
        id: "m1",
        type: "memberships",
        attributes: {
          id: "m1",
          group_id: "g1",
          member_user_id: "u1",
          role: "admin",
        },
      },
      {
        id: "m2",
        type: "memberships",
        attributes: {
          id: "m2",
          group_id: "g1",
          member_user_id: "u2",
          role: "admin",
        },
      },
    ],
  })
)

describe("<GroupSettingsPage />", () => {
  it("redirects to /no-group when there's no group id in the URL", () => {
    setAccessToken("good-token")
    renderWithProviders({
      initialPath: "/groups/settings",
      routes: (
        <>
          <Route
            path="/groups/settings"
            element={
              <AuthProvider>
                <GroupProvider>
                  <ConfirmProvider>
                    <GroupSettingsPage />
                  </ConfirmProvider>
                </GroupProvider>
              </AuthProvider>
            }
          />
          <Route path="*" element={<LocationProbe />} />
        </>
      ),
    })
    // useParams<{ groupId }>() returns undefined → <Navigate to="/no-group">.
    expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
  })

  it("renders the form with name + icon prefilled and a read-only currency for an admin", async () => {
    server.use(...baseHandlers, adminMembership)
    const user = userEvent.setup()
    renderSettings()
    // Info section is the default — identity form should be present right away.
    await waitFor(() => expect(screen.getByTestId("settings-name-input")).toHaveValue("Household"))
    // Members shortcut lives behind the Members nav.
    await user.click(screen.getByTestId("group-settings-nav-members"))
    expect(await screen.findByTestId("settings-members-link")).toBeInTheDocument()
    // Danger zone is admin-only, lives behind the Management nav.
    await user.click(screen.getByTestId("group-settings-nav-management"))
    expect(await screen.findByTestId("delete-group-open")).toBeInTheDocument()
  })

  it("hides admin-only sections when the viewer is not an admin", async () => {
    server.use(
      ...baseHandlers,
      msw.get(api("/groups/g1/members"), () =>
        HttpResponse.json({
          data: [
            {
              id: "m1",
              type: "memberships",
              attributes: {
                id: "m1",
                group_id: "g1",
                member_user_id: "u1",
                role: "user",
              },
            },
          ],
        })
      )
    )
    const user = userEvent.setup()
    renderSettings()
    await waitFor(() => expect(screen.getByTestId("group-settings-page")).toBeInTheDocument())
    // Non-admin Info: no name input — read-only "admin only" panel instead.
    expect(screen.queryByTestId("settings-name-input")).not.toBeInTheDocument()
    // Non-admin Management: delete CTA is gated.
    await user.click(screen.getByTestId("group-settings-nav-management"))
    expect(screen.queryByTestId("delete-group-open")).not.toBeInTheDocument()
  })

  it("disables Leave for the last owner", async () => {
    // Post-#1533 the ≥1 invariant moves from admin to owner — the leave
    // guard fires when this user is the only owner left.
    server.use(
      ...baseHandlers,
      msw.get(api("/groups/g1/members"), () =>
        HttpResponse.json({
          data: [
            {
              id: "m1",
              type: "memberships",
              attributes: {
                id: "m1",
                group_id: "g1",
                member_user_id: "u1",
                role: "owner",
              },
            },
          ],
        })
      )
    )
    const user = userEvent.setup()
    renderSettings()
    await waitFor(() => expect(screen.getByTestId("group-settings-page")).toBeInTheDocument())
    await user.click(screen.getByTestId("group-settings-nav-members"))
    const leave = await screen.findByTestId("leave-group-btn")
    expect(leave).toBeDisabled()
  })

  it("opens the delete dialog and rejects a confirm-word mismatch client-side", async () => {
    server.use(...baseHandlers, adminMembership)
    const user = userEvent.setup()
    renderSettings()
    await waitFor(() => expect(screen.getByTestId("group-settings-page")).toBeInTheDocument())
    await user.click(screen.getByTestId("group-settings-nav-management"))
    await user.click(await screen.findByTestId("delete-group-open"))
    const dialog = await screen.findByTestId("delete-group-dialog")
    expect(dialog).toBeInTheDocument()
    await user.type(screen.getByTestId("delete-confirm-word"), "Wrong Name")
    await user.type(screen.getByTestId("delete-password"), "secret-pw")
    await user.click(screen.getByTestId("delete-group-submit"))
    expect(await screen.findByTestId("delete-confirm-word-error")).toBeInTheDocument()
  })

  it("submits the delete with confirm-word + password and navigates to /no-group", async () => {
    let captured: { confirm_word?: string; password?: string } | null = null
    server.use(
      ...baseHandlers,
      adminMembership,
      msw.delete(api("/groups/g1"), async ({ request }) => {
        captured = (await request.json()) as typeof captured
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderSettings()
    await waitFor(() => expect(screen.getByTestId("group-settings-page")).toBeInTheDocument())
    await user.click(screen.getByTestId("group-settings-nav-management"))
    await user.click(await screen.findByTestId("delete-group-open"))
    await user.type(await screen.findByTestId("delete-confirm-word"), "Household")
    await user.type(screen.getByTestId("delete-password"), "secret-pw")
    await user.click(screen.getByTestId("delete-group-submit"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
    )
    expect(captured).toEqual({ confirm_word: "Household", password: "secret-pw" })
  })

  it("saves name + icon edits via PATCH /groups/:id", async () => {
    let captured: { data?: { attributes?: Record<string, unknown> } } | null = null
    server.use(
      ...baseHandlers,
      adminMembership,
      msw.patch(api("/groups/g1"), async ({ request }) => {
        captured = (await request.json()) as typeof captured
        return HttpResponse.json({
          data: {
            id: "g1",
            type: "groups",
            attributes: {
              id: "g1",
              slug: "household",
              name: (captured?.data?.attributes?.name as string) ?? "Household",
              icon: (captured?.data?.attributes?.icon as string) ?? "🏠",
              group_currency: "USD",
            },
          },
        })
      })
    )
    const user = userEvent.setup()
    renderSettings()
    const name = (await screen.findByTestId("settings-name-input")) as HTMLInputElement
    await waitFor(() => expect(name).toHaveValue("Household"))
    await user.clear(name)
    await user.type(name, "Renamed")
    await user.click(screen.getByTestId("settings-save"))
    await waitFor(() =>
      expect(captured?.data?.attributes).toMatchObject({ name: "Renamed", icon: "🏠" })
    )
  })
})
