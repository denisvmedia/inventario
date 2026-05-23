import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"

// The global test setup mocks sonner to a no-op (renders nothing). Re-mock
// it locally with a spy so the block / unblock success-toast assertions can
// check the wrapper delegated — vi.mock hoists per-file so this wins.
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

import { toast } from "sonner"

import { AuthProvider } from "@/features/auth/AuthContext"
import { BackofficeAuthProvider } from "@/features/backoffice/auth/context"
import { clearBackofficeAuth, setBackofficeAccessToken } from "@/features/backoffice/auth/storage"
import { initI18n } from "@/i18n"
import { clearAuth, getImpersonationReturn, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { __resetNavigationForTests, setHardRedirect } from "@/lib/navigation"
import { AdminUserDetailPage } from "@/pages/admin/AdminUserDetailPage"
import { backofficeAuthHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

const adminUser = {
  id: "u1",
  email: "admin@example.com",
  name: "Admin",
  is_system_admin: true,
}

// The base GET /admin/users/{id} payload — an active user with one group
// membership and two live sessions.
const userDetail = {
  id: "user-1",
  type: "admin_users",
  name: "Ada Lovelace",
  email: "ada@northwind.example.com",
  tenant_id: "t1",
  is_active: true,
  is_system_admin: false,
  last_login_at: "2026-05-01T10:00:00Z",
  created_at: "2023-02-11T00:00:00Z",
  active_session_count: 2,
  group_memberships: [
    {
      group_id: "group-1",
      group_slug: "hq",
      group_name: "HQ Inventory",
      role: "owner",
      joined_at: "2024-06-01T00:00:00Z",
    },
  ],
}

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  clearBackofficeAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  __resetNavigationForTests()
  setAccessToken("good-token")
  setBackofficeAccessToken("good-bo-token")
  server.use(...backofficeAuthHandlers.signedIn())
  vi.mocked(toast.success).mockClear()
})

afterEach(() => {
  __resetNavigationForTests()
})

// Seeds /auth/me + the user-detail endpoint. `detail` overrides let a
// case ship a blocked user or a memberless one.
function seedDetail(detail: Record<string, unknown> = {}) {
  server.use(
    http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
    http.get(api("/admin/users/user-1"), () =>
      HttpResponse.json({ data: { ...userDetail, ...detail } })
    )
  )
}

function renderPage(initialPath = "/admin/users/user-1") {
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/admin/users/:userId"
        element={
          <AuthProvider>
            <BackofficeAuthProvider>
              <main>
                <AdminUserDetailPage />
              </main>
            </BackofficeAuthProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

describe("AdminUserDetailPage", () => {
  it("renders the identity card, session count, and group memberships", async () => {
    seedDetail()
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-user-identity")).toBeInTheDocument())
    expect(screen.getByRole("heading", { name: "Ada Lovelace" })).toBeInTheDocument()
    expect(screen.getByText("ada@northwind.example.com")).toBeInTheDocument()

    // Sessions render as a count summary (the BE returns only a count).
    const sessions = screen.getByTestId("admin-user-sessions")
    expect(within(sessions).getByText("2 active sessions")).toBeInTheDocument()

    // The group membership row links to the admin group detail page.
    const groupRow = screen.getByTestId("admin-user-group-row")
    expect(within(groupRow).getByText("HQ Inventory")).toBeInTheDocument()
    expect(groupRow).toHaveAttribute("href", "/admin/groups/group-1")
  })

  it("blocks a user: optimistic badge flip, success toast, dialog closes", async () => {
    // Stateful fixture: the block POST flips `is_active`, and the
    // subsequent invalidation-driven GET reflects the new value — so the
    // badge stays Blocked after the refetch settles.
    let isActive = true
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/users/user-1"), () =>
        HttpResponse.json({ data: { ...userDetail, is_active: isActive } })
      ),
      http.post(api("/admin/users/user-1/block"), () => {
        isActive = false
        return HttpResponse.json({
          data: {
            attributes: { id: "user-1", name: "Ada Lovelace", is_active: false },
          },
        })
      })
    )
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-user-block")).toBeInTheDocument())
    await userEvent.click(screen.getByTestId("admin-user-block"))

    // Confirm button is disabled until a reason is entered.
    const confirm = screen.getByTestId("admin-user-action-confirm")
    expect(confirm).toBeDisabled()
    await userEvent.type(screen.getByTestId("admin-user-action-reason"), "Policy violation")
    expect(confirm).toBeEnabled()
    await userEvent.click(confirm)

    // The dialog closes and the success toast fires.
    await waitFor(() =>
      expect(screen.queryByTestId("admin-user-action-dialog")).not.toBeInTheDocument()
    )
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("Ada Lovelace has been blocked.")
    )

    // The badge flipped to Blocked and the action is now Unblock.
    const identity = screen.getByTestId("admin-user-identity")
    expect(within(identity).getByText("Blocked")).toBeInTheDocument()
  })

  it("keeps the confirm button disabled for a whitespace-only reason", async () => {
    seedDetail()
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-user-block")).toBeInTheDocument())
    await userEvent.click(screen.getByTestId("admin-user-block"))

    const confirm = screen.getByTestId("admin-user-action-confirm")
    expect(confirm).toBeDisabled()

    // A whitespace-only reason trims to empty — the BE would reject it with
    // `reason_required`, so the gate must keep the confirm button disabled.
    await userEvent.type(screen.getByTestId("admin-user-action-reason"), "   ")
    expect(confirm).toBeDisabled()

    // A real reason re-enables it.
    await userEvent.type(screen.getByTestId("admin-user-action-reason"), "Policy violation")
    expect(confirm).toBeEnabled()
  })

  it("surfaces a typed 422 error inline and keeps the user active on block failure", async () => {
    seedDetail()
    server.use(
      http.post(api("/admin/users/user-1/block"), () =>
        HttpResponse.json(
          { errors: [{ code: "admin.block.admin_requires_force", detail: "needs force" }] },
          { status: 422 }
        )
      )
    )
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-user-block")).toBeInTheDocument())
    await userEvent.click(screen.getByTestId("admin-user-block"))
    await userEvent.type(screen.getByTestId("admin-user-action-reason"), "Block an admin")
    await userEvent.click(screen.getByTestId("admin-user-action-confirm"))

    // The typed error banner renders the specific localized message and
    // the dialog stays open.
    await waitFor(() =>
      expect(screen.getByTestId("admin-user-action-error")).toHaveTextContent(
        "system administrator"
      )
    )
    expect(screen.getByTestId("admin-user-action-dialog")).toBeInTheDocument()

    // The optimistic flip rolled back — the user is still Active.
    const identity = screen.getByTestId("admin-user-identity")
    expect(within(identity).getByText("Active")).toBeInTheDocument()
  })

  it("unblocks a blocked user: badge flips back to Active and toast appears", async () => {
    // Stateful fixture: the unblock POST flips `is_active` back to true so
    // the invalidation-driven refetch keeps the badge on Active.
    let isActive = false
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/users/user-1"), () =>
        HttpResponse.json({ data: { ...userDetail, is_active: isActive } })
      ),
      http.post(api("/admin/users/user-1/unblock"), () => {
        isActive = true
        return HttpResponse.json({
          data: {
            attributes: { id: "user-1", name: "Ada Lovelace", is_active: true },
          },
        })
      })
    )
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-user-unblock")).toBeInTheDocument())
    await userEvent.click(screen.getByTestId("admin-user-unblock"))
    await userEvent.type(screen.getByTestId("admin-user-action-reason"), "Appeal approved")
    await userEvent.click(screen.getByTestId("admin-user-action-confirm"))

    await waitFor(() =>
      expect(screen.queryByTestId("admin-user-action-dialog")).not.toBeInTheDocument()
    )
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("Ada Lovelace has been unblocked.")
    )
    const identity = screen.getByTestId("admin-user-identity")
    expect(within(identity).getByText("Active")).toBeInTheDocument()
  })

  // Each typed 422 `admin.block.*` code must map to its own localized
  // banner message — catches a typo in any BLOCK_ERROR_KEY suffix or a
  // missing `userDetail.errors.*` i18n key. The substrings are unique
  // fragments of the en catalog copy for each error.
  it.each([
    ["admin.block.self_blocked", "your own account"],
    ["admin.block.admin_requires_force", "is not supported from this screen"],
    ["admin.block.reason_required", "A reason is required"],
    ["admin.block.reason_too_long", "too long"],
  ])("surfaces the localized banner for %s", async (code, fragment) => {
    seedDetail()
    server.use(
      http.post(api("/admin/users/user-1/block"), () =>
        HttpResponse.json({ errors: [{ code, detail: "rejected" }] }, { status: 422 })
      )
    )
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-user-block")).toBeInTheDocument())
    await userEvent.click(screen.getByTestId("admin-user-block"))
    await userEvent.type(screen.getByTestId("admin-user-action-reason"), "Some reason")
    await userEvent.click(screen.getByTestId("admin-user-action-confirm"))

    await waitFor(() =>
      expect(screen.getByTestId("admin-user-action-error")).toHaveTextContent(fragment)
    )
  })

  it("surfaces a typed 422 error inline and keeps the user blocked on unblock failure", async () => {
    seedDetail({ is_active: false })
    server.use(
      http.post(api("/admin/users/user-1/unblock"), () =>
        HttpResponse.json(
          { errors: [{ code: "admin.block.reason_required", detail: "needs reason" }] },
          { status: 422 }
        )
      )
    )
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-user-unblock")).toBeInTheDocument())
    await userEvent.click(screen.getByTestId("admin-user-unblock"))
    await userEvent.type(screen.getByTestId("admin-user-action-reason"), "Reinstate")
    await userEvent.click(screen.getByTestId("admin-user-action-confirm"))

    // The typed error banner renders the specific localized message and
    // the dialog stays open.
    await waitFor(() =>
      expect(screen.getByTestId("admin-user-action-error")).toHaveTextContent(
        "A reason is required"
      )
    )
    expect(screen.getByTestId("admin-user-action-dialog")).toBeInTheDocument()

    // The optimistic flip rolled back — the user is still Blocked.
    const identity = screen.getByTestId("admin-user-identity")
    expect(within(identity).getByText("Blocked")).toBeInTheDocument()
  })

  it("does not dismiss the dialog while the block mutation is pending", async () => {
    seedDetail()
    // Hold the block POST open so the mutation stays in its pending state
    // for the duration of the dismiss attempts.
    let releaseBlock: () => void = () => {}
    const blockGate = new Promise<void>((resolve) => {
      releaseBlock = resolve
    })
    server.use(
      http.post(api("/admin/users/user-1/block"), async () => {
        await blockGate
        return HttpResponse.json({
          data: { attributes: { id: "user-1", name: "Ada Lovelace", is_active: false } },
        })
      })
    )
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-user-block")).toBeInTheDocument())
    await userEvent.click(screen.getByTestId("admin-user-block"))
    await userEvent.type(screen.getByTestId("admin-user-action-reason"), "Policy violation")
    await userEvent.click(screen.getByTestId("admin-user-action-confirm"))

    // While pending: the cancel button is disabled and an Escape press
    // does not close the dialog (onOpenChange is gated on `pending`).
    await waitFor(() => expect(screen.getByTestId("admin-user-action-cancel")).toBeDisabled())
    await userEvent.keyboard("{Escape}")
    expect(screen.getByTestId("admin-user-action-dialog")).toBeInTheDocument()

    // Releasing the request lets the mutation settle and the dialog close.
    releaseBlock()
    await waitFor(() =>
      expect(screen.queryByTestId("admin-user-action-dialog")).not.toBeInTheDocument()
    )
  })

  it("renders the not-found state when the user request 404s", async () => {
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/users/user-1"), () => HttpResponse.json({ errors: [] }, { status: 404 }))
    )
    renderPage()

    await waitFor(() => expect(screen.getByText("User not found.")).toBeInTheDocument())
  })

  describe("impersonation", () => {
    it("disables the Impersonate button for a blocked user", async () => {
      seedDetail({ is_active: false })
      renderPage()

      await waitFor(() => expect(screen.getByTestId("admin-user-impersonate")).toBeInTheDocument())
      expect(screen.getByTestId("admin-user-impersonate")).toBeDisabled()
    })

    it("disables the Impersonate button for a system-admin user", async () => {
      seedDetail({ is_system_admin: true })
      renderPage()

      await waitFor(() => expect(screen.getByTestId("admin-user-impersonate")).toBeInTheDocument())
      expect(screen.getByTestId("admin-user-impersonate")).toBeDisabled()
    })

    it("enables the Impersonate button for a normal active user", async () => {
      seedDetail()
      renderPage()

      await waitFor(() => expect(screen.getByTestId("admin-user-impersonate")).toBeEnabled())
    })

    it("confirms an impersonation: POST /impersonate → hard redirect to /", async () => {
      let impersonateCalls = 0
      seedDetail()
      server.use(
        http.post(api("/admin/users/user-1/impersonate"), () => {
          impersonateCalls++
          return HttpResponse.json({
            access_token: "target-token",
            csrf_token: "target-csrf",
            user: { id: "user-1", email: "ada@northwind.example.com", name: "Ada Lovelace" },
          })
        })
      )
      const redirect = vi.fn()
      setHardRedirect(redirect)
      renderPage()

      await waitFor(() => expect(screen.getByTestId("admin-user-impersonate")).toBeEnabled())
      await userEvent.click(screen.getByTestId("admin-user-impersonate"))

      // The impersonate dialog has NO reason textarea.
      const dialog = screen.getByTestId("admin-user-action-dialog")
      expect(within(dialog).queryByTestId("admin-user-action-reason")).not.toBeInTheDocument()

      await userEvent.click(screen.getByTestId("admin-user-action-confirm"))

      await waitFor(() => expect(impersonateCalls).toBe(1))
      await waitFor(() => expect(redirect).toHaveBeenCalledWith("/"))
      expect(getImpersonationReturn()).toEqual({ targetUserId: "user-1" })
    })

    // Each typed impersonate error code surfaces its own localized banner.
    it.each([
      ["admin.impersonate.target_is_admin", 422, "cannot be impersonated"],
      ["admin.impersonate.target_blocked", 422, "is blocked"],
      ["admin.impersonate.nested", 422, "already impersonating"],
      ["admin.impersonate.rate_limited", 429, "Too many impersonation attempts"],
    ])("surfaces the localized banner for %s", async (code, status, fragment) => {
      seedDetail()
      server.use(
        http.post(api("/admin/users/user-1/impersonate"), () =>
          HttpResponse.json({ errors: [{ code, detail: "rejected" }] }, { status })
        )
      )
      setHardRedirect(vi.fn())
      renderPage()

      await waitFor(() => expect(screen.getByTestId("admin-user-impersonate")).toBeEnabled())
      await userEvent.click(screen.getByTestId("admin-user-impersonate"))
      await userEvent.click(screen.getByTestId("admin-user-action-confirm"))

      await waitFor(() =>
        expect(screen.getByTestId("admin-user-action-error")).toHaveTextContent(fragment)
      )
      expect(screen.getByTestId("admin-user-action-dialog")).toBeInTheDocument()
    })
  })
})
