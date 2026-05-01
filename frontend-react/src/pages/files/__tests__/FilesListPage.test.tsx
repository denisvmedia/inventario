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
})
