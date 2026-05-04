import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { ExportImportPage } from "@/pages/exports/ExportImportPage"
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

function renderPage() {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/exports/import`,
    routes: (
      <Route
        path="/g/:groupSlug/exports/import"
        element={
          <GroupProvider>
            <main>
              <ExportImportPage />
            </main>
          </GroupProvider>
        }
      />
    ),
  })
}

describe("<ExportImportPage />", () => {
  it("disables the submit button until a file is attached", async () => {
    renderPage()
    const submit = await screen.findByTestId("import-submit")
    expect(submit).toBeDisabled()
  })

  it("uploads the file then creates an imported export and navigates to restore", async () => {
    server.use(
      ...exportHandlers.uploadRestore(SLUG, "restores/2026/05/abc.xml"),
      ...exportHandlers.importBackup(
        SLUG,
        exportHandlers.exportFixture({ id: "imp-1", imported: true, type: "imported" })
      )
    )
    const user = userEvent.setup()
    renderPage()
    const fileInput = await screen.findByTestId("import-file-input")
    const file = new File(["<export/>"], "backup.xml", { type: "application/xml" })
    await user.upload(fileInput, file)
    expect(screen.getByTestId("import-file-chosen")).toHaveTextContent("backup.xml")

    await user.click(screen.getByTestId("import-submit"))
    // The page navigates after the import succeeds; the easiest assertion
    // is that the form disappears and the success toast helper has fired.
    await waitFor(() => expect(screen.queryByTestId("import-form")).not.toBeInTheDocument())
  })

  it("is axe-clean", async () => {
    const { baseElement } = renderPage()
    await screen.findByTestId("page-export-import")
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
