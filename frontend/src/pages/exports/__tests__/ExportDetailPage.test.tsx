import { screen } from "@testing-library/react"
import { axe } from "jest-axe"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { ExportDetailPage } from "@/pages/exports/ExportDetailPage"
import { exportHandlers, groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const SLUG = "household"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  server.use(...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household" }]))
})

function renderPage(id: string) {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/exports/${id}`,
    routes: (
      <Route
        path="/g/:groupSlug/exports/:id"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <ExportDetailPage />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

describe("<ExportDetailPage />", () => {
  it("renders metadata, counts, and an empty restore history", async () => {
    server.use(
      ...exportHandlers.detail(
        SLUG,
        exportHandlers.exportFixture({
          id: "e1",
          description: "Weekly snapshot",
          file_count: 3,
          location_count: 2,
        })
      ),
      ...exportHandlers.listRestores(SLUG, "e1", [])
    )
    renderPage("e1")
    await screen.findByTestId("page-export-detail")
    expect(screen.getByText("Weekly snapshot")).toBeVisible()
    expect(screen.getByTestId("export-detail-counts")).toHaveTextContent("3")
    expect(await screen.findByTestId("restores-empty")).toBeVisible()
  })

  it("renders the import banner when the export was imported", async () => {
    server.use(
      ...exportHandlers.detail(
        SLUG,
        exportHandlers.exportFixture({
          id: "e1",
          imported: true,
          type: "imported",
        })
      ),
      ...exportHandlers.listRestores(SLUG, "e1", [])
    )
    renderPage("e1")
    expect(await screen.findByTestId("export-detail-imported-banner")).toBeVisible()
  })

  it("is axe-clean", async () => {
    server.use(
      ...exportHandlers.detail(SLUG, exportHandlers.exportFixture({ id: "e1" })),
      ...exportHandlers.listRestores(SLUG, "e1", [])
    )
    const { baseElement } = renderPage("e1")
    await screen.findByTestId("page-export-detail")
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
