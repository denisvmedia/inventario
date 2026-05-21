import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { AuthProvider } from "@/features/auth/AuthContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { AdminGroupDetailPage } from "@/pages/admin/AdminGroupDetailPage"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

const adminUser = {
  id: "u1",
  email: "admin@example.com",
  name: "Admin",
  is_system_admin: true,
}

const activeGroup = {
  id: "g1",
  name: "HQ Inventory",
  slug: "hq",
  currency: "USD",
  status: "active",
  member_count: 4,
  created_by: "owner@acme.example.com",
  created_at: "2024-01-10T00:00:00Z",
  tenant_id: "t1",
  tenant: { id: "t1", name: "Acme Inc", slug: "acme" },
}

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

// Seeds GET /admin/groups/g1 and the soft-delete DELETE. `deleteCalls`
// counts how many times DELETE fired; the DELETE always echoes a
// pending_deletion row (matching the BE's idempotent contract).
function seedDetail(opts: { getStatus?: number; deleteCalls?: number[] } = {}) {
  server.use(
    http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
    http.get(api("/admin/groups/g1"), () => {
      if (opts.getStatus && opts.getStatus !== 200) {
        return HttpResponse.json({ errors: [] }, { status: opts.getStatus })
      }
      return HttpResponse.json({ data: activeGroup })
    }),
    http.delete(api("/admin/groups/g1"), () => {
      opts.deleteCalls?.push(1)
      return HttpResponse.json({ data: { ...activeGroup, status: "pending_deletion" } })
    })
  )
}

function renderPage(initialPath = "/admin/groups/g1") {
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/admin/groups/:groupId"
        element={
          <AuthProvider>
            <ConfirmProvider>
              <main>
                <AdminGroupDetailPage />
              </main>
            </ConfirmProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

describe("AdminGroupDetailPage", () => {
  it("renders the group header with identity and metrics", async () => {
    seedDetail()
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-group-header")).toBeInTheDocument())
    expect(screen.getByRole("heading", { name: "HQ Inventory" })).toBeInTheDocument()
    expect(screen.getByText("hq")).toBeInTheDocument()
    expect(screen.getByText("Acme Inc")).toBeInTheDocument()
    expect(screen.getByText("owner@acme.example.com")).toBeInTheDocument()
  })

  it("renders the Members panel as a labelled placeholder, not an editor", async () => {
    seedDetail()
    renderPage()

    await waitFor(() =>
      expect(screen.getByTestId("admin-group-members-placeholder")).toBeInTheDocument()
    )
    expect(
      screen.getByText("Member management ships in a follow-up update.")
    ).toBeInTheDocument()
    // No add-member control — the editor is a later sub-issue.
    expect(screen.queryByRole("button", { name: /add member/i })).not.toBeInTheDocument()
  })

  it("renders the not-found state when the group request 404s", async () => {
    seedDetail({ getStatus: 404 })
    renderPage()

    await waitFor(() => expect(screen.getByText("Group not found.")).toBeInTheDocument())
    expect(
      screen.queryByText("Could not load this group. Please try again.")
    ).not.toBeInTheDocument()
  })

  it("renders the generic error state when the group request fails", async () => {
    seedDetail({ getStatus: 500 })
    renderPage()

    await waitFor(() =>
      expect(screen.getByText("Could not load this group. Please try again.")).toBeInTheDocument()
    )
    expect(screen.queryByText("Group not found.")).not.toBeInTheDocument()
  })

  it("soft-deletes: confirm → success flips the badge and makes the page read-only", async () => {
    seedDetail()
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-group-danger-zone")).toBeInTheDocument())

    // Active group: delete button is enabled and labelled "Delete group".
    const deleteButton = screen.getByTestId("admin-group-delete-button")
    expect(deleteButton).toBeEnabled()
    expect(deleteButton).toHaveTextContent("Delete group")

    await userEvent.click(deleteButton)
    const dialog = await screen.findByTestId("confirm-dialog")
    // The confirm body explains the two-phase async purge.
    expect(within(dialog).getByText(/purge worker/i)).toBeInTheDocument()
    await userEvent.click(screen.getByTestId("confirm-accept"))

    // The cache update flips the detail to pending_deletion: the banner
    // appears and the delete button becomes a disabled "Deletion pending".
    await waitFor(() =>
      expect(screen.getByTestId("admin-group-pending-banner")).toBeInTheDocument()
    )
    const after = screen.getByTestId("admin-group-delete-button")
    expect(after).toBeDisabled()
    expect(after).toHaveTextContent("Deletion pending")
  })

  it("shows the pending-deletion banner and a disabled action for an already-pending group", async () => {
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/groups/g1"), () =>
        HttpResponse.json({ data: { ...activeGroup, status: "pending_deletion" } })
      )
    )
    renderPage()

    await waitFor(() =>
      expect(screen.getByTestId("admin-group-pending-banner")).toBeInTheDocument()
    )
    expect(screen.getByTestId("admin-group-delete-button")).toBeDisabled()
    // The idempotent re-delete is simply unreachable through the UI.
  })

  it("does not surface an error when the idempotent delete returns HTTP 200", async () => {
    const deleteCalls: number[] = []
    seedDetail({ deleteCalls })
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-group-danger-zone")).toBeInTheDocument())

    await userEvent.click(screen.getByTestId("admin-group-delete-button"))
    await userEvent.click(await screen.findByTestId("confirm-accept"))

    await waitFor(() => expect(deleteCalls).toHaveLength(1))
    await waitFor(() =>
      expect(screen.getByTestId("admin-group-pending-banner")).toBeInTheDocument()
    )
    // The DELETE returned 200 (the BE's idempotent contract) — no error
    // notice is rendered.
    expect(screen.queryByTestId("admin-group-delete-error")).not.toBeInTheDocument()
  })

  it("surfaces a delete error when the soft-delete request fails", async () => {
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/groups/g1"), () => HttpResponse.json({ data: activeGroup })),
      http.delete(api("/admin/groups/g1"), () => HttpResponse.json({ errors: [] }, { status: 500 }))
    )
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-group-danger-zone")).toBeInTheDocument())

    await userEvent.click(screen.getByTestId("admin-group-delete-button"))
    await userEvent.click(await screen.findByTestId("confirm-accept"))

    await waitFor(() =>
      expect(screen.getByTestId("admin-group-delete-error")).toBeInTheDocument()
    )
    // The group stayed active — the button is still actionable.
    expect(screen.getByTestId("admin-group-delete-button")).toHaveTextContent("Delete group")
  })
})
