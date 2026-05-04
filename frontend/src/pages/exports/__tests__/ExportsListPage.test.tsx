import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { ExportsListPage } from "@/pages/exports/ExportsListPage"
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
})

function renderPage(initialPath = `/g/${SLUG}/exports`) {
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/exports"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <ExportsListPage />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

function seed(items = [exportHandlers.exportFixture()]) {
  server.use(
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household" }]),
    ...exportHandlers.list(SLUG, items)
  )
}

describe("<ExportsListPage />", () => {
  it("renders the empty state when there are no exports", async () => {
    seed([])
    renderPage()
    expect(await screen.findByTestId("exports-list-empty")).toBeVisible()
  })

  it("renders one row per export with status badge and counts", async () => {
    seed([
      exportHandlers.exportFixture({ id: "e1", description: "Weekly snapshot" }),
      exportHandlers.exportFixture({ id: "e2", status: "in_progress" }),
    ])
    renderPage()
    expect(await screen.findByTestId("export-row-e1")).toBeVisible()
    expect(screen.getByTestId("export-row-e1")).toHaveTextContent("Weekly snapshot")
    // Live indicator surfaces when any export is non-terminal.
    expect(screen.getByTestId("exports-live-indicator")).toBeVisible()
    expect(screen.getByTestId("status-completed")).toBeVisible()
    expect(screen.getByTestId("status-in_progress")).toBeVisible()
  })

  it("respects the show-deleted toggle by passing include_deleted=true", async () => {
    let captured: URL | null = null
    server.use(
      ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household" }]),
      http.get(`${window.location.origin}/api/v1/g/${SLUG}/exports`, ({ request }) => {
        captured = new URL(request.url)
        return HttpResponse.json({ data: [], meta: { exports: 0 } })
      })
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("exports-list-empty")
    await user.click(screen.getByTestId("exports-show-deleted"))
    await waitFor(() => expect(captured!.searchParams.get("include_deleted")).toBe("true"))
  })

  it("is axe-clean once data has loaded", async () => {
    seed([exportHandlers.exportFixture({ id: "e1" })])
    const { baseElement } = renderPage()
    await screen.findByTestId("export-row-e1")
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
