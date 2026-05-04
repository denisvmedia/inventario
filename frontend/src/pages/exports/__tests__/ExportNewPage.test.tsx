import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"
import { Outlet, Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { ExportNewPage } from "@/pages/exports/ExportNewPage"
import { exportHandlers, groupHandlers } from "@/test/handlers"
import { apiUrl } from "@/test/handlers"
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

function renderPage() {
  // Wrap in a parent <Route> that mounts <GroupProvider> once so both
  // /exports/new and the detail page (the wizard's success target) share
  // the same provider — otherwise navigate() to the detail route would
  // unmount and remount the provider, invalidating the URL slug.
  return renderWithProviders({
    initialPath: `/g/${SLUG}/exports/new`,
    routes: (
      <Route
        path="/g/:groupSlug"
        element={
          <GroupProvider>
            <main>
              <Outlet />
            </main>
          </GroupProvider>
        }
      >
        <Route path="exports/new" element={<ExportNewPage />} />
        <Route
          path="exports/:id"
          element={<div data-testid="stub-export-detail">stub-detail</div>}
        />
      </Route>
    ),
  })
}

describe("<ExportNewPage />", () => {
  it("renders step 1 with the scope radios", async () => {
    renderPage()
    expect(await screen.findByTestId("wizard-step-1-content")).toBeVisible()
    expect(screen.getByTestId("wizard-scope-full_database")).toBeVisible()
    expect(screen.getByTestId("wizard-scope-selected_items")).toBeVisible()
  })

  it("walks through both steps, submits, and navigates to the detail page", async () => {
    let captured: unknown = null
    server.use(
      http.post(apiUrl(`/g/${SLUG}/exports`), async ({ request }) => {
        captured = await request.json()
        return HttpResponse.json(
          {
            data: {
              id: "e1",
              type: "exports",
              attributes: {
                type: "full_database",
                status: "completed",
                file_size: 0,
                description: "",
                include_file_data: true,
                created_date: "2026-05-01T10:00:00Z",
                completed_date: "2026-05-01T10:00:30Z",
              },
            },
          },
          { status: 201 }
        )
      }),
      ...exportHandlers.detail(
        SLUG,
        exportHandlers.exportFixture({ id: "e1", status: "completed" })
      )
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("wizard-step-1-content")

    await user.click(screen.getByTestId("wizard-next"))
    expect(await screen.findByTestId("wizard-step-2-content")).toBeVisible()

    await user.click(screen.getByTestId("wizard-submit"))
    // The wizard navigates to the detail page on success — that's the
    // canonical "watch this export" surface (status badge polling,
    // download/restore CTAs, restore history).
    await waitFor(() => expect(screen.getByTestId("stub-export-detail")).toBeVisible())
    expect(captured).toMatchObject({
      data: { type: "exports", attributes: { type: "full_database" } },
    })
  })

  it("blocks the next button when selected_items is picked but empty", async () => {
    server.use(
      http.get(apiUrl(`/g/${SLUG}/locations`), () =>
        HttpResponse.json({ data: [], meta: { locations: 0 } })
      )
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("wizard-step-1-content")
    await user.click(screen.getByTestId("wizard-scope-selected_items"))
    await user.click(screen.getByTestId("wizard-next"))
    expect(await screen.findByTestId("selected-items-picker-error")).toBeVisible()
  })

  it("is axe-clean on step 1", async () => {
    const { baseElement } = renderPage()
    await screen.findByTestId("wizard-step-1-content")
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
