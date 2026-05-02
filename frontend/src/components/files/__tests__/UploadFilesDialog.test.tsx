import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { UploadFilesDialog } from "@/components/files/UploadFilesDialog"
import type { UploadFilesDialogProps } from "@/components/files/UploadFilesDialog"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { apiUrl, fileHandlers, groupHandlers } from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
]

function renderDialog(props: Partial<Omit<UploadFilesDialogProps, "open" | "onOpenChange">> = {}) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/files`,
    routes: (
      <Route
        path="/g/:groupSlug/files"
        element={
          <GroupProvider>
            <UploadFilesDialog open onOpenChange={() => {}} {...props} />
          </GroupProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<UploadFilesDialog />", () => {
  it("renders the select-step dropzone and disables Next until files are queued", async () => {
    server.use(...groupHandlers.list(groupFixture))
    renderDialog()
    expect(await screen.findByTestId("files-upload-dropzone")).toBeInTheDocument()
    const next = screen.getByTestId("files-upload-next")
    expect(next).toBeDisabled()
  })

  it("queues files dropped through the hidden input and advances to the metadata step", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture))
    renderDialog()

    const input = (await screen.findByTestId("files-upload-input")) as HTMLInputElement
    const file = new File(["hello"], "doc.pdf", { type: "application/pdf" })
    await user.upload(input, file)
    await waitFor(() => expect(screen.getByTestId("files-upload-list")).toBeInTheDocument())

    const next = screen.getByTestId("files-upload-next")
    expect(next).not.toBeDisabled()
    await user.click(next)

    // Metadata step renders one row per file with editable title +
    // category. The category MIME-derives to "documents" for PDFs.
    expect(await screen.findByTestId("files-upload-metadata-list")).toBeInTheDocument()
  })

  it("activates the dropzone on Space keydown", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture))
    renderDialog()
    const dropzone = await screen.findByTestId("files-upload-dropzone")
    dropzone.focus()
    // Pressing Space while the dropzone has focus delegates to the
    // hidden file input's click — the input itself isn't visible but
    // the page would navigate the OS file picker open. We can't open
    // a file picker in jsdom, but we can assert no crash + no state
    // change.
    await user.keyboard(" ")
    expect(screen.getByTestId("files-upload-next")).toBeDisabled()
  })

  it("derives a per-file category default from the dropped MIME type", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture))
    renderDialog()
    const input = (await screen.findByTestId("files-upload-input")) as HTMLInputElement
    await user.upload(input, [
      new File(["x"], "photo.jpg", { type: "image/jpeg" }),
      new File(["x"], "doc.pdf", { type: "application/pdf" }),
      new File(["x"], "song.mp3", { type: "audio/mpeg" }),
    ])
    await user.click(screen.getByTestId("files-upload-next"))
    // Wait for the metadata step to render with one select per file.
    await waitFor(() => expect(screen.getAllByRole("combobox")).toHaveLength(3))
    const all = screen.getAllByRole("combobox") as HTMLSelectElement[]
    expect(all[0].value).toBe("photos")
    expect(all[1].value).toBe("documents")
    expect(all[2].value).toBe("other")
  })

  it("seeds initialFiles from the page-level drop catcher and enables Next immediately", async () => {
    server.use(...groupHandlers.list(groupFixture))
    const dropped = [
      new File(["x"], "from-page.png", { type: "image/png" }),
      new File(["y"], "also.pdf", { type: "application/pdf" }),
    ]
    renderDialog({ initialFiles: dropped })
    expect(await screen.findByText("from-page.png")).toBeInTheDocument()
    expect(screen.getByText("also.pdf")).toBeInTheDocument()
    expect(screen.getByTestId("files-upload-next")).not.toBeDisabled()
  })

  it("renders the linked-entity title when linkedEntity.name is provided", async () => {
    server.use(...groupHandlers.list(groupFixture))
    renderDialog({
      linkedEntity: { type: "commodity", id: "com-9", name: "Vintage camera" },
    })
    expect(
      await screen.findByRole("heading", { name: /attach files to vintage camera/i })
    ).toBeInTheDocument()
  })

  it("PUTs linked_entity_* on every successful upload when linkedEntity is set", async () => {
    const updateBodies: Array<Record<string, unknown>> = []
    let capacityRequests = 0
    server.use(
      ...groupHandlers.list(groupFixture),
      // Upload slot check returns can_start_upload=true; the dialog
      // gates startUpload behind this.
      http.get(apiUrl(`/g/${SLUG}/upload-slots/check`), () => {
        capacityRequests++
        return HttpResponse.json({
          data: {
            attributes: {
              operation_name: "files-upload",
              active_uploads: 0,
              max_uploads: 4,
              available_uploads: 4,
              can_start_upload: true,
            },
          },
        })
      }),
      ...fileHandlers.upload(SLUG, {
        title: "from-drop",
        category: "photos",
        mime_type: "image/png",
      }),
      http.put(apiUrl(`/g/${SLUG}/files/uploaded-1`), async ({ request }) => {
        const body = (await request.json()) as { data?: { attributes?: Record<string, unknown> } }
        updateBodies.push(body.data?.attributes ?? {})
        return HttpResponse.json({
          id: "uploaded-1",
          type: "files",
          attributes: body.data?.attributes ?? {},
        })
      })
    )
    const user = userEvent.setup()
    const dropped = [new File(["x"], "from-drop.png", { type: "image/png" })]
    renderDialog({
      initialFiles: dropped,
      linkedEntity: { type: "commodity", id: "com-9", name: "Camera" },
    })
    await screen.findByText("from-drop.png")
    await user.click(screen.getByTestId("files-upload-next"))
    await screen.findByTestId("files-upload-metadata-list")
    await user.click(screen.getByTestId("files-upload-start"))
    await waitFor(() => expect(screen.getByTestId("files-upload-progress")).toBeInTheDocument())
    await waitFor(() => expect(updateBodies).toHaveLength(1))
    expect(capacityRequests).toBeGreaterThanOrEqual(1)
    const attrs = updateBodies[0]
    expect(attrs.linked_entity_type).toBe("commodity")
    expect(attrs.linked_entity_id).toBe("com-9")
  })

  it("marks an item as failed when the linkage PUT fails (orphan would otherwise be silent)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      http.get(apiUrl(`/g/${SLUG}/upload-slots/check`), () =>
        HttpResponse.json({
          data: {
            attributes: {
              operation_name: "files-upload",
              active_uploads: 0,
              max_uploads: 4,
              available_uploads: 4,
              can_start_upload: true,
            },
          },
        })
      ),
      ...fileHandlers.upload(SLUG, { title: "x", category: "photos" }),
      http.put(apiUrl(`/g/${SLUG}/files/uploaded-1`), () =>
        HttpResponse.json({ error: "linkage rejected" }, { status: 422 })
      )
    )
    const user = userEvent.setup()
    renderDialog({
      initialFiles: [new File(["x"], "fail.png", { type: "image/png" })],
      linkedEntity: { type: "commodity", id: "com-9" },
    })
    await screen.findByText("fail.png")
    await user.click(screen.getByTestId("files-upload-next"))
    await user.click(screen.getByTestId("files-upload-start"))
    await waitFor(() => {
      const progressItems = screen.getAllByTestId(/files-upload-progress-item-/)
      expect(progressItems[0].getAttribute("data-status")).toBe("failed")
    })
  })

  it("removes a queued file via the per-row remove button", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture))
    renderDialog()

    const input = (await screen.findByTestId("files-upload-input")) as HTMLInputElement
    const file = new File(["x"], "a.png", { type: "image/png" })
    await user.upload(input, file)
    await screen.findByText("a.png")

    await user.click(screen.getByLabelText(/^remove a\.png$/i))
    expect(screen.queryByText("a.png")).not.toBeInTheDocument()
    expect(screen.getByTestId("files-upload-next")).toBeDisabled()
  })
})
