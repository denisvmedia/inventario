import { screen } from "@testing-library/react"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest"

import { RestoreLogDialog } from "@/components/exports/RestoreLogDialog"
import { GroupProvider } from "@/features/group/GroupContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { exportHandlers, groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const SLUG = "household"
const EXPORT_ID = "exp-1"
const RESTORE_ID = "rest-1"

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

function renderDialog({ dryRun = false }: { dryRun?: boolean } = {}) {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/exports`,
    routes: (
      <Route
        path="/g/:groupSlug/exports"
        element={
          <GroupProvider>
            <RestoreLogDialog
              open={true}
              onOpenChange={vi.fn()}
              exportId={EXPORT_ID}
              restoreId={RESTORE_ID}
              dryRun={dryRun}
            />
          </GroupProvider>
        }
      />
    ),
  })
}

describe("<RestoreLogDialog />", () => {
  it("renders the per-step log entries with result-specific emojis", async () => {
    server.use(
      ...exportHandlers.getRestore(
        SLUG,
        EXPORT_ID,
        exportHandlers.restoreFixture({
          id: RESTORE_ID,
          status: "completed",
          steps: [
            {
              id: "s1",
              name: "Restoring location: Main House",
              result: "success",
              restore_operation_id: RESTORE_ID,
            },
            {
              id: "s2",
              name: "Restoring commodity: Unknown",
              result: "error",
              reason: "Area not found",
              restore_operation_id: RESTORE_ID,
            },
            {
              id: "s3",
              name: "Skipping duplicate area",
              result: "skipped",
              restore_operation_id: RESTORE_ID,
            },
          ],
        })
      )
    )
    renderDialog()
    expect(await screen.findByTestId("restore-log-list")).toBeVisible()
    expect(screen.getByTestId("restore-log-step-success")).toHaveTextContent(
      "Restoring location: Main House"
    )
    const errored = screen.getByTestId("restore-log-step-error")
    expect(errored).toHaveTextContent("Area not found")
    expect(errored).toHaveClass("text-destructive")
    expect(screen.getByTestId("restore-log-step-skipped")).toHaveTextContent(
      "Skipping duplicate area"
    )
  })

  it("flips the title between preview and complete based on dryRun", async () => {
    server.use(
      ...exportHandlers.getRestore(
        SLUG,
        EXPORT_ID,
        exportHandlers.restoreFixture({ id: RESTORE_ID, steps: [] })
      )
    )
    renderDialog({ dryRun: true })
    expect(await screen.findByText("Restore preview")).toBeVisible()
  })
})
