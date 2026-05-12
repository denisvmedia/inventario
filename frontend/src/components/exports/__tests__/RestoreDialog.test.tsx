import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest"

import { RestoreDialog } from "@/components/exports/RestoreDialog"
import { type Export } from "@/features/export/api"
import { GroupProvider } from "@/features/group/GroupContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, exportHandlers, groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const SLUG = "household"
const EXPORT_ID = "exp-1"

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

function exportRecord(): Export {
  const fix = exportHandlers.exportFixture({
    id: EXPORT_ID,
    type: "full_database",
    description: "Weekly snapshot",
  })
  return { ...fix, id: EXPORT_ID } as Export
}

function renderDialog(opts: { onCompleted?: (id: string, dryRun: boolean) => void } = {}) {
  const onOpenChange = vi.fn()
  return {
    onOpenChange,
    onCompleted: opts.onCompleted,
    ...renderWithProviders({
      initialPath: `/g/${SLUG}/exports`,
      routes: (
        <Route
          path="/g/:groupSlug/exports"
          element={
            <GroupProvider>
              <RestoreDialog
                open={true}
                onOpenChange={onOpenChange}
                export={exportRecord()}
                onCompleted={opts.onCompleted}
              />
            </GroupProvider>
          }
        />
      ),
    }),
  }
}

describe("<RestoreDialog />", () => {
  it("renders the form with risk badges and the scope/date header", async () => {
    renderDialog()
    expect(await screen.findByTestId("restore-dialog")).toBeVisible()
    expect(screen.getByTestId("restore-strategy-merge_add")).toHaveTextContent("Safe")
    expect(screen.getByTestId("restore-strategy-merge_update")).toHaveTextContent("Moderate risk")
    expect(screen.getByTestId("restore-strategy-full_replace")).toHaveTextContent("Destructive")
  })

  it("submits the chosen options to the BE and fires onCompleted", async () => {
    let captured: unknown = null
    server.use(
      http.post(apiUrl(`/g/${SLUG}/exports/${EXPORT_ID}/restores`), async ({ request }) => {
        captured = await request.json()
        return HttpResponse.json(
          {
            data: {
              id: "rest-99",
              type: "restores",
              attributes: { status: "pending", description: "smoke" },
            },
          },
          { status: 201 }
        )
      })
    )
    const onCompleted = vi.fn()
    const user = userEvent.setup()
    renderDialog({ onCompleted })
    await screen.findByTestId("restore-dialog")
    await user.type(screen.getByTestId("restore-description"), "smoke")
    await user.click(screen.getByTestId("restore-dialog-submit"))
    await waitFor(() =>
      expect(captured).toMatchObject({
        data: {
          type: "restores",
          attributes: {
            description: "smoke",
            options: { strategy: "merge_add", dry_run: true, include_file_data: true },
          },
        },
      })
    )
    await waitFor(() => expect(onCompleted).toHaveBeenCalledWith("rest-99", true))
  })

  it("flips submit copy based on the dry-run switch", async () => {
    const user = userEvent.setup()
    renderDialog()
    const submit = await screen.findByTestId("restore-dialog-submit")
    expect(submit).toHaveTextContent("Preview restore")
    await user.click(screen.getByTestId("restore-dry-run"))
    expect(submit).toHaveTextContent("Restore now")
  })
})
