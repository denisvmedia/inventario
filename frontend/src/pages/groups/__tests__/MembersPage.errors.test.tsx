import { beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

// Local sonner mock per-file: the global setup turns `toast.error` into a
// no-op (test/setup.ts) so the tree never portals an actual <Toaster /> —
// fine for everything else, but here we need to assert the wrapper picked
// the typed-code copy instead of the generic fallback. `vi.mock` hoists
// per-file, so this override wins inside this file only.
vi.mock("sonner", () => {
  const toastError = vi.fn(() => "stub-toast-id")
  const noop = vi.fn(() => "stub-toast-id")
  return {
    Toaster: () => null,
    toast: Object.assign(noop, {
      success: noop,
      error: toastError,
      info: noop,
      warning: noop,
      message: noop,
      promise: noop,
      dismiss: vi.fn(),
      loading: noop,
      custom: noop,
    }),
  }
})

import { toast } from "sonner"

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

const userMeHandler = msw.get(api("/auth/me"), () =>
  HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
)

// Two-row roster: the current user is the owner, plus a second non-self
// member whose row exposes the actions dropdown (self-row is gated out
// by the page). The remove button on the non-self row is the surface we
// drive in these tests.
const twoMemberRoster = msw.get(api("/groups/g1/members"), () =>
  HttpResponse.json({
    data: [
      {
        id: "m1",
        type: "memberships",
        attributes: {
          group_id: "g1",
          member_user_id: "u1",
          role: "owner",
          joined_at: "2026-04-01T00:00:00Z",
          user: { id: "u1", name: "Alex Doe", email: "alex@example.com" },
        },
      },
      {
        id: "m2",
        type: "memberships",
        attributes: {
          group_id: "g1",
          member_user_id: "u2-other",
          role: "user",
          joined_at: "2026-04-02T00:00:00Z",
          user: { id: "u2-other", name: "Bea Smith", email: "bea@example.com" },
        },
      },
    ],
  })
)

const invitesEmpty = msw.get(api("/groups/g1/invites"), () => HttpResponse.json({ data: [] }))

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
  ;(toast.error as ReturnType<typeof vi.fn>).mockClear()
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
  })
})

// #1652: the row-level dropdown disables the remove button when
// isLastOwner, but a stale membership snapshot (loading flicker, role
// drift, or a sysadmin-side change committed mid-render) can let a
// click through. Both invariants ship distinct toast copy so the user
// gets an actionable remediation rather than a generic "Couldn't load"
// message that buries the actual cause.

describe("<MembersPage /> typed error toasts (#1652)", () => {
  it("surfaces the lastOwner copy when DELETE returns group.last_owner", async () => {
    server.use(
      groupsHandler,
      userMeHandler,
      twoMemberRoster,
      invitesEmpty,
      msw.delete(api("/groups/g1/members/u2-other"), () =>
        HttpResponse.json(
          { errors: [{ code: "group.last_owner", detail: "sole owner" }] },
          { status: 422 }
        )
      )
    )
    const user = userEvent.setup()
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-list")).toBeInTheDocument())
    await user.click(screen.getByTestId("member-actions-u2-other"))
    await user.click(await screen.findByTestId("remove-member-btn-u2-other"))
    await user.click(await screen.findByTestId("confirm-accept"))

    await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1))
    // The toast carries the localized "last owner" copy (i18n
    // members:errors.lastOwner), not the generic fallback. We match on
    // the leading words so harmless copy tweaks don't force a test
    // update — the assertion's point is that we landed on the typed
    // branch and not on `parseServerError`'s "Couldn't load".
    expect((toast.error as ReturnType<typeof vi.fn>).mock.calls[0][0]).toMatch(/last owner/i)
  })

  it("surfaces the lastMember copy when DELETE returns group.last_member", async () => {
    server.use(
      groupsHandler,
      userMeHandler,
      twoMemberRoster,
      invitesEmpty,
      msw.delete(api("/groups/g1/members/u2-other"), () =>
        HttpResponse.json(
          { errors: [{ code: "group.last_member", detail: "leaves zero members" }] },
          { status: 422 }
        )
      )
    )
    const user = userEvent.setup()
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-list")).toBeInTheDocument())
    await user.click(screen.getByTestId("member-actions-u2-other"))
    await user.click(await screen.findByTestId("remove-member-btn-u2-other"))
    await user.click(await screen.findByTestId("confirm-accept"))

    await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1))
    // members:errors.lastMember — "delete the group instead" is the
    // actionable hint the user actually needs.
    expect((toast.error as ReturnType<typeof vi.fn>).mock.calls[0][0]).toMatch(
      /delete the group instead/i
    )
  })

  it("falls back to the generic parseServerError copy for an unrelated 422", async () => {
    server.use(
      groupsHandler,
      userMeHandler,
      twoMemberRoster,
      invitesEmpty,
      msw.delete(api("/groups/g1/members/u2-other"), () =>
        HttpResponse.json(
          { errors: [{ code: "validation_error", detail: "something else" }] },
          { status: 422 }
        )
      )
    )
    const user = userEvent.setup()
    renderMembers()
    await waitFor(() => expect(screen.getByTestId("members-list")).toBeInTheDocument())
    await user.click(screen.getByTestId("member-actions-u2-other"))
    await user.click(await screen.findByTestId("remove-member-btn-u2-other"))
    await user.click(await screen.findByTestId("confirm-accept"))

    await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1))
    // The typed-code map didn't match → parseServerError won. Assert
    // we didn't accidentally render either typed copy (otherwise the
    // map fallthrough is broken).
    const msg = (toast.error as ReturnType<typeof vi.fn>).mock.calls[0][0] as string
    expect(msg).not.toMatch(/last owner/i)
    expect(msg).not.toMatch(/delete the group instead/i)
  })
})
