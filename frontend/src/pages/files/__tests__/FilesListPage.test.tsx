import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"
// test/setup.ts mocks sonner with one spy per variant.
import { toast } from "sonner"

import { FilesListPage } from "@/pages/files/FilesListPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { WhenRoleKnown } from "@/test/role-known"
import { server } from "@/test/server"
import { apiUrl, fileHandlers, groupHandlers, memberHandlers, tagHandlers } from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function fileRow(id: string, attrs: Partial<Schema<"models.FileEntity">> = {}) {
  return {
    id,
    attributes: {
      id,
      title: `File ${id}`,
      category: "images" as const,
      type: "image" as const,
      tags: [],
      path: `file-${id}`,
      original_path: `file-${id}.jpg`,
      ext: ".jpg",
      mime_type: "image/jpeg",
      created_at: "2026-04-30T10:00:00Z",
      ...attrs,
    },
  }
}

function Where() {
  const location = useLocation()
  return (
    <p data-testid="where">
      {location.pathname}
      {location.search}
    </p>
  )
}

function renderPage(initialPath = `/g/${SLUG}/files`) {
  setAccessToken("good-token")
  return renderWithProviders({
    withAuth: true,
    initialPath,
    routes: (
      <>
        <Route
          path="/g/:groupSlug/files"
          element={
            <GroupProvider>
              <ConfirmProvider>
                <WhenRoleKnown>
                  <FilesListPage />
                </WhenRoleKnown>
              </ConfirmProvider>
            </GroupProvider>
          }
        />
        <Route
          path="/g/:groupSlug/files/:id"
          element={
            <GroupProvider>
              <ConfirmProvider>
                <WhenRoleKnown>
                  <FilesListPage />
                </WhenRoleKnown>
                <Where />
              </ConfirmProvider>
            </GroupProvider>
          }
        />
        <Route path="/g/:groupSlug/files/:id/edit" element={<Where />} />
      </>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

// The page offers write actions by the caller's role. Every test runs as
// the group's owner unless it registers another role before rendering.
beforeEach(() => {
  server.use(...memberHandlers.asRole("g1", "owner"))
})

describe("<FilesListPage />", () => {
  it("renders the empty state when the group has no files", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, []),
      ...fileHandlers.counts(SLUG, { all: 0 })
    )
    renderPage()
    expect(await screen.findByTestId("files-empty")).toBeInTheDocument()
  })

  it("lists files returned from the BE and renders them as list rows by default", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1"), fileRow("f2")]),
      ...fileHandlers.counts(SLUG, { all: 2, images: 2 })
    )
    renderPage()
    // List view is the default per the #1538 mock — files render as
    // table rows. The grid renderer is exercised by the view-toggle test.
    expect(await screen.findByTestId("file-row-f1")).toBeInTheDocument()
    expect(screen.getByTestId("file-row-f2")).toBeInTheDocument()
    expect(screen.getAllByText("File f1").length).toBeGreaterThan(0)
  })

  it("renders the FileCard grid when ?view=grid is set in the URL", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1, images: 1 })
    )
    renderPage(`/g/${SLUG}/files?view=grid`)
    expect(await screen.findByTestId("file-card-f1")).toBeInTheDocument()
  })

  it("renders the three category tiles with counts from the counts endpoint (#1622)", async () => {
    // Post-#1622 the `invoices` tile is gone — its rows fold into
    // `documents` and the BE response shape no longer carries an
    // `invoices` count field. The tile total ("all") still equals the
    // sum across the three remaining buckets.
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, []),
      ...fileHandlers.counts(SLUG, { all: 11, images: 3, documents: 6, other: 2 })
    )
    renderPage()
    expect(await screen.findByTestId("files-tile-all")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByTestId("files-tile-count-images")).toHaveTextContent("3")
    )
    expect(screen.getByTestId("files-tile-count-documents")).toHaveTextContent("6")
    expect(screen.getByTestId("files-tile-count-other")).toHaveTextContent("2")
    expect(screen.queryByTestId("files-tile-invoices")).toBeNull()
  })

  it("filters by category when a tile is selected", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1, images: 1 })
    )
    renderPage()
    await screen.findByTestId("files-tile-images")
    // Initially "all" is selected; clicking "images" should flip
    // aria-selected so screen readers and the visual highlight track it.
    expect(screen.getByTestId("files-tile-all")).toHaveAttribute("aria-selected", "true")
    await user.click(screen.getByTestId("files-tile-images"))
    await waitFor(() =>
      expect(screen.getByTestId("files-tile-images")).toHaveAttribute("aria-selected", "true")
    )
    expect(screen.getByTestId("files-tile-all")).toHaveAttribute("aria-selected", "false")
  })

  it("opens the bulk-delete bar when a list row checkbox is selected", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1 })
    )
    renderPage()
    await screen.findByTestId("file-row-f1")
    // The desktop row checkbox is the visible-by-default branch in
    // jsdom (matchMedia stub returns matches=false → desktop).
    const checkboxes = screen.getAllByLabelText(/select file f1/i)
    await user.click(checkboxes[0])
    expect(await screen.findByTestId("files-bulk-bar")).toBeInTheDocument()
  })

  it("has no axe a11y violations on the populated list", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1, images: 1 })
    )
    const { container } = renderPage()
    await screen.findByTestId("file-row-f1")
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })

  it("falls back to category=all when the URL carries an unknown ?category= value", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1 })
    )
    renderPage(`/g/${SLUG}/files?category=warranty`)
    await screen.findByTestId("files-tile-all")
    expect(screen.getByTestId("files-tile-all")).toHaveAttribute("aria-selected", "true")
  })

  it("falls back to page=1 when the URL carries a non-numeric ?page= value", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [], { total: 0 }),
      ...fileHandlers.counts(SLUG, { all: 0 })
    )
    renderPage(`/g/${SLUG}/files?page=abc`)
    expect(await screen.findByTestId("files-empty")).toBeInTheDocument()
  })

  it("renders an error alert when the list endpoint 5xx", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.error(SLUG, 500),
      ...fileHandlers.counts(SLUG, { all: 0 })
    )
    renderPage()
    expect(await screen.findByRole("alert")).toBeInTheDocument()
  })

  it("renders the empty-state with the upload CTA when no filters are active", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, []),
      ...fileHandlers.counts(SLUG, { all: 0 })
    )
    renderPage()
    await screen.findByTestId("files-empty")
    // The empty-state subtitle is the no-filters branch; ensures the
    // hasFilters=false render path is covered.
    expect(screen.getByRole("button", { name: /upload your first file/i })).toBeInTheDocument()
  })

  it("re-categorizes selected files via the bulk-move dropdown", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1, images: 1 }),
      ...fileHandlers.update(SLUG, "f1", { id: "f1", category: "documents" })
    )
    renderPage()
    await screen.findByTestId("file-row-f1")
    const checkboxes = screen.getAllByLabelText(/select file f1/i)
    await user.click(checkboxes[0])
    await screen.findByTestId("files-bulk-bar")
    const moveSelect = screen.getByTestId("files-bulk-move") as HTMLSelectElement
    expect(moveSelect).toHaveAttribute("data-slot", "native-select")
    await user.selectOptions(moveSelect, "documents")
    // After bulk move, selection is cleared and the bar disappears.
    await waitFor(() => expect(screen.queryByTestId("files-bulk-bar")).not.toBeInTheDocument())
  })

  it("adds and removes tags on the selected files without replacing the others", async () => {
    const user = userEvent.setup()
    vi.mocked(toast.success).mockClear()
    const requests: unknown[] = []
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1"), fileRow("f2")]),
      ...fileHandlers.counts(SLUG, { all: 2, images: 2 }),
      ...tagHandlers.list(SLUG, [
        { id: "t1", slug: "receipt", label: "Receipt", color: "blue" },
        { id: "t2", slug: "old", label: "Old", color: "muted" },
      ]),
      ...fileHandlers.bulkUpdate(SLUG, { onRequest: (a) => requests.push(a) })
    )
    renderPage()
    await screen.findByTestId("file-row-f2")
    await user.click(screen.getAllByLabelText(/select file f1/i)[0])
    await user.click(screen.getAllByLabelText(/select file f2/i)[0])
    await user.click(await screen.findByTestId("files-bulk-tags"))

    const dialog = await screen.findByTestId("bulk-tags-dialog")
    expect(within(dialog).getByText("Edit the tags of 2 files")).toBeInTheDocument()
    await user.click(await within(dialog).findByTestId("bulk-tags-add-option-receipt"))
    await user.type(within(dialog).getByTestId("bulk-tags-add-input"), "Tax 2026{Enter}")
    await user.click(within(dialog).getByTestId("bulk-tags-remove-option-old"))
    expect(await axe(dialog)).toHaveNoViolations()
    await user.click(within(dialog).getByTestId("bulk-tags-confirm"))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toEqual({
      ids: ["f1", "f2"],
      patch: { tags: { add: ["receipt", "Tax 2026"], remove: ["old"] } },
    })
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("Tags changed on 2 files.", undefined)
    )
    await waitFor(() => expect(screen.queryByTestId("files-bulk-bar")).not.toBeInTheDocument())
  })

  it("keeps the files whose tags could not change selected for a retry", async () => {
    const user = userEvent.setup()
    vi.mocked(toast.warning).mockClear()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1"), fileRow("f2")]),
      ...fileHandlers.counts(SLUG, { all: 2, images: 2 }),
      ...tagHandlers.list(SLUG, [{ id: "t1", slug: "receipt", label: "Receipt", color: "blue" }]),
      ...fileHandlers.bulkUpdate(SLUG, {
        respond: () => ({
          succeeded: ["f1"],
          failed: [{ id: "f2", name: "File f2", code: "failed", error: "could not be changed" }],
        }),
      })
    )
    renderPage()
    await screen.findByTestId("file-row-f2")
    await user.click(screen.getAllByLabelText(/select file f1/i)[0])
    await user.click(screen.getAllByLabelText(/select file f2/i)[0])
    await user.click(await screen.findByTestId("files-bulk-tags"))
    const dialog = await screen.findByTestId("bulk-tags-dialog")
    await user.click(await within(dialog).findByTestId("bulk-tags-add-option-receipt"))
    await user.click(within(dialog).getByTestId("bulk-tags-confirm"))

    await waitFor(() =>
      expect(toast.warning).toHaveBeenCalledWith(
        "Tags changed on 1, 1 failed. The failed files stay selected, so you can try again.",
        undefined
      )
    )
    const bar = await screen.findByTestId("files-bulk-bar")
    expect(within(bar).getByText("1 selected")).toBeInTheDocument()
  })

  it("says that no file had the tags a removal names", async () => {
    const user = userEvent.setup()
    vi.mocked(toast.info).mockClear()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1, images: 1 }),
      ...tagHandlers.list(SLUG, [{ id: "t1", slug: "receipt", label: "Receipt", color: "blue" }]),
      ...fileHandlers.bulkUpdate(SLUG, {
        respond: () => ({ skipped: [{ id: "f1", name: "File f1", reason: "unchanged" }] }),
      })
    )
    renderPage()
    await screen.findByTestId("file-row-f1")
    await user.click(screen.getAllByLabelText(/select file f1/i)[0])
    await user.click(await screen.findByTestId("files-bulk-tags"))
    const dialog = await screen.findByTestId("bulk-tags-dialog")
    await user.click(await within(dialog).findByTestId("bulk-tags-remove-option-receipt"))
    await user.click(within(dialog).getByTestId("bulk-tags-confirm"))

    await waitFor(() =>
      expect(toast.info).toHaveBeenCalledWith(
        "None of the selected files had these tags.",
        undefined
      )
    )
  })

  it("keeps a selection made while a tag change was running", async () => {
    const user = userEvent.setup()
    vi.mocked(toast.warning).mockClear()
    let release = () => {}
    const gate = new Promise<void>((resolve) => (release = resolve))
    let answered = false
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1"), fileRow("f2"), fileRow("f3")]),
      ...fileHandlers.counts(SLUG, { all: 3, images: 3 }),
      ...tagHandlers.list(SLUG, [{ id: "t1", slug: "receipt", label: "Receipt", color: "blue" }]),
      http.post(apiUrl(`/g/${SLUG}/files/bulk-update`), async () => {
        await gate
        answered = true
        return HttpResponse.json({
          data: {
            type: "files.bulk_result",
            attributes: {
              succeeded: ["f1"],
              skipped: [],
              failed: [
                { id: "f2", name: "File f2", code: "failed", error: "could not be changed" },
              ],
            },
          },
        })
      })
    )
    renderPage()
    await screen.findByTestId("file-row-f3")
    await user.click(screen.getAllByLabelText(/select file f1/i)[0])
    await user.click(screen.getAllByLabelText(/select file f2/i)[0])
    await user.click(await screen.findByTestId("files-bulk-tags"))
    const dialog = await screen.findByTestId("bulk-tags-dialog")
    await user.click(await within(dialog).findByTestId("bulk-tags-add-option-receipt"))
    await user.click(within(dialog).getByTestId("bulk-tags-confirm"))

    // While the change runs, the user closes the dialog and picks a third file.
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }))
    await user.click(screen.getAllByLabelText(/select file f3/i)[0])

    release()
    await waitFor(() => expect(answered).toBe(true))
    await waitFor(() => expect(toast.warning).toHaveBeenCalled())
    // The failed file does not replace the selection the user made since.
    const bar = await screen.findByTestId("files-bulk-bar")
    expect(within(bar).getByText("3 selected")).toBeInTheDocument()
  })

  it("reflects the active tag filter from the URL on the matching pill", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1", { tags: ["invoice"] })]),
      ...fileHandlers.counts(SLUG, { all: 1 })
    )
    renderPage(`/g/${SLUG}/files?tags=invoice`)
    await screen.findByTestId("file-row-f1")
    // The curated pill matching the URL tag is pressed; the others are
    // not. Custom tags (not in FILE_TAG_PILLS) don't render as pills.
    expect(screen.getByTestId("files-tag-pill-invoice")).toHaveAttribute("aria-pressed", "true")
    expect(screen.getByTestId("files-tag-pill-warranty")).toHaveAttribute("aria-pressed", "false")
  })

  it("toggles a curated tag pill — flips aria-pressed and reveals Clear all", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, []),
      ...fileHandlers.counts(SLUG, { all: 0 })
    )
    renderPage()
    const pill = await screen.findByTestId("files-tag-pill-warranty")
    expect(pill).toHaveAttribute("aria-pressed", "false")
    expect(screen.queryByTestId("files-tag-clear")).not.toBeInTheDocument()
    await user.click(pill)
    await waitFor(() =>
      expect(screen.getByTestId("files-tag-pill-warranty")).toHaveAttribute("aria-pressed", "true")
    )
    expect(screen.getByTestId("files-tag-clear")).toBeInTheDocument()
  })

  it("filters by any file tag from the tag picker and shows it as a pill", async () => {
    const user = userEvent.setup()
    const queries: URLSearchParams[] = []
    server.use(
      ...groupHandlers.list(groupFixture),
      ...tagHandlers.list(SLUG, [
        { id: "t1", slug: "receipt-2024", label: "Receipt 2024", color: "amber" },
      ]),
      http.get(apiUrl(`/g/${SLUG}/files`), ({ request }) => {
        queries.push(new URL(request.url).searchParams)
        return HttpResponse.json({ data: [], meta: { total: 0 } })
      }),
      ...fileHandlers.counts(SLUG, { all: 0 })
    )
    renderPage()
    await user.click(await screen.findByTestId("files-tag-picker"))
    await user.click(await screen.findByTestId("files-tag-picker-option-receipt-2024"))
    await waitFor(() => expect(queries[queries.length - 1].get("tags")).toBe("receipt-2024"))
    await user.keyboard("{Escape}")

    const pill = screen.getByTestId("files-tag-pill-receipt-2024")
    expect(pill).toHaveTextContent("Receipt 2024")
    expect(pill).toHaveAttribute("aria-pressed", "true")
    await user.click(pill)
    await waitFor(() => expect(queries[queries.length - 1].has("tags")).toBe(false))
    expect(screen.queryByTestId("files-tag-pill-receipt-2024")).toBeNull()
  })

  it("toggles between list and grid views via the toolbar buttons", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1, images: 1 })
    )
    renderPage()
    // Default = list view (per the #1538 mock).
    expect(await screen.findByTestId("files-list")).toBeInTheDocument()
    expect(screen.getByTestId("file-row-f1")).toBeInTheDocument()
    expect(screen.queryByTestId("files-grid")).not.toBeInTheDocument()

    await user.click(screen.getByTestId("files-view-grid"))
    expect(await screen.findByTestId("files-grid")).toBeInTheDocument()
    expect(screen.getByTestId("file-card-f1")).toBeInTheDocument()
    expect(screen.queryByTestId("files-list")).not.toBeInTheDocument()
  })

  it("renders the cumulative footer with humanised total bytes", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, {
        all: 1,
        images: 1,
        bytes: { images: 1024, all: 1024 },
      })
    )
    renderPage()
    const footer = await screen.findByTestId("files-cumulative-footer")
    // Shape of "{N} file · {Y} total" — formatBytes() emits binary
    // units (KiB / MiB / …) per ECMAScript Intl conventions; the
    // assertion is loose on the unit so locale shifts don't fail the
    // test.
    expect(footer.textContent).toMatch(/1\s*file/i)
    expect(footer.textContent).toMatch(/total/i)
    expect(footer.textContent).toMatch(/[KMG]i?B/i)
  })

  it("opens the file detail sheet when navigating to /files/:id directly", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1 }),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "Direct file",
        category: "images",
        type: "image",
        path: "direct",
        ext: ".jpg",
        mime_type: "image/jpeg",
      })
    )
    renderPage(`/g/${SLUG}/files/f1`)
    expect(await screen.findByTestId("file-detail-sheet")).toBeInTheDocument()
  })

  it("renders pagination controls when total exceeds the page size", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(
        SLUG,
        Array.from({ length: 24 }, (_, i) => fileRow(`f${i}`)),
        { total: 50 }
      ),
      ...fileHandlers.counts(SLUG, { all: 50 })
    )
    renderPage()
    await screen.findByTestId("file-row-f0")
    expect(screen.getByTestId("files-pagination")).toBeInTheDocument()
  })

  it("moves from a page past the end to the last page with files", async () => {
    const rows = Array.from({ length: 24 }, (_, i) => fileRow(`f${i}`))
    server.use(
      ...groupHandlers.list(groupFixture),
      http.get(apiUrl(`/g/${SLUG}/files`), ({ request }) => {
        const page = Number(new URL(request.url).searchParams.get("page")) || 1
        return HttpResponse.json({ data: page === 1 ? rows : [], meta: { total: rows.length } })
      }),
      ...fileHandlers.counts(SLUG, { all: 24 })
    )
    // Page 2 of a list that a save has just shrunk to one page.
    renderPage(`/g/${SLUG}/files?page=2`)
    expect(await screen.findByTestId("file-row-f0")).toBeInTheDocument()
    expect(screen.queryByTestId("files-pagination")).toBeNull()
  })

  it("preserves search input state and submits to the list query", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1 })
    )
    renderPage()
    await screen.findByTestId("file-row-f1")
    const input = screen.getByTestId("files-search-input") as HTMLInputElement
    await user.type(input, "invoice{Enter}")
    // The toolbar pendingSearch state was committed to URL params; the
    // input retains the typed value through the rerender.
    await waitFor(() => expect(input.value).toBe("invoice"))
  })

  it("keeps the list's filters, page and layout behind the detail panel", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1", { tags: ["invoice"] })]),
      ...fileHandlers.counts(SLUG, { all: 1 }),
      ...fileHandlers.detail(SLUG, "f1", { id: "f1", title: "Invoice", path: "invoice" })
    )
    renderPage(`/g/${SLUG}/files?tags=invoice&view=list`)
    await screen.findByTestId("file-row-f1")

    await user.click(within(screen.getByTestId("file-row-f1")).getByRole("button"))
    await screen.findByTestId("file-detail-sheet")
    expect(screen.getByTestId("where")).toHaveTextContent(
      `/g/${SLUG}/files/f1?tags=invoice&view=list`
    )

    await user.keyboard("{Escape}")
    await waitFor(() => expect(screen.queryByTestId("file-detail-sheet")).not.toBeInTheDocument())
    expect(screen.getByTestId("files-tag-pill-invoice")).toHaveAttribute("aria-pressed", "true")
  })

  it("opens the review page with this list as its context", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1", { category: "documents" })]),
      ...fileHandlers.counts(SLUG, { all: 1 }),
      ...fileHandlers.detail(SLUG, "f1", { id: "f1", title: "Manual", path: "manual" })
    )
    renderPage(`/g/${SLUG}/files/f1?category=documents&page=1&view=grid`)

    await user.click(await screen.findByTestId("file-detail-edit"))
    const where = new URL(screen.getByTestId("where").textContent ?? "", "http://app.test")
    expect(where.pathname).toBe(`/g/${SLUG}/files/f1/edit`)
    expect(where.searchParams.get("from")).toBe(
      `/g/${SLUG}/files?category=documents&page=1&view=grid`
    )
  })
})

describe("<FilesListPage /> by role (#2886)", () => {
  // Files take writes from users, admins and owners; a viewer browses,
  // opens and downloads.
  it("offers a viewer no upload, selection, Edit or Delete", async () => {
    server.use(
      ...memberHandlers.asRole("g1", "viewer"),
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1", { category: "documents" })]),
      ...fileHandlers.counts(SLUG, { all: 1 }),
      ...fileHandlers.detail(SLUG, "f1", { id: "f1", title: "Manual", path: "manual" })
    )
    renderPage(`/g/${SLUG}/files/f1`)
    expect(await screen.findByTestId("file-detail-filename")).toHaveTextContent("manual")
    expect(screen.queryByTestId("file-detail-edit")).not.toBeInTheDocument()
    expect(screen.queryByTestId("file-detail-delete")).not.toBeInTheDocument()
    expect(screen.getByTestId("file-row-f1")).toBeInTheDocument()
    expect(screen.queryByTestId("file-row-checkbox-f1")).not.toBeInTheDocument()
    expect(screen.queryByTestId("files-list-select-all")).not.toBeInTheDocument()
    expect(screen.queryByTestId("files-upload-cta")).not.toBeInTheDocument()
  })

  it("offers a viewer no upload in the empty state", async () => {
    server.use(
      ...memberHandlers.asRole("g1", "viewer"),
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, []),
      ...fileHandlers.counts(SLUG, { all: 0 })
    )
    renderPage()
    const empty = await screen.findByTestId("files-empty")
    expect(within(empty).queryByRole("button")).not.toBeInTheDocument()
    expect(screen.queryByTestId("files-upload-cta")).not.toBeInTheDocument()
  })

  it("offers a user upload and selection", async () => {
    server.use(
      ...memberHandlers.asRole("g1", "user"),
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1 })
    )
    renderPage()
    expect(await screen.findByTestId("file-row-checkbox-f1")).toBeInTheDocument()
    expect(screen.getByTestId("files-upload-cta")).toBeInTheDocument()
  })
})
