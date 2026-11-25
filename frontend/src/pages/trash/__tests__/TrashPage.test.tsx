import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
// test/setup.ts mocks sonner with one spy per variant.
import { toast } from "sonner"
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest"

import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { TrashPage } from "@/pages/trash/TrashPage"
import { apiUrl, authHandlers, groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

const trashUrl = (path: string) => apiUrl(`/g/${SLUG}/trash${path}`)

function members(role: "viewer" | "user" | "admin" | "owner") {
  return http.get(apiUrl("/groups/g1/members"), () =>
    HttpResponse.json({
      data: [
        {
          id: "m1",
          type: "memberships",
          attributes: { group_id: "g1", member_user_id: "u1", role },
        },
      ],
    })
  )
}

const drill = {
  id: "c1",
  type: "commodities",
  attributes: {
    id: "c1",
    name: "Cordless drill",
    short_name: "drill",
    deleted_at: "2026-11-20T10:00:00Z",
  },
  meta: {
    deleted_by_name: "Denis",
    expires_at: "2026-12-20T10:00:00Z",
    file_count: 2,
    file_bytes: 2048,
  },
}

const saw = {
  id: "c2",
  type: "commodities",
  attributes: { id: "c2", name: "Circular saw", deleted_at: "2026-11-21T10:00:00Z" },
  meta: { expires_at: "2026-12-21T10:00:00Z", file_count: 0, file_bytes: 0 },
}

const receipt = {
  id: "f1",
  type: "files",
  attributes: {
    id: "f1",
    title: "Receipt",
    size_bytes: 1024,
    deleted_at: "2026-11-22T10:00:00Z",
    links: [{ entity_type: "location", entity_id: "l1", entity_name: "Home", meta: "files" }],
  },
  meta: { deleted_by_name: "Denis", expires_at: "2026-12-22T10:00:00Z" },
}

function listMeta(total: number) {
  return { total, page: 1, per_page: 50, total_pages: 1, retention_days: 30 }
}

function trashLists({
  items = [drill, saw],
  files = [receipt],
}: { items?: unknown[]; files?: unknown[] } = {}) {
  return [
    http.get(trashUrl("/commodities"), () =>
      HttpResponse.json({ data: items, meta: listMeta(items.length) })
    ),
    http.get(trashUrl("/files"), () =>
      HttpResponse.json({ data: files, meta: listMeta(files.length) })
    ),
  ]
}

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
  vi.mocked(toast.success).mockClear()
  vi.mocked(toast.warning).mockClear()
})

function renderPage(initialPath = `/g/${SLUG}/trash`) {
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/trash"
        element={
          <AuthProvider>
            <ConfirmProvider>
              <GroupProvider>
                <main>
                  <TrashPage />
                </main>
              </GroupProvider>
            </ConfirmProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

function setup(role: "viewer" | "user" | "admin" | "owner", lists = trashLists()) {
  server.use(
    ...authHandlers.signedIn(),
    ...groupHandlers.list(groupFixture),
    members(role),
    ...lists
  )
}

describe("<TrashPage />", () => {
  it("lists trashed items with their files and the retention", async () => {
    setup("admin")
    renderPage()

    const row = await screen.findByTestId("trash-item-c1")
    expect(within(row).getByText("Cordless drill")).toBeInTheDocument()
    expect(within(row).getByText(/2 files/)).toBeInTheDocument()
    expect(within(row).getByText("by Denis")).toBeInTheDocument()
    expect(screen.getByText(/stay here for 30 days/)).toBeInTheDocument()
    expect(screen.getByTestId("trash-tab-items")).toHaveTextContent("Items (2)")
    expect(screen.getByTestId("trash-tab-files")).toHaveTextContent("Files (1)")
    await waitFor(() => expect(screen.getByTestId("trash-empty")).toBeInTheDocument())
  })

  it("shows the empty state when nothing is in the trash", async () => {
    setup("admin", trashLists({ items: [], files: [] }))
    renderPage()
    expect(await screen.findByTestId("trash-items-empty")).toBeInTheDocument()
    expect(screen.queryByTestId("trash-empty")).toBeNull()
  })

  it("restores an item and says when it came back without a location", async () => {
    setup("user")
    let restored = ""
    server.use(
      http.post(trashUrl("/commodities/:id/restore"), ({ params }) => {
        restored = String(params.id)
        return HttpResponse.json({
          data: { id: restored, type: "commodities", attributes: { id: restored } },
          meta: { area_removed: true, files_restored: 2 },
        })
      })
    )
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId("trash-item-c1-restore"))
    await waitFor(() => expect(restored).toBe("c1"))
    await waitFor(() =>
      expect(toast.warning).toHaveBeenCalledWith(
        "Cordless drill restored without a location.",
        expect.objectContaining({ action: expect.objectContaining({ label: "Open" }) })
      )
    )
  })

  it("restores the selected items in one request", async () => {
    setup("user")
    let body: unknown = null
    server.use(
      http.post(trashUrl("/commodities/restore"), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({
          data: {
            type: "commodities.bulk_restore_result",
            id: "bulk-result",
            attributes: { succeeded: ["c1", "c2"], failed: [], area_removed: [] },
          },
        })
      })
    )
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId("trash-item-c1-select"))
    await user.click(screen.getByTestId("trash-item-c2-select"))
    await user.click(screen.getByTestId("trash-items-bulk-restore"))

    await waitFor(() =>
      expect(body).toEqual({ data: { type: "commodities", attributes: { ids: ["c1", "c2"] } } })
    )
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith("2 items restored.", undefined))
  })

  it("lets a user restore but not delete for good", async () => {
    setup("user")
    renderPage()
    expect(await screen.findByTestId("trash-item-c1-restore")).toBeInTheDocument()
    expect(screen.queryByTestId("trash-item-c1-purge")).toBeNull()
    expect(screen.queryByTestId("trash-empty")).toBeNull()
  })

  it("offers a viewer no actions", async () => {
    setup("viewer")
    renderPage()
    await screen.findByTestId("trash-item-c1")
    // Wait for the membership to load before asserting absence.
    await waitFor(() => expect(screen.queryByTestId("trash-item-c1-restore")).toBeNull())
    expect(screen.queryByTestId("trash-item-c1-purge")).toBeNull()
    expect(screen.queryByTestId("trash-item-c1-select")).toBeNull()
    expect(screen.queryByTestId("trash-empty")).toBeNull()
  })

  it("deletes an item for good after confirmation", async () => {
    setup("admin")
    let purged = ""
    server.use(
      http.delete(trashUrl("/commodities/:id"), ({ params }) => {
        purged = String(params.id)
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId("trash-item-c1-purge"))
    const dialog = await screen.findByRole("dialog")
    expect(
      within(dialog).getByText(/the 2 files that went to the trash with it/)
    ).toBeInTheDocument()
    await user.click(within(dialog).getByRole("button", { name: "Delete now" }))
    await waitFor(() => expect(purged).toBe("c1"))
  })

  it("empties the trash after confirmation", async () => {
    setup("owner")
    let emptied = false
    server.use(
      http.delete(trashUrl(""), () => {
        emptied = true
        return HttpResponse.json({
          data: { type: "trash", id: "trash", attributes: { commodities: 2, files: 1 } },
        })
      })
    )
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId("trash-empty"))
    const dialog = await screen.findByRole("dialog")
    await user.click(within(dialog).getByRole("button", { name: "Empty trash" }))
    await waitFor(() => expect(emptied).toBe(true))
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith(
        "Trash emptied. 3 entries deleted for good.",
        undefined
      )
    )
  })

  it("lists files deleted on their own with where they were attached", async () => {
    setup("admin")
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId("trash-tab-files"))
    const row = await screen.findByTestId("trash-file-f1")
    expect(within(row).getByText("Receipt")).toBeInTheDocument()
    expect(within(row).getByText("Home")).toBeInTheDocument()
  })

  it("has no axe violations", async () => {
    setup("admin")
    const { container } = renderPage()
    await screen.findByTestId("trash-item-c1")
    expect(await axe(container)).toHaveNoViolations()
  })
})
