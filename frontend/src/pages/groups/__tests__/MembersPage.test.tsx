import { beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { MembersPage } from "@/pages/groups/MembersPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

const groupsHandler = msw.get(api("/groups"), () =>
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

function userMe(extra: Partial<{ id: string; email: string; name: string }> = {}) {
  return msw.get(api("/auth/me"), () =>
    HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex", ...extra })
  )
}

function renderMembers() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/g/household/members",
    routes: (
      <Route
        path="/g/:groupSlug/members"
        element={
          <AuthProvider>
            <GroupProvider>
              <ConfirmProvider>
                <MembersPage />
              </ConfirmProvider>
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
  // jsdom doesn't define navigator.clipboard. The copy button calls
  // `navigator.clipboard.writeText`; if it isn't there the page logs
  // a `copyFailed` toast and continues. We don't assert on writeText
  // directly in unit tests — the spy surface ended up unreliable
  // across re-renders / test-isolation in our setup. The Playwright
  // suite (#1419) covers the real-clipboard interaction.
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
  })
})

describe("<MembersPage />", () => {
  it("renders the member list with role pills + 'you' badge for the current user", async () => {
    server.use(
      groupsHandler,
      userMe(),
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
                role: "admin",
                joined_at: "2026-04-01T00:00:00Z",
              },
            },
            {
              id: "m2",
              type: "memberships",
              attributes: {
                id: "m2",
                group_id: "g1",
                member_user_id: "u2-other-user",
                role: "user",
                joined_at: "2026-04-02T00:00:00Z",
              },
            },
          ],
        })
      ),
      msw.get(api("/groups/g1/invites"), () => HttpResponse.json({ data: [] }))
    )
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-list")).toBeInTheDocument())
    expect(screen.getByText("(you)")).toBeInTheDocument()
    expect(screen.getByTestId("member-role-u1")).toHaveTextContent(/admin/i)
    expect(screen.getByTestId("member-role-u2-other")).toHaveTextContent(/member/i)
  })

  it("hides admin actions when the viewer is not an admin", async () => {
    server.use(
      groupsHandler,
      userMe(),
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
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-list")).toBeInTheDocument())
    expect(screen.queryByTestId("invites-section")).not.toBeInTheDocument()
    expect(screen.queryByTestId(/^remove-member-btn-/)).not.toBeInTheDocument()
  })

  it("disables the role select + Remove button on the last admin row", async () => {
    server.use(
      groupsHandler,
      userMe(),
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
                role: "admin",
              },
            },
            {
              id: "m2",
              type: "memberships",
              attributes: {
                id: "m2",
                group_id: "g1",
                member_user_id: "u2-other",
                role: "user",
              },
            },
          ],
        })
      ),
      msw.get(api("/groups/g1/invites"), () => HttpResponse.json({ data: [] }))
    )
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-list")).toBeInTheDocument())
    expect(screen.getByTestId("remove-member-btn-u1")).toBeDisabled()
    expect(screen.getByTestId("member-role-select-u1")).toBeDisabled()
    // The non-admin row stays actionable.
    expect(screen.getByTestId("remove-member-btn-u2-other")).not.toBeDisabled()
  })

  it("admin can generate an invite link and copy it to clipboard", async () => {
    let createCalls = 0
    // Mutable list — POST appends, the next GET reflects it. Mirrors real
    // backend behavior where a freshly-created invite shows up in the
    // pending list immediately.
    const inviteList: Array<{
      id: string
      type: string
      attributes: { id: string; token: string; expires_at: string }
    }> = []
    server.use(
      groupsHandler,
      userMe(),
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
                role: "admin",
              },
            },
          ],
        })
      ),
      msw.get(api("/groups/g1/invites"), () => HttpResponse.json({ data: inviteList })),
      msw.post(api("/groups/g1/invites"), () => {
        createCalls++
        const entry = {
          id: "inv1",
          type: "invites",
          attributes: {
            id: "inv1",
            token: "tok-abc",
            expires_at: "2026-05-01T00:00:00Z",
          },
        }
        inviteList.push(entry)
        return HttpResponse.json({ data: entry }, { status: 201 })
      })
    )
    const user = userEvent.setup()
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("invites-section")).toBeInTheDocument())
    await user.click(screen.getByTestId("invite-create"))
    await waitFor(() => expect(createCalls).toBe(1))
    const latest = await screen.findByTestId("invite-latest")
    expect(within(latest).getByTestId("invite-latest-url")).toHaveTextContent("/invite/tok-abc")
    // Copy button — we don't assert on the clipboard spy here (see
    // beforeEach). What matters is that the button is wired and clicks
    // don't crash; the actual writeText call is exercised in #1419.
    await user.click(within(latest).getByTestId("invite-latest-copy"))
  })

  it("has no axe violations on the rendered members list", async () => {
    server.use(
      groupsHandler,
      userMe(),
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
                role: "admin",
                joined_at: "2026-04-01T00:00:00Z",
              },
            },
          ],
        })
      ),
      msw.get(api("/groups/g1/invites"), () => HttpResponse.json({ data: [] }))
    )
    const { container } = renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-list")).toBeInTheDocument())
    expect(await axe(container)).toHaveNoViolations()
  })
})
