import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CommodityFilesTab } from "@/components/files/CommodityFilesTab"
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
const COMMODITY = "com-1"

const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

interface RenderOptions {
  onAttachClick?: () => void
}

function renderTab(opts: RenderOptions = {}) {
  setAccessToken("good-token")
  const onAttachClick = opts.onAttachClick ?? vi.fn()
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/${COMMODITY}`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <GroupProvider>
            <ConfirmProvider>
              <CommodityFilesTab commodityId={COMMODITY} onAttachClick={onAttachClick} />
            </ConfirmProvider>
          </GroupProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  // The grid/list toggle persists to localStorage (`files:entityViewMode`);
  // clear it so each test starts from the default (grid) regardless of
  // what a previous test toggled.
  window.localStorage.clear()
})

const photoFixture = {
  id: "f-photo",
  title: "Front view",
  path: "front",
  ext: ".jpg",
  mime_type: "image/jpeg",
  category: "images",
  type: "image",
  linked_entity_type: "commodity",
  linked_entity_id: COMMODITY,
  size_bytes: 1024 * 200,
  tags: [],
  created_at: "2026-04-01T10:00:00Z",
}

const invoiceFixture = {
  id: "f-invoice",
  title: "Receipt",
  path: "receipt",
  ext: ".pdf",
  mime_type: "application/pdf",
  // #1622: invoice files live in `documents` and carry the conventional
  // `invoice` tag — the Invoices chip filters by that tag, not by category.
  category: "documents",
  type: "document",
  linked_entity_type: "commodity",
  linked_entity_id: COMMODITY,
  size_bytes: 1024 * 64,
  tags: ["tax", "invoice"],
  created_at: "2026-04-02T10:00:00Z",
}

const documentFixture = {
  id: "f-document",
  title: "Manual",
  path: "manual",
  ext: ".pdf",
  mime_type: "application/pdf",
  category: "documents",
  type: "document",
  linked_entity_type: "commodity",
  linked_entity_id: COMMODITY,
  size_bytes: 1024 * 1024,
  tags: ["warranty"],
  created_at: "2026-04-03T10:00:00Z",
}

describe("<CommodityFilesTab />", () => {
  it("renders the chip-bar with derived counts and filters by chip", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [
        { id: photoFixture.id, attributes: photoFixture },
        { id: invoiceFixture.id, attributes: invoiceFixture },
        { id: documentFixture.id, attributes: documentFixture },
      ])
    )
    renderTab()
    // Wait for the count badge to populate — the chip-bar renders
    // unconditionally, but the count > 0 branch is what proves the files
    // query resolved against the seeded fixture (and therefore GroupContext
    // + slug rewrite have settled).
    await screen.findByTestId("commodity-files-chip-all-count")
    // Total = 3. Images = 1 (photo). Documents = 2 — the invoice file now
    // lives in `documents` (post-#1622) plus the manual. The Invoices chip
    // counts by tag, so it picks up only the invoice-tagged row; documents
    // and invoices intentionally overlap.
    expect(screen.getByTestId("commodity-files-chip-all-count")).toHaveTextContent("3")
    expect(screen.getByTestId("commodity-files-chip-images-count")).toHaveTextContent("1")
    expect(screen.getByTestId("commodity-files-chip-invoices-count")).toHaveTextContent("1")
    expect(screen.getByTestId("commodity-files-chip-documents-count")).toHaveTextContent("2")
    // Default chip = All, default view = grid — every file renders uniformly
    // as a FileCard (no more photo-grid / non-photo-list split, #1966).
    expect(screen.getByTestId(`file-card-${photoFixture.id}`)).toBeInTheDocument()
    expect(screen.getByTestId(`file-card-${invoiceFixture.id}`)).toBeInTheDocument()
    expect(screen.getByTestId(`file-card-${documentFixture.id}`)).toBeInTheDocument()
    // Switch to Invoices — only the invoice-tagged card stays.
    await user.click(screen.getByTestId("commodity-files-chip-invoices"))
    expect(screen.getByTestId(`file-card-${invoiceFixture.id}`)).toBeInTheDocument()
    expect(screen.queryByTestId(`file-card-${documentFixture.id}`)).toBeNull()
    expect(screen.queryByTestId(`file-card-${photoFixture.id}`)).toBeNull()
  })

  it("switches the upload-zone copy by active chip", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture), ...fileHandlers.list(SLUG, []))
    renderTab()
    const zone = await screen.findByTestId("commodity-files-upload-zone")
    expect(zone).toHaveTextContent(/Drop files or/i)
    await user.click(screen.getByTestId("commodity-files-chip-images"))
    expect(screen.getByTestId("commodity-files-upload-zone")).toHaveTextContent(/Drop images or/i)
    await user.click(screen.getByTestId("commodity-files-chip-invoices"))
    expect(screen.getByTestId("commodity-files-upload-zone")).toHaveTextContent(/Drop invoices or/i)
    await user.click(screen.getByTestId("commodity-files-chip-documents"))
    expect(screen.getByTestId("commodity-files-upload-zone")).toHaveTextContent(
      /Drop documents or/i
    )
  })

  it("fires onAttachClick when the upload zone is activated", async () => {
    const user = userEvent.setup()
    const onAttachClick = vi.fn()
    server.use(...groupHandlers.list(groupFixture), ...fileHandlers.list(SLUG, []))
    renderTab({ onAttachClick })
    const zone = await screen.findByTestId("commodity-files-upload-zone")
    await user.click(zone)
    expect(onAttachClick).toHaveBeenCalledTimes(1)
  })

  it("renders files as a FileCard grid by default", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [{ id: photoFixture.id, attributes: photoFixture }])
    )
    renderTab()
    expect(await screen.findByTestId(`file-card-${photoFixture.id}`)).toBeInTheDocument()
    expect(screen.getByTestId("commodity-files-grid")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-files-list")).toBeNull()
  })

  it("toggles between grid and list view", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [{ id: documentFixture.id, attributes: documentFixture }])
    )
    renderTab()
    // Default = grid → FileCard.
    expect(await screen.findByTestId(`file-card-${documentFixture.id}`)).toBeInTheDocument()
    // Switch to list → FileListRow rows, consistent with the main Files page.
    await user.click(screen.getByTestId("commodity-files-view-list"))
    expect(await screen.findByTestId("commodity-files-list")).toBeInTheDocument()
    expect(screen.getByTestId(`file-row-${documentFixture.id}`)).toBeInTheDocument()
    expect(screen.queryByTestId(`file-card-${documentFixture.id}`)).toBeNull()
    // Switch back to grid.
    await user.click(screen.getByTestId("commodity-files-view-grid"))
    expect(await screen.findByTestId(`file-card-${documentFixture.id}`)).toBeInTheDocument()
  })

  it("renders the chip-aware empty state when no files match the active chip", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [{ id: invoiceFixture.id, attributes: invoiceFixture }])
    )
    renderTab()
    // Default = All chip; the invoice card is visible.
    await screen.findByTestId(`file-card-${invoiceFixture.id}`)
    // Switching to Photos shows the empty-state copy specific to the chip —
    // "No images yet…" rather than the generic copy.
    await user.click(screen.getByTestId("commodity-files-chip-images"))
    const empty = await screen.findByTestId("commodity-files-empty")
    expect(empty).toHaveTextContent(/No images yet/i)
  })

  it("opens the right-side FileDetailSheet in place (not fullscreen) when a file is activated (#1966)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [{ id: photoFixture.id, attributes: photoFixture }], {
        signed_urls: { [photoFixture.id]: { url: "https://files.example.com/photo/raw" } },
      }),
      // The side panel fetches the file detail when opened.
      ...fileHandlers.detail(
        SLUG,
        photoFixture.id,
        { id: photoFixture.id, ...photoFixture },
        { url: "https://files.example.com/photo/raw" }
      )
    )
    renderTab()
    const openBtn = await screen.findByTestId(`file-card-open-${photoFixture.id}`)
    await user.click(openBtn)
    // Clicking a file opens the shared right-side FileDetailSheet *in place*
    // (the user stays on the commodity detail page) — consistent with the
    // Files page and the location/area panel, and NOT the fullscreen
    // image/PDF viewer (#1966).
    expect(await screen.findByTestId("file-detail-sheet")).toBeInTheDocument()
    expect(screen.queryByTestId("file-image-viewer")).toBeNull()
    expect(screen.queryByTestId("file-preview-dialog-pdf")).toBeNull()
  })

  it("surfaces an error alert when the list endpoint 500s", async () => {
    server.use(...groupHandlers.list(groupFixture), ...fileHandlers.error(SLUG, 500))
    renderTab()
    expect(await screen.findByTestId("commodity-files-error")).toBeInTheDocument()
  })

  it("is axe-clean in the populated state", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [
        { id: photoFixture.id, attributes: photoFixture },
        { id: invoiceFixture.id, attributes: invoiceFixture },
      ])
    )
    const { container } = renderTab()
    await screen.findByTestId(`file-card-${photoFixture.id}`)
    expect(await axe(container)).toHaveNoViolations()
  })
})
