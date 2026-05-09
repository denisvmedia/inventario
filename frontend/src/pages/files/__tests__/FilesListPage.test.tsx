import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { FilesListPage } from "@/pages/files/FilesListPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { fileHandlers, groupHandlers } from "@/test/handlers"
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

function renderPage(initialPath = `/g/${SLUG}/files`) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
    routes: (
      <>
        <Route
          path="/g/:groupSlug/files"
          element={
            <GroupProvider>
              <ConfirmProvider>
                <FilesListPage />
              </ConfirmProvider>
            </GroupProvider>
          }
        />
        <Route
          path="/g/:groupSlug/files/:id"
          element={
            <GroupProvider>
              <ConfirmProvider>
                <FilesListPage />
              </ConfirmProvider>
            </GroupProvider>
          }
        />
      </>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
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

  it("renders the four category tiles with counts from the counts endpoint", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, []),
      ...fileHandlers.counts(SLUG, { all: 11, images: 3, invoices: 5, documents: 1, other: 2 })
    )
    renderPage()
    expect(await screen.findByTestId("files-tile-all")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByTestId("files-tile-count-images")).toHaveTextContent("3")
    )
    expect(screen.getByTestId("files-tile-count-invoices")).toHaveTextContent("5")
    expect(screen.getByTestId("files-tile-count-documents")).toHaveTextContent("1")
    expect(screen.getByTestId("files-tile-count-other")).toHaveTextContent("2")
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
    await user.selectOptions(moveSelect, "documents")
    // After bulk move, selection is cleared and the bar disappears.
    await waitFor(() => expect(screen.queryByTestId("files-bulk-bar")).not.toBeInTheDocument())
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
})
