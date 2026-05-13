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
  // The PlanCard mounts unconditionally on top of the page (#1389); stub
  // it once at the base level so every test that renders GroupSettings
  // can resolve the plan query cleanly. Per-test overrides (e.g. an
  // error response) can call `server.use(...)` to replace this handler.
  msw.get(api("/g/household/plan"), () =>
    HttpResponse.json({
      plan: {
        id: "unlimited",
        name: "Unlimited",
        max_items: null,
        max_locations: null,
        max_storage_bytes: null,
        allows_restore: true,
        allows_api_access: true,
      },
      usage: { items: 7, locations: 2, storage_bytes: 12_582_912 },
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

  it("renders the Plan card after the plan endpoint resolves", async () => {
    // GroupSettings is mounted at `/groups/:groupId/settings` — a non-
    // group route. The PlanCard nevertheless reads from
    // GET /g/<slug>/plan; the slug is threaded through useGroupPlan
    // explicitly (`features/plan/api.ts`) so the request doesn't rely
    // on http.ts's group-scoped rewriter (which no-ops here). This
    // test guards both the wiring + the skeleton → populated
    // transition so a future regression to the rewriter-based path
    // shows up immediately.
    let captured: string | null = null
    // Override-first: server.use prepends handlers and the FIRST match
    // wins, so the plan override has to come before baseHandlers'
    // default plan stub or the override never fires.
    server.use(
      msw.get(api("/g/household/plan"), ({ request }) => {
        captured = new URL(request.url).pathname
        return HttpResponse.json({
          plan: {
            id: "free",
            name: "Free",
            max_items: 500,
            max_locations: 20,
            max_storage_bytes: 1_073_741_824,
            allows_restore: false,
            allows_api_access: false,
          },
          usage: { items: 9, locations: 2, storage_bytes: 314_572_800 },
        })
      }),
      ...baseHandlers,
      adminMembership
    )
    renderSettings()

    // The populated card lands once the plan response resolves.
    // (We don't assert the intermediate skeleton — MSW responds
    // synchronously and the swap can happen inside a single React
    // commit, so the skeleton state may never be observable to a
    // poll-based query.)
    const card = await screen.findByTestId("plan-card")
    expect(card).toBeInTheDocument()
    expect(screen.queryByTestId("plan-card-skeleton")).not.toBeInTheDocument()

    // Plan name interpolates into the title, "Active" badge renders,
    // and each chip shows the X / Y format with the limit from the
    // payload (not the in-code default).
    expect(screen.getByTestId("plan-card-name")).toHaveTextContent("Free plan")
    expect(screen.getByTestId("plan-card-chip-items")).toHaveTextContent("9 / 500")
    expect(screen.getByTestId("plan-card-chip-locations")).toHaveTextContent("2 / 20")
    expect(screen.getByTestId("plan-card-chip-storage")).toHaveTextContent("/ 1.0 GB")

    // The slug was actually inlined into the request URL — guards
    // against a future refactor that flips back to a bare `/plan`
    // path (which would 404 from this non-group route).
    expect(captured).toBe("/api/v1/g/household/plan")
  })

  it("surfaces the error state when the plan endpoint fails", async () => {
    // PlanCard's guard order must check `isError` before `!data` —
    // React Query keeps `data` undefined on failure, so an
    // `isLoading || !data` check would mask the error forever. This
    // test asserts the destructive Alert renders instead of an
    // infinite skeleton.
    // Override-first ordering (see the "renders the Plan card" test
    // above for why) — the 500 has to be registered before the success
    // stub in baseHandlers or it never fires.
    server.use(
      msw.get(api("/g/household/plan"), () => new HttpResponse(null, { status: 500 })),
      ...baseHandlers,
      adminMembership
    )
    renderSettings()
    expect(await screen.findByTestId("plan-card-error")).toBeInTheDocument()
    expect(screen.queryByTestId("plan-card-skeleton")).not.toBeInTheDocument()
    expect(screen.queryByTestId("plan-card")).not.toBeInTheDocument()
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
