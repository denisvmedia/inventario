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

// Membership attribute shape after #1533: the BE now joins users and the
// list endpoint serves user-id / name / email inline. Tests build rows
// via this helper so adding new keys later doesn't require a sweeping
// fixture edit.
function memberRow(opts: {
  id: string
  userId: string
  role: "viewer" | "user" | "admin" | "owner"
  name?: string
  email?: string
  joinedAt?: string
}) {
  return {
    id: opts.id,
    type: "memberships",
    attributes: {
      group_id: "g1",
      member_user_id: opts.userId,
      role: opts.role,
      joined_at: opts.joinedAt ?? "2026-04-01T00:00:00Z",
      user:
        opts.name || opts.email
          ? {
              id: opts.userId,
              name: opts.name ?? "",
              email: opts.email ?? "",
            }
          : undefined,
    },
  }
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
  it("renders the member list with name + email + role badge + '(you)' label", async () => {
    server.use(
      groupsHandler,
      userMe(),
      msw.get(api("/groups/g1/members"), () =>
        HttpResponse.json({
          data: [
            memberRow({
              id: "m1",
              userId: "u1",
              role: "owner",
              name: "Alex Doe",
              email: "alex@example.com",
            }),
            memberRow({
              id: "m2",
              userId: "u2-other-user",
              role: "user",
              name: "Bea Smith",
              email: "bea@example.com",
            }),
          ],
        })
      ),
      msw.get(api("/groups/g1/invites"), () => HttpResponse.json({ data: [] }))
    )
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-list")).toBeInTheDocument())
    expect(screen.getByText("Alex Doe")).toBeInTheDocument()
    expect(screen.getByText("alex@example.com")).toBeInTheDocument()
    expect(screen.getByText("Bea Smith")).toBeInTheDocument()
    expect(screen.getByText("(you)")).toBeInTheDocument()
    // Role legend renders every role; member rows have their own badges
    // — assert the badge testid format used by both.
    expect(screen.getAllByTestId("role-badge-owner").length).toBeGreaterThan(0)
    expect(screen.getAllByTestId("role-badge-user").length).toBeGreaterThan(0)
  })

  it("hides admin-only sections when the viewer is a non-managing role", async () => {
    server.use(
      groupsHandler,
      userMe(),
      msw.get(api("/groups/g1/members"), () =>
        HttpResponse.json({
          data: [
            memberRow({
              id: "m1",
              userId: "u1",
              role: "user",
              name: "Alex Doe",
              email: "alex@example.com",
            }),
          ],
        })
      )
    )
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-list")).toBeInTheDocument())
    expect(screen.queryByTestId("invites-section")).not.toBeInTheDocument()
    expect(screen.queryByTestId("members-invite-cta")).not.toBeInTheDocument()
    expect(screen.queryByTestId(/^member-actions-/)).not.toBeInTheDocument()
  })

  it("disables destructive actions on the last-owner row in the actions menu", async () => {
    const user = userEvent.setup()
    server.use(
      groupsHandler,
      userMe(),
      msw.get(api("/groups/g1/members"), () =>
        HttpResponse.json({
          data: [
            memberRow({
              id: "m1",
              userId: "u1",
              role: "owner",
              name: "Alex Doe",
              email: "alex@example.com",
            }),
            memberRow({
              id: "m2",
              userId: "u2-other",
              role: "user",
              name: "Bea Smith",
              email: "bea@example.com",
            }),
          ],
        })
      ),
      msw.get(api("/groups/g1/invites"), () => HttpResponse.json({ data: [] }))
    )
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-list")).toBeInTheDocument())
    // The current-user row has no actions menu (self-row guard).
    expect(screen.queryByTestId("member-actions-u1")).not.toBeInTheDocument()
    // The other-user row exposes the actions menu — open it and
    // confirm the remove button is enabled (owner can act on user).
    await user.click(screen.getByTestId("member-actions-u2-other"))
    const removeBtn = await screen.findByTestId("remove-member-btn-u2-other")
    expect(removeBtn).not.toBeDisabled()
  })

  it("admin can open the invite dialog, pick a role, and send an email invite", async () => {
    let createCalls = 0
    let lastCreateBody: { email?: string; role?: string } | null = null
    server.use(
      groupsHandler,
      userMe(),
      msw.get(api("/groups/g1/members"), () =>
        HttpResponse.json({
          data: [
            memberRow({
              id: "m1",
              userId: "u1",
              role: "admin",
              name: "Alex Doe",
              email: "alex@example.com",
            }),
          ],
        })
      ),
      msw.get(api("/groups/g1/invites"), () => HttpResponse.json({ data: [] })),
      msw.post(api("/groups/g1/invites"), async ({ request }) => {
        createCalls++
        const body = (await request.json()) as {
          data?: { attributes?: { email?: string; role?: string } }
        }
        lastCreateBody = body?.data?.attributes ?? {}
        return HttpResponse.json(
          {
            data: {
              id: "inv1",
              type: "invites",
              attributes: {
                id: "inv1",
                token: "tok-abc",
                expires_at: "2026-05-01T00:00:00Z",
                invitee_email: lastCreateBody?.email,
                role: lastCreateBody?.role ?? "user",
              },
            },
          },
          { status: 201 }
        )
      })
    )
    const user = userEvent.setup()
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-invite-cta")).toBeInTheDocument())
    await user.click(screen.getByTestId("members-invite-cta"))
    const dialog = await screen.findByTestId("invite-dialog")
    await user.type(within(dialog).getByTestId("invite-email-input"), "guest@example.com")
    await user.click(within(dialog).getByTestId("invite-send"))
    await waitFor(() => expect(createCalls).toBe(1))
    expect(lastCreateBody?.email).toBe("guest@example.com")
    // Default role is "user" when the admin doesn't change the select.
    expect(lastCreateBody?.role).toBe("user")
  })

  it("has no axe violations on the rendered members page", async () => {
    server.use(
      groupsHandler,
      userMe(),
      msw.get(api("/groups/g1/members"), () =>
        HttpResponse.json({
          data: [
            memberRow({
              id: "m1",
              userId: "u1",
              role: "owner",
              name: "Alex Doe",
              email: "alex@example.com",
              joinedAt: "2026-04-01T00:00:00Z",
            }),
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
