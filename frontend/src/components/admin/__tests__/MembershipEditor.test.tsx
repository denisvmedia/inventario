import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { MembershipEditor } from "@/components/admin/MembershipEditor"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

const adminUser = {
  id: "u1",
  email: "admin@example.com",
  name: "Admin",
  is_system_admin: true,
}

// A populated roster — one owner, one viewer.
const members = [
  {
    id: "m1",
    type: "admin_group_members",
    group_id: "g1",
    member_user_id: "user-owner",
    role: "owner",
    joined_at: "2026-01-10T00:00:00Z",
    user: { id: "user-owner", name: "Jordan Doe", email: "jordan@acme.example.com" },
  },
  {
    id: "m2",
    type: "admin_group_members",
    group_id: "g1",
    member_user_id: "user-viewer",
    role: "viewer",
    joined_at: "2026-02-01T00:00:00Z",
    user: { id: "user-viewer", name: "Casey Lin", email: "casey@acme.example.com" },
  },
]

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

// Seeds /auth/me (so useIsSystemAdmin gates the editor's queries open) and
// the members-list endpoint. `roster` overrides the seeded list.
function seedMembers(roster: unknown[] = members) {
  server.use(
    http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
    http.get(api("/admin/groups/g1/members"), () => HttpResponse.json({ data: roster }))
  )
}

function renderEditor(props: Partial<Parameters<typeof MembershipEditor>[0]> = {}) {
  return renderWithProviders({
    withAuth: true,
    children: (
      <ConfirmProvider>
        <MembershipEditor
          groupId="g1"
          groupName="HQ Inventory"
          tenantId="t1"
          tenantName="Acme Corp"
          {...props}
        />
      </ConfirmProvider>
    ),
  })
}

describe("MembershipEditor", () => {
  it("renders the members table from the mocked list", async () => {
    seedMembers()
    renderEditor()

    await waitFor(() => expect(screen.getAllByTestId("admin-group-member-row")).toHaveLength(2))
    expect(screen.getByText("Jordan Doe")).toBeInTheDocument()
    expect(screen.getByText("casey@acme.example.com")).toBeInTheDocument()
  })

  it("renders the empty state for a group with no members", async () => {
    seedMembers([])
    renderEditor()

    await waitFor(() => expect(screen.getByText("No members in this group.")).toBeInTheDocument())
  })

  it("adds a member: email lookup resolves, then POSTs the userID", async () => {
    seedMembers([])
    let postBody: { userID?: string; role?: string } | null = null
    server.use(
      // The tenant-user search the dialog debounces — returns the exact
      // match for the typed email.
      http.get(api("/admin/tenants/t1/users"), () =>
        HttpResponse.json({
          data: [{ id: "user-new", name: "Pat Quinn", email: "pat@acme.example.com" }],
          meta: {},
        })
      ),
      http.post(api("/admin/groups/g1/members"), async ({ request }) => {
        postBody = (await request.json()) as { userID?: string; role?: string }
        return HttpResponse.json({ data: { id: "m3", type: "group_memberships" } }, { status: 201 })
      })
    )
    renderEditor()

    await waitFor(() => expect(screen.getByTestId("admin-group-members-add")).toBeInTheDocument())
    await userEvent.click(screen.getByTestId("admin-group-members-add"))

    const confirm = screen.getByTestId("admin-group-add-confirm")
    expect(confirm).toBeDisabled()

    await userEvent.type(screen.getByTestId("admin-group-add-email"), "pat@acme.example.com")
    // The debounced lookup resolves the user and enables the Add button.
    await waitFor(() => expect(screen.getByTestId("admin-group-add-resolved")).toBeInTheDocument())
    await waitFor(() => expect(confirm).toBeEnabled())

    await userEvent.click(confirm)
    await waitFor(() => expect(postBody).toEqual({ userID: "user-new", role: "user" }))
    // The dialog closes on success.
    await waitFor(() =>
      expect(screen.queryByTestId("admin-group-add-dialog")).not.toBeInTheDocument()
    )
  })

  it("shows a not-found notice when no tenant user matches the email", async () => {
    seedMembers([])
    server.use(
      http.get(api("/admin/tenants/t1/users"), () => HttpResponse.json({ data: [], meta: {} }))
    )
    renderEditor()

    await userEvent.click(await screen.findByTestId("admin-group-members-add"))
    await userEvent.type(screen.getByTestId("admin-group-add-email"), "ghost@acme.example.com")

    await waitFor(() => expect(screen.getByTestId("admin-group-add-not-found")).toBeInTheDocument())
    expect(screen.getByTestId("admin-group-add-confirm")).toBeDisabled()
  })

  it("surfaces the tenant_mismatch typed error inside the add dialog", async () => {
    seedMembers([])
    server.use(
      http.get(api("/admin/tenants/t1/users"), () =>
        HttpResponse.json({
          data: [{ id: "user-x", name: "Other Tenant", email: "x@other.example.com" }],
          meta: {},
        })
      ),
      http.post(api("/admin/groups/g1/members"), () =>
        HttpResponse.json(
          { errors: [{ code: "admin.member.tenant_mismatch", detail: "wrong tenant" }] },
          { status: 422 }
        )
      )
    )
    renderEditor()

    await userEvent.click(await screen.findByTestId("admin-group-members-add"))
    await userEvent.type(screen.getByTestId("admin-group-add-email"), "x@other.example.com")
    await waitFor(() => expect(screen.getByTestId("admin-group-add-confirm")).toBeEnabled())
    await userEvent.click(screen.getByTestId("admin-group-add-confirm"))

    await waitFor(() =>
      expect(screen.getByTestId("admin-group-add-error")).toHaveTextContent("different tenant")
    )
    expect(screen.getByTestId("admin-group-add-dialog")).toBeInTheDocument()
  })

  it("removes a member: confirm fires the DELETE", async () => {
    seedMembers()
    let deleteUrl: string | null = null
    server.use(
      http.delete(api("/admin/groups/g1/members/:userId"), ({ request }) => {
        deleteUrl = new URL(request.url).pathname
        return new HttpResponse(null, { status: 204 })
      })
    )
    renderEditor()

    await waitFor(() => expect(screen.getAllByTestId("admin-group-member-row")).toHaveLength(2))
    const ownerRow = screen.getByText("Jordan Doe").closest("tr")!
    await userEvent.click(within(ownerRow).getByTestId("admin-group-member-actions"))
    await userEvent.click(screen.getByTestId("admin-group-member-remove"))

    // The remove confirmation is the shared useConfirm dialog.
    await userEvent.click(await screen.findByTestId("confirm-accept"))
    await waitFor(() => expect(deleteUrl).toBe("/api/v1/admin/groups/g1/members/user-owner"))
  })

  it("surfaces the last_owner typed error inline on a failed remove", async () => {
    seedMembers()
    server.use(
      http.delete(api("/admin/groups/g1/members/:userId"), () =>
        HttpResponse.json(
          { errors: [{ code: "group.last_owner", detail: "sole owner" }] },
          { status: 422 }
        )
      )
    )
    renderEditor()

    await waitFor(() => expect(screen.getAllByTestId("admin-group-member-row")).toHaveLength(2))
    const ownerRow = screen.getByText("Jordan Doe").closest("tr")!
    await userEvent.click(within(ownerRow).getByTestId("admin-group-member-actions"))
    await userEvent.click(screen.getByTestId("admin-group-member-remove"))
    await userEvent.click(await screen.findByTestId("confirm-accept"))

    await waitFor(() =>
      expect(screen.getByTestId("admin-group-members-error")).toHaveTextContent(
        "Transfer ownership first"
      )
    )
  })

  it("changes a role: PATCHes the new role", async () => {
    seedMembers()
    let patchBody: { role?: string } | null = null
    server.use(
      http.patch(api("/admin/groups/g1/members/:userId"), async ({ request }) => {
        patchBody = (await request.json()) as { role?: string }
        return HttpResponse.json({ data: { id: "m2", type: "group_memberships" } })
      })
    )
    renderEditor()

    await waitFor(() => expect(screen.getAllByTestId("admin-group-member-row")).toHaveLength(2))
    const viewerRow = screen.getByText("Casey Lin").closest("tr")!
    // The viewer row's inline role <Select> — open it and pick "Administrator".
    await userEvent.click(within(viewerRow).getByTestId("admin-group-member-role"))
    await userEvent.click(screen.getByRole("option", { name: /administrator/i }))

    await waitFor(() => expect(patchBody).toEqual({ role: "admin" }))
  })

  it("surfaces the last_owner typed error inline on a failed role change", async () => {
    seedMembers()
    server.use(
      http.patch(api("/admin/groups/g1/members/:userId"), () =>
        HttpResponse.json(
          { errors: [{ code: "group.last_owner", detail: "sole owner" }] },
          { status: 422 }
        )
      )
    )
    renderEditor()

    await waitFor(() => expect(screen.getAllByTestId("admin-group-member-row")).toHaveLength(2))
    const ownerRow = screen.getByText("Jordan Doe").closest("tr")!
    await userEvent.click(within(ownerRow).getByTestId("admin-group-member-role"))
    await userEvent.click(screen.getByRole("option", { name: /^viewer$/i }))

    await waitFor(() =>
      expect(screen.getByTestId("admin-group-members-error")).toHaveTextContent(
        "Transfer ownership first"
      )
    )
  })

  it("hides every mutating control in read-only mode", async () => {
    seedMembers()
    renderEditor({ readOnly: true })

    await waitFor(() => expect(screen.getAllByTestId("admin-group-member-row")).toHaveLength(2))
    // No Add button, no per-row actions, no inline role Select.
    expect(screen.queryByTestId("admin-group-members-add")).not.toBeInTheDocument()
    expect(screen.queryByTestId("admin-group-member-actions")).not.toBeInTheDocument()
    expect(screen.queryByTestId("admin-group-member-role")).not.toBeInTheDocument()
    // The role still renders, as static text.
    expect(screen.getByText("Owner")).toBeInTheDocument()
  })
})
