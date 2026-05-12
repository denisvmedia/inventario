import { screen, waitFor, within } from "@testing-library/react"
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
import { ExportRestorePage } from "@/pages/exports/ExportRestorePage"
import { exportHandlers, groupHandlers } from "@/test/handlers"
import { apiUrl } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const SLUG = "household"
const EXPORT_ID = "e1"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  server.use(
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household" }]),
    ...exportHandlers.detail(SLUG, exportHandlers.exportFixture({ id: EXPORT_ID }))
  )
})

function renderPage() {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/exports/${EXPORT_ID}/restore`,
    routes: (
      <Route
        path="/g/:groupSlug/exports/:id/restore"
        element={
          <GroupProvider>
            <main>
              <ExportRestorePage />
            </main>
          </GroupProvider>
        }
      />
    ),
  })
}

describe("<ExportRestorePage />", () => {
  it("renders the strategy radios with merge_add as the default", async () => {
    renderPage()
    expect(await screen.findByTestId("restore-strategy-merge_add")).toBeVisible()
    // Radix RadioGroup renders a button[role=radio]; check the checked
    // state via aria-checked rather than the legacy <input> form control.
    const merge = within(screen.getByTestId("restore-strategy-merge_add")).getByRole("radio")
    expect(merge).toHaveAttribute("aria-checked", "true")
  })

  it("shows risk pills next to each strategy", async () => {
    renderPage()
    const merge = await screen.findByTestId("restore-strategy-merge_add")
    expect(merge).toHaveTextContent("Safe")
    expect(screen.getByTestId("restore-strategy-merge_update")).toHaveTextContent("Moderate risk")
    expect(screen.getByTestId("restore-strategy-full_replace")).toHaveTextContent("Destructive")
  })

  it("warns when full_replace is picked WITHOUT dry-run", async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("restore-form")
    await user.click(screen.getByTestId("restore-strategy-full_replace"))
    // dry-run defaults to true; warning is suppressed
    expect(screen.queryByTestId("restore-destructive-warning")).not.toBeInTheDocument()
    await user.click(screen.getByTestId("restore-dry-run"))
    expect(screen.getByTestId("restore-destructive-warning")).toBeVisible()
  })

  it("submits the chosen options to the BE", async () => {
    let captured: unknown = null
    server.use(
      http.post(apiUrl(`/g/${SLUG}/exports/${EXPORT_ID}/restores`), async ({ request }) => {
        captured = await request.json()
        return HttpResponse.json(
          {
            data: {
              id: "r1",
              type: "restores",
              attributes: { status: "pending", description: "" },
            },
          },
          { status: 201 }
        )
      })
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("restore-form")
    // Description is required (BE: jsonapi/restore_operations.go enforces
    // Required + Length 1..500). Fill it so the submit button enables.
    await user.type(screen.getByTestId("restore-description"), "smoke restore")
    await user.click(screen.getByTestId("restore-submit"))
    await waitFor(() =>
      expect(captured).toMatchObject({
        data: {
          type: "restores",
          attributes: {
            description: "smoke restore",
            options: { strategy: "merge_add", include_file_data: true, dry_run: true },
          },
        },
      })
    )
  })

  it("is axe-clean", async () => {
    const { baseElement } = renderPage()
    await screen.findByTestId("restore-form")
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
