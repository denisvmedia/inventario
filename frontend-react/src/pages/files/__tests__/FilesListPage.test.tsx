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
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
]

function fileRow(id: string, attrs: Partial<Schema<"models.FileEntity">> = {}) {
  return {
    id,
    attributes: {
      id,
      title: `File ${id}`,
      category: "photos" as const,
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

  it("lists files returned from the BE and renders them as cards", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1"), fileRow("f2")]),
      ...fileHandlers.counts(SLUG, { all: 2, photos: 2 })
    )
    renderPage()
    expect(await screen.findByTestId("file-card-f1")).toBeInTheDocument()
    expect(screen.getByTestId("file-card-f2")).toBeInTheDocument()
    expect(screen.getByText("File f1")).toBeInTheDocument()
  })

  it("renders the four category tiles with counts from the counts endpoint", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, []),
      ...fileHandlers.counts(SLUG, { all: 11, photos: 3, invoices: 5, documents: 1, other: 2 })
    )
    renderPage()
    expect(await screen.findByTestId("files-tile-all")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByTestId("files-tile-count-photos")).toHaveTextContent("3")
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
      ...fileHandlers.counts(SLUG, { all: 1, photos: 1 })
    )
    renderPage()
    await screen.findByTestId("files-tile-photos")
    // Initially "all" is selected; clicking "photos" should flip
    // aria-selected so screen readers and the visual highlight track it.
    expect(screen.getByTestId("files-tile-all")).toHaveAttribute("aria-selected", "true")
    await user.click(screen.getByTestId("files-tile-photos"))
    await waitFor(() =>
      expect(screen.getByTestId("files-tile-photos")).toHaveAttribute("aria-selected", "true")
    )
    expect(screen.getByTestId("files-tile-all")).toHaveAttribute("aria-selected", "false")
  })

  it("opens the bulk-delete bar when a card is selected", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1 })
    )
    renderPage()
    await screen.findByTestId("file-card-f1")
    await user.click(screen.getByTestId("file-card-checkbox-f1"))
    expect(await screen.findByTestId("files-bulk-bar")).toBeInTheDocument()
  })

  it("has no axe a11y violations on the populated list", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1, photos: 1 })
    )
    const { container } = renderPage()
    await screen.findByTestId("file-card-f1")
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
      ...fileHandlers.counts(SLUG, { all: 1, photos: 1 }),
      ...fileHandlers.update(SLUG, "f1", { id: "f1", category: "documents" })
    )
    renderPage()
    await screen.findByTestId("file-card-f1")
    await user.click(screen.getByTestId("file-card-checkbox-f1"))
    await screen.findByTestId("files-bulk-bar")
    const moveSelect = screen.getByTestId("files-bulk-move") as HTMLSelectElement
    await user.selectOptions(moveSelect, "documents")
    // After bulk move, selection is cleared and the bar disappears.
    await waitFor(() => expect(screen.queryByTestId("files-bulk-bar")).not.toBeInTheDocument())
  })

  it("preserves the active tag filter from the URL query", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1", { tags: ["receipt"] })]),
      ...fileHandlers.counts(SLUG, { all: 1 })
    )
    renderPage(`/g/${SLUG}/files?tags=receipt,important`)
    await screen.findByTestId("file-card-f1")
    // Both tags persist as chips in the toolbar's tag-filter input.
    const chips = screen.getAllByTestId("files-tag-filter-chip")
    const labels = chips.map((c) => c.textContent?.trim())
    expect(labels).toContain("receipt")
    expect(labels).toContain("important")
  })

  it("opens the file detail sheet when navigating to /files/:id directly", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [fileRow("f1")]),
      ...fileHandlers.counts(SLUG, { all: 1 }),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "Direct file",
        category: "photos",
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
    await screen.findByTestId("file-card-f0")
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
    await screen.findByTestId("file-card-f1")
    const input = screen.getByTestId("files-search-input") as HTMLInputElement
    await user.type(input, "invoice")
    await user.click(screen.getByRole("button", { name: /^search$/i }))
    // The toolbar pendingSearch state was committed to URL params; the
    // input retains the typed value through the rerender.
    await waitFor(() => expect(input.value).toBe("invoice"))
  })
})
