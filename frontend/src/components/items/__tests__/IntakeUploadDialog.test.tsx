import { useState } from "react"
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
        // Safari reports an iPhone photo as HEIC; a browser that reports
        // nothing is judged by the extension.
        new File(["heic"], "IMG_0001.HEIC", { type: "image/heic" }),
        new File(["heic"], "IMG_0002.heif", { type: "" }),
      ],
      limits({ maxFileBytes: 10 })
    )
    const dialog = await screen.findByTestId("intake-upload-dialog")
    const rows = within(dialog).getAllByTestId("intake-upload-file")
    expect(rows).toHaveLength(6)
    expect(rows[1]).toHaveTextContent("the scan reads PDF, JPEG, PNG, and WebP files only")
    expect(rows[2]).toHaveTextContent("larger than")
    expect(rows[4]).toHaveTextContent("HEIC photos cannot be scanned")
    expect(rows[5]).toHaveTextContent("HEIC photos cannot be scanned")
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

  it("tells a screen reader how far the upload is", async () => {
    const fake = intakeHandlers.backend(SLUG)
    fake.uploadDelayMs = 50
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onQueued } = renderDialog([jpg("a.jpg"), jpg("b.jpg")])
    const progress = await screen.findByTestId("intake-upload-progress")
    expect(progress).toHaveAttribute("role", "status")
    expect(progress).toBeEmptyDOMElement()
    await user.click(screen.getByTestId("intake-upload-submit"))
    expect(await screen.findByText("Uploading 0 of 2…", { selector: "[role=status]" })).toBe(
      progress
    )
    await waitFor(() => expect(progress).toHaveTextContent("Uploading 1 of 2…"))
    expect(screen.getAllByTestId("intake-upload-file")[0]).toHaveTextContent("a.jpg uploaded")
    await waitFor(() => expect(onQueued).toHaveBeenCalledTimes(1))
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

// The dialog uploads the files before it queues the batch. Files it uploaded
// that no batch holds once it closes go to Trash (#1626).
describe("<IntakeUploadDialog /> uploads left behind", () => {
  function Closable({ files, l = limits() }: { files: File[]; l?: IntakeLimits }) {
    const [open, setOpen] = useState(true)
    return (
      <IntakeUploadDialog
        open={open}
        onOpenChange={setOpen}
        limits={l}
        initialFiles={files}
        onQueued={() => setOpen(false)}
      />
    )
  }

  function renderClosable(files: File[], l?: IntakeLimits) {
    renderWithProviders({
      initialPath: `/g/${SLUG}/commodities`,
      routes: (
        <Route
          path="/g/:groupSlug/commodities"
          element={
            <AuthProvider>
              <GroupProvider>
                <Closable files={files} l={l} />
              </GroupProvider>
            </AuthProvider>
          }
        />
      ),
    })
  }

  it("moves the uploads to Trash when the dialog is closed after the batch failed", async () => {
    const fake = intakeHandlers.backend(SLUG)
    fake.failCreate = 500
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    renderClosable([jpg("a.jpg"), jpg("b.jpg")])
    await user.click(await screen.findByTestId("intake-upload-submit"))
    expect(await screen.findByTestId("intake-upload-error")).toBeInTheDocument()
    expect(fake.uploads).toBe(2)
    await user.click(screen.getByRole("button", { name: "Cancel" }))
    await waitFor(() => expect(fake.trashed).toEqual(["file-1", "file-2"]))
    expect(screen.queryByTestId("intake-upload-dialog")).not.toBeInTheDocument()
  })

  it("keeps the files of a queued batch", async () => {
    const fake = intakeHandlers.backend(SLUG)
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    renderClosable([jpg("a.jpg"), jpg("b.jpg")])
    await user.click(await screen.findByTestId("intake-upload-submit"))
    await waitFor(() =>
      expect(screen.queryByTestId("intake-upload-dialog")).not.toBeInTheDocument()
    )
    expect(fake.sessions).toHaveLength(1)
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(fake.trashed).toEqual([])
  })

  it("keeps the files of a batch queued although its answer was lost", async () => {
    const fake = intakeHandlers.backend(SLUG)
    fake.loseCreate = true
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    renderClosable([jpg("a.jpg")])
    await user.click(await screen.findByTestId("intake-upload-submit"))
    expect(await screen.findByTestId("intake-upload-error")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Cancel" }))
    await waitFor(() =>
      expect(screen.queryByTestId("intake-upload-dialog")).not.toBeInTheDocument()
    )
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(fake.trashed).toEqual([])
  })

  it("moves an upload the batch left out to Trash", async () => {
    const fake = intakeHandlers.backend(SLUG)
    fake.failCreate = 500
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    // One item takes one file here, so the switch leaves b.jpg out.
    renderClosable([jpg("a.jpg"), jpg("b.jpg")], limits({ maxFilesPerSource: 1 }))
    await user.click(await screen.findByTestId("intake-upload-submit"))
    expect(await screen.findByTestId("intake-upload-error")).toBeInTheDocument()
    fake.failCreate = null
    await user.click(screen.getByTestId("intake-upload-one-item"))
    await user.click(screen.getByTestId("intake-upload-submit"))
    await waitFor(() => expect(fake.sessions).toHaveLength(1))
    expect(fake.sessions[0]?.sources).toEqual([
      expect.objectContaining({ file_ids: ["file-1"], one_item: true }),
    ])
    await waitFor(() => expect(fake.trashed).toEqual(["file-2"]))
  })
})
