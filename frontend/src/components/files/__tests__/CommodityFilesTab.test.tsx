import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
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
  // Lets a test inspect the URL the tab navigates to when a card / row
  // is clicked. The location sentinel renders inside a sibling route
  // so it stays mounted alongside the tab.
  withLocationSpy?: boolean
}

function renderTab(opts: RenderOptions = {}) {
  setAccessToken("good-token")
  const onAttachClick = opts.onAttachClick ?? vi.fn()
  function LocationSpy() {
    const location = useLocation()
    return <div data-testid="location-spy">{location.pathname}</div>
  }
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/${COMMODITY}`,
    routes: (
      <>
        <Route
          path="/g/:groupSlug/commodities/:id"
          element={
            <GroupProvider>
              <ConfirmProvider>
                <CommodityFilesTab
                  commodityId={COMMODITY}
                  onAttachClick={onAttachClick}
                />
              </ConfirmProvider>
            </GroupProvider>
          }
        />
        {opts.withLocationSpy ? (
          <Route path="/g/:groupSlug/files/:id" element={<LocationSpy />} />
        ) : null}
      </>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

const photoFixture = {
  id: "f-photo",
  title: "Front view",
  path: "front",
  ext: ".jpg",
  mime_type: "image/jpeg",
  category: "photos",
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
  category: "invoices",
  type: "document",
  linked_entity_type: "commodity",
  linked_entity_id: COMMODITY,
  size_bytes: 1024 * 64,
  tags: ["tax"],
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
    // unconditionally, but the count > 0 branch is what proves the
    // files query resolved against the seeded fixture (and therefore
    // GroupContext + slug rewrite have settled).
    await screen.findByTestId("commodity-files-chip-all-count")
    // Total count is 3, photos = 1, invoices = 1, documents = 1.
    expect(screen.getByTestId("commodity-files-chip-all-count")).toHaveTextContent("3")
    expect(screen.getByTestId("commodity-files-chip-photos-count")).toHaveTextContent("1")
    expect(screen.getByTestId("commodity-files-chip-invoices-count")).toHaveTextContent("1")
    expect(screen.getByTestId("commodity-files-chip-documents-count")).toHaveTextContent("1")
    // Default chip = All — all three rows are visible.
    expect(screen.getByTestId(`commodity-files-photo-${photoFixture.id}`)).toBeInTheDocument()
    expect(screen.getByTestId(`commodity-files-row-${invoiceFixture.id}`)).toBeInTheDocument()
    expect(screen.getByTestId(`commodity-files-row-${documentFixture.id}`)).toBeInTheDocument()
    // Switch to Invoices — only the invoice row stays.
    await user.click(screen.getByTestId("commodity-files-chip-invoices"))
    expect(screen.getByTestId(`commodity-files-row-${invoiceFixture.id}`)).toBeInTheDocument()
    expect(screen.queryByTestId(`commodity-files-row-${documentFixture.id}`)).toBeNull()
    // Photo grid is hidden in non-photo / non-all chips.
    expect(screen.queryByTestId("commodity-files-photo-grid")).toBeNull()
  })

  it("switches the upload-zone copy by active chip", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture), ...fileHandlers.list(SLUG, []))
    renderTab()
    const zone = await screen.findByTestId("commodity-files-upload-zone")
    expect(zone).toHaveTextContent(/Drop files or/i)
    await user.click(screen.getByTestId("commodity-files-chip-photos"))
    expect(screen.getByTestId("commodity-files-upload-zone")).toHaveTextContent(/Drop photos or/i)
    await user.click(screen.getByTestId("commodity-files-chip-invoices"))
    expect(screen.getByTestId("commodity-files-upload-zone")).toHaveTextContent(
      /Drop invoices or/i
    )
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

  it("renders the chip-aware empty state when no files match the active chip", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [{ id: invoiceFixture.id, attributes: invoiceFixture }])
    )
    renderTab()
    // Default = All chip; the invoice is visible.
    await screen.findByTestId(`commodity-files-row-${invoiceFixture.id}`)
    // Switching to Photos shows the empty-state copy specific to the
    // chip — "No photos yet…" rather than the generic copy.
    await user.click(screen.getByTestId("commodity-files-chip-photos"))
    const empty = await screen.findByTestId("commodity-files-empty")
    expect(empty).toHaveTextContent(/No photos yet/i)
  })

  it("opens the FileDetailSheet route when a non-photo row is activated", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [{ id: invoiceFixture.id, attributes: invoiceFixture }])
    )
    renderTab({ withLocationSpy: true })
    const openCta = await screen.findByTestId(
      `commodity-files-row-open-${invoiceFixture.id}`
    )
    // Invoices are PDFs — CTA copy = "Open".
    expect(openCta).toHaveTextContent(/Open/i)
    await user.click(openCta)
    await waitFor(() =>
      expect(screen.getByTestId("location-spy")).toHaveTextContent(
        `/g/${SLUG}/files/${invoiceFixture.id}`
      )
    )
  })

  it("uses the View CTA for image rows when listed under All", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [
        // A non-categorised image (BE backfill edge case) so it lands
        // in the non-photo list under All; CTA copy must be "View".
        {
          id: "f-other-image",
          attributes: {
            ...photoFixture,
            id: "f-other-image",
            category: "other",
            linked_entity_id: COMMODITY,
          },
        },
      ])
    )
    renderTab()
    const cta = await screen.findByTestId("commodity-files-row-open-f-other-image")
    expect(cta).toHaveTextContent(/View/i)
  })

  it("renders the photo grid for image-category rows", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [{ id: photoFixture.id, attributes: photoFixture }])
    )
    renderTab()
    expect(
      await screen.findByTestId(`commodity-files-photo-${photoFixture.id}`)
    ).toBeInTheDocument()
    expect(screen.getByTestId("commodity-files-photo-grid")).toBeInTheDocument()
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
    await screen.findByTestId(`commodity-files-photo-${photoFixture.id}`)
    expect(await axe(container)).toHaveNoViolations()
  })
})
