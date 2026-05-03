import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
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
  return renderWithProviders({
    initialPath: `/g/${SLUG}/exports/new`,
    routes: (
      <Route
        path="/g/:groupSlug/exports/new"
        element={
          <GroupProvider>
            <main>
              <ExportNewPage />
            </main>
          </GroupProvider>
        }
      />
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

  it("walks through all three steps and submits the create call", async () => {
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
    await waitFor(() => expect(screen.getByTestId("wizard-step-3-content")).toBeVisible())
    expect(captured).toMatchObject({
      data: { type: "exports", attributes: { type: "full_database" } },
    })
    // step 3 polls /exports/:id; once status=completed surfaces the
    // download CTA.
    await waitFor(() => expect(screen.getByTestId("wizard-download")).toBeVisible())
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
