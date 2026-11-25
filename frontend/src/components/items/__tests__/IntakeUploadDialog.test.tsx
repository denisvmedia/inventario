import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { IntakeUploadDialog } from "@/components/items/IntakeUploadDialog"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { DEFAULT_LIMITS } from "@/test/handlers/intake"
import type { IntakeLimits, IntakeSessionDetail } from "@/features/intake/api"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { authHandlers, groupHandlers, intakeHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function limits(patch: Partial<IntakeLimits> = {}): IntakeLimits {
  return {
    enabled: true,
    maxFilesPerBatch: DEFAULT_LIMITS.max_files_per_batch,
    maxFilesPerSource: DEFAULT_LIMITS.max_files_per_source,
    maxFileBytes: DEFAULT_LIMITS.max_file_bytes,
    maxQueuedPerGroup: DEFAULT_LIMITS.max_queued_per_group,
    queued: 0,
    ratePerHour: DEFAULT_LIMITS.rate_limit_per_hour,
    acceptedMimeTypes: DEFAULT_LIMITS.accepted_mime_types,
    ...patch,
  }
}

const jpg = (name: string) => new File(["jpeg"], name, { type: "image/jpeg" })

function renderDialog(files: File[], l: IntakeLimits = limits()) {
  const onQueued = vi.fn<(s: IntakeSessionDetail) => void>()
  renderWithProviders({
    initialPath: `/g/${SLUG}/commodities`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities"
        element={
          <AuthProvider>
            <GroupProvider>
              <IntakeUploadDialog
                open
                onOpenChange={() => {}}
                limits={l}
                initialFiles={files}
                onQueued={onQueued}
              />
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
  return { onQueued }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
})

describe("<IntakeUploadDialog />", () => {
  it("sends each file as its own purchase and leaves out files it cannot scan", async () => {
    const fake = intakeHandlers.backend(SLUG)
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const big = new File(["0123456789abcdef"], "huge.jpg", { type: "image/jpeg" })
    const { onQueued } = renderDialog(
      [
        jpg("receipt.jpg"),
        new File(["x"], "notes.txt", { type: "text/plain" }),
        big,
        jpg("tv.jpg"),
      ],
      limits({ maxFileBytes: 10 })
    )
    const dialog = await screen.findByTestId("intake-upload-dialog")
    const rows = within(dialog).getAllByTestId("intake-upload-file")
    expect(rows).toHaveLength(4)
    expect(rows[1]).toHaveTextContent("only images and PDFs")
    expect(rows[2]).toHaveTextContent("larger than")
    const submit = within(dialog).getByTestId("intake-upload-submit")
    expect(submit).toHaveTextContent("Scan 2 files")
    await user.click(submit)
    await waitFor(() => expect(onQueued).toHaveBeenCalledTimes(1))
    expect(fake.uploads).toBe(2)
    expect(fake.creates).toHaveLength(1)
    expect(fake.creates[0]?.sources).toEqual([
      { file_ids: ["file-1"], one_item: false },
      { file_ids: ["file-2"], one_item: false },
    ])
    expect(onQueued.mock.calls[0]?.[0].id).toBe(fake.sessions[0]?.id)
  })

  it("sends all files as one purchase of one item when asked", async () => {
    const fake = intakeHandlers.backend(SLUG)
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onQueued } = renderDialog([jpg("front.jpg"), jpg("label.jpg")])
    await user.click(await screen.findByTestId("intake-upload-one-item"))
    await user.click(screen.getByTestId("intake-upload-submit"))
    await waitFor(() => expect(onQueued).toHaveBeenCalledTimes(1))
    expect(fake.creates[0]?.sources).toEqual([{ file_ids: ["file-1", "file-2"], one_item: true }])
  })

  it("retries a batch whose answer was lost with the same key and no new uploads", async () => {
    const fake = intakeHandlers.backend(SLUG)
    fake.loseCreate = true
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onQueued } = renderDialog([jpg("a.jpg"), jpg("b.jpg")])
    await user.click(await screen.findByTestId("intake-upload-submit"))
    expect(await screen.findByTestId("intake-upload-error")).toBeInTheDocument()
    expect(onQueued).not.toHaveBeenCalled()
    await user.click(screen.getByTestId("intake-upload-submit"))
    await waitFor(() => expect(onQueued).toHaveBeenCalledTimes(1))
    expect(fake.uploads).toBe(2)
    expect(fake.creates).toHaveLength(2)
    expect(fake.creates[1]?.client_key).toBe(fake.creates[0]?.client_key)
    expect(fake.sessions).toHaveLength(1)
  })

  it("refuses a batch the scan queue has no room for", async () => {
    const fake = intakeHandlers.backend(SLUG)
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(groups), ...fake.handlers)
    renderDialog([jpg("a.jpg"), jpg("b.jpg")], limits({ maxQueuedPerGroup: 10, queued: 9 }))
    expect(await screen.findByTestId("intake-upload-queue-full")).toHaveTextContent(
      "room for 1 more purchase"
    )
    expect(screen.getByTestId("intake-upload-submit")).toBeDisabled()
  })
})
