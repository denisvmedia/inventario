import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { EntityFilesPanel } from "@/components/files/EntityFilesPanel"
import { GroupProvider } from "@/features/group/GroupContext"
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
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
]

function renderPanel(
  linkedEntityType: "commodity" | "location" = "commodity",
  options: { onAttachClick?: () => void } = {}
) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/${COMMODITY}`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <GroupProvider>
            <EntityFilesPanel
              linkedEntityType={linkedEntityType}
              linkedEntityId={COMMODITY}
              onAttachClick={options.onAttachClick}
            />
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
})

describe("<EntityFilesPanel />", () => {
  it("renders the empty state when no files are linked", async () => {
    server.use(...groupHandlers.list(groupFixture), ...fileHandlers.list(SLUG, []))
    renderPanel()
    await waitFor(() => expect(screen.getByTestId("entity-files-panel-empty")).toBeInTheDocument())
    expect(screen.queryByTestId("entity-files-panel-grid")).not.toBeInTheDocument()
  })

  it("renders a grid with one card per file", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [
        {
          id: "f-1",
          title: "Receipt",
          path: "receipt-1",
          ext: ".pdf",
          mime_type: "application/pdf",
          category: "invoices",
          type: "document",
          linked_entity_type: "commodity",
          linked_entity_id: COMMODITY,
          tags: ["tax"],
          created_at: "2026-04-01T10:00:00Z",
        },
        {
          id: "f-2",
          title: "Photo",
          path: "photo-1",
          ext: ".jpg",
          mime_type: "image/jpeg",
          category: "photos",
          type: "image",
          linked_entity_type: "commodity",
          linked_entity_id: COMMODITY,
          tags: [],
          created_at: "2026-04-02T10:00:00Z",
        },
      ])
    )
    renderPanel()
    await waitFor(() => expect(screen.getByTestId("entity-files-panel-grid")).toBeInTheDocument())
    expect(screen.getByTestId("file-card-f-1")).toBeInTheDocument()
    expect(screen.getByTestId("file-card-f-2")).toBeInTheDocument()
    // Read-only: no checkbox should be rendered when onToggleSelect is omitted.
    expect(screen.queryByTestId("file-card-checkbox-f-1")).not.toBeInTheDocument()
  })

  it("surfaces an error alert when the list endpoint 500s", async () => {
    server.use(...groupHandlers.list(groupFixture), ...fileHandlers.error(SLUG, 500))
    renderPanel()
    await waitFor(() => expect(screen.getByTestId("entity-files-panel-error")).toBeInTheDocument())
  })

  it("hides the Attach files button when onAttachClick is not provided", async () => {
    server.use(...groupHandlers.list(groupFixture), ...fileHandlers.list(SLUG, []))
    renderPanel()
    await waitFor(() => expect(screen.getByTestId("entity-files-panel-empty")).toBeInTheDocument())
    expect(screen.queryByTestId("entity-files-panel-attach")).not.toBeInTheDocument()
  })

  it("renders the Attach files button and fires onAttachClick when clicked", async () => {
    const user = userEvent.setup()
    const onAttachClick = vi.fn()
    server.use(...groupHandlers.list(groupFixture), ...fileHandlers.list(SLUG, []))
    renderPanel("commodity", { onAttachClick })
    const btn = await screen.findByTestId("entity-files-panel-attach")
    await user.click(btn)
    expect(onAttachClick).toHaveBeenCalledTimes(1)
  })

  it("is axe-clean in the populated state", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, [
        {
          id: "f-1",
          title: "Receipt",
          path: "receipt-1",
          ext: ".pdf",
          mime_type: "application/pdf",
          category: "invoices",
          type: "document",
          linked_entity_type: "commodity",
          linked_entity_id: COMMODITY,
          tags: [],
          created_at: "2026-04-01T10:00:00Z",
        },
      ])
    )
    const { container } = renderPanel()
    await waitFor(() => expect(screen.getByTestId("entity-files-panel-grid")).toBeInTheDocument())
    const results = await axe(container)
    expect(results.violations).toEqual([])
  })
})
