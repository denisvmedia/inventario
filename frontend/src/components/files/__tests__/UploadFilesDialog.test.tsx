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
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
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
    // Wait for the metadata step to render with one select per file. Narrowed
    // to the <select> elements: the per-file tag input is a combobox too now
    // that it implements the ARIA pattern (#1630), so the bare role query
    // would match six.
    const categorySelects = () =>
      screen.getAllByRole("combobox").filter((el) => el.tagName === "SELECT") as HTMLSelectElement[]
    await waitFor(() => expect(categorySelects()).toHaveLength(3))
    const all = categorySelects()
    expect(all[0].value).toBe("images")
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

  it("attaches every successful upload to the linked entity", async () => {
    const attachBodies: Array<Record<string, unknown>> = []
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
        category: "images",
        mime_type: "image/png",
      }),
      ...fileHandlers.attach(SLUG, "uploaded-1", { onBody: (attrs) => attachBodies.push(attrs) }),
      ...fileHandlers.duplicates(SLUG, "uploaded-1")
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
    await waitFor(() => expect(attachBodies).toHaveLength(1))
    expect(capacityRequests).toBeGreaterThanOrEqual(1)
    // A photo goes to the item's images bucket.
    expect(attachBodies[0]).toEqual({
      entity_type: "commodity",
      entity_id: "com-9",
      meta: "images",
    })
    await waitFor(() =>
      expect(screen.getAllByTestId(/files-upload-progress-item-/)[0]).toHaveAttribute(
        "data-status",
        "done"
      )
    )
    expect(screen.queryByTestId("files-upload-duplicate")).not.toBeInTheDocument()
  })

  it("marks an item as failed when attaching it fails (a standalone file would otherwise be silent)", async () => {
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
      ...fileHandlers.upload(SLUG, { title: "x", category: "images" }),
      ...fileHandlers.attach(SLUG, "uploaded-1", { status: 422 })
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

  it("hides the use-as-cover checkbox when no onSetCover handler is supplied", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture))
    renderDialog({
      initialFiles: [new File(["x"], "shot.png", { type: "image/png" })],
      linkedEntity: { type: "commodity", id: "com-9" },
    })
    await screen.findByText("shot.png")
    await user.click(screen.getByTestId("files-upload-next"))
    await screen.findByTestId("files-upload-metadata-list")
    expect(screen.queryByTestId(/files-upload-meta-cover-/)).not.toBeInTheDocument()
  })

  it("defaults the use-as-cover checkbox ON for the first photo when commodity has no cover", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture))
    const photoA = new File(["a"], "a.png", { type: "image/png" })
    const photoB = new File(["b"], "b.png", { type: "image/png" })
    renderDialog({
      initialFiles: [photoA, photoB],
      linkedEntity: { type: "commodity", id: "com-9" },
      onSetCover: () => {},
      commodityHasCover: false,
    })
    await screen.findByText("a.png")
    await user.click(screen.getByTestId("files-upload-next"))
    const checkboxes = await screen.findAllByTestId(/^files-upload-meta-cover-/)
    expect(checkboxes).toHaveLength(2)
    // Only the first photo has the box pre-ticked.
    expect((checkboxes[0] as HTMLInputElement).checked).toBe(true)
    expect((checkboxes[1] as HTMLInputElement).checked).toBe(false)
  })

  it("leaves the use-as-cover checkbox OFF when the commodity already has a cover", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture))
    renderDialog({
      initialFiles: [new File(["x"], "later.png", { type: "image/png" })],
      linkedEntity: { type: "commodity", id: "com-9" },
      onSetCover: () => {},
      commodityHasCover: true,
    })
    await screen.findByText("later.png")
    await user.click(screen.getByTestId("files-upload-next"))
    const checkboxes = await screen.findAllByTestId(/^files-upload-meta-cover-/)
    expect((checkboxes[0] as HTMLInputElement).checked).toBe(false)
  })

  it("does not re-promote a cover after the user explicitly unchecks it and adds another file", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture))
    renderDialog({
      initialFiles: [new File(["a"], "first.png", { type: "image/png" })],
      linkedEntity: { type: "commodity", id: "com-9" },
      onSetCover: () => {},
      commodityHasCover: false,
    })
    await screen.findByText("first.png")
    await user.click(screen.getByTestId("files-upload-next"))
    // Step 1: default-on → user unchecks.
    const checkboxFirst = (await screen.findByTestId(
      /^files-upload-meta-cover-/
    )) as HTMLInputElement
    expect(checkboxFirst.checked).toBe(true)
    await user.click(checkboxFirst)
    expect(
      ((await screen.findByTestId(/^files-upload-meta-cover-/)) as HTMLInputElement).checked
    ).toBe(false)
    // Step 2: go back, drop another photo. The first photo's checkbox
    // must NOT be silently re-promoted.
    await user.click(screen.getByRole("button", { name: /^back$/i }))
    const input = (await screen.findByTestId("files-upload-input")) as HTMLInputElement
    await user.upload(input, new File(["b"], "second.png", { type: "image/png" }))
    await user.click(screen.getByTestId("files-upload-next"))
    const checkboxes = await screen.findAllByTestId(/^files-upload-meta-cover-/)
    expect(checkboxes.map((c) => (c as HTMLInputElement).checked)).toEqual([false, false])
  })

  it("clears the use-as-cover flag when the user changes the category away from photos", async () => {
    const user = userEvent.setup()
    server.use(...groupHandlers.list(groupFixture))
    renderDialog({
      initialFiles: [new File(["x"], "lonely.png", { type: "image/png" })],
      linkedEntity: { type: "commodity", id: "com-9" },
      onSetCover: () => {},
      commodityHasCover: false,
    })
    await screen.findByText("lonely.png")
    await user.click(screen.getByTestId("files-upload-next"))
    const checkbox = await screen.findByTestId(/^files-upload-meta-cover-/)
    expect((checkbox as HTMLInputElement).checked).toBe(true)
    const categorySelect = screen.getByTestId(/^files-upload-meta-category-/)
    expect(categorySelect).toHaveAttribute("data-slot", "native-select")
    await user.selectOptions(categorySelect, "documents")
    // Switching category away from photos hides the checkbox entirely.
    expect(screen.queryByTestId(/^files-upload-meta-cover-/)).not.toBeInTheDocument()
  })

  it("calls onSetCover with the uploaded file id after upload completes", async () => {
    const onSetCover = (await import("vitest")).vi.fn()
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
      ...fileHandlers.upload(SLUG, { title: "cover-shot", category: "images" }),
      ...fileHandlers.attach(SLUG, "uploaded-1"),
      ...fileHandlers.duplicates(SLUG, "uploaded-1")
    )
    const user = userEvent.setup()
    renderDialog({
      initialFiles: [new File(["x"], "cover-shot.png", { type: "image/png" })],
      linkedEntity: { type: "commodity", id: "com-9" },
      onSetCover,
      commodityHasCover: false,
    })
    await screen.findByText("cover-shot.png")
    await user.click(screen.getByTestId("files-upload-next"))
    // Default-on checkbox stays checked → upload promotes the file.
    await user.click(screen.getByTestId("files-upload-start"))
    await waitFor(() => expect(onSetCover).toHaveBeenCalledWith("uploaded-1"))
  })

  describe("when the group already holds the same content", () => {
    const existing = {
      id: "existing-1",
      attributes: { title: "Store receipt", category: "documents", mime_type: "application/pdf" },
    }

    function seedDuplicate(onAttachExisting?: (attrs: Record<string, unknown>) => void) {
      let deleted = false
      server.use(
        ...groupHandlers.list(groupFixture),
        ...fileHandlers.uploadCapacity(SLUG),
        ...fileHandlers.upload(SLUG, { title: "receipt", category: "documents" }),
        ...fileHandlers.attach(SLUG, "uploaded-1"),
        ...fileHandlers.attach(SLUG, "existing-1", { onBody: onAttachExisting }),
        ...fileHandlers.duplicates(SLUG, "uploaded-1", [existing]),
        http.delete(apiUrl(`/g/${SLUG}/files/uploaded-1`), () => {
          deleted = true
          return new HttpResponse(null, { status: 204 })
        })
      )
      return { wasDeleted: () => deleted }
    }

    async function upload(user: ReturnType<typeof userEvent.setup>) {
      await screen.findByText("receipt.pdf")
      await user.click(screen.getByTestId("files-upload-next"))
      await user.click(screen.getByTestId("files-upload-start"))
      return screen.findByTestId("files-upload-duplicate")
    }

    it("attaches the existing file instead and deletes the new copy", async () => {
      const attached: Array<Record<string, unknown>> = []
      const { wasDeleted } = seedDuplicate((attrs) => attached.push(attrs))
      const user = userEvent.setup()
      renderDialog({
        initialFiles: [new File(["x"], "receipt.pdf", { type: "application/pdf" })],
        linkedEntity: { type: "commodity", id: "com-9" },
      })
      const notice = await upload(user)
      expect(notice).toHaveTextContent("Store receipt")
      await user.click(screen.getByTestId("files-upload-duplicate-use-existing"))
      expect(await screen.findByTestId("files-upload-duplicate-done")).toBeInTheDocument()
      expect(attached).toEqual([
        { entity_type: "commodity", entity_id: "com-9", meta: "documents" },
      ])
      expect(wasDeleted()).toBe(true)
    })

    it("moves the cover to the existing file when the new copy was the cover", async () => {
      const onSetCover = (await import("vitest")).vi.fn()
      seedDuplicate()
      const user = userEvent.setup()
      renderDialog({
        initialFiles: [new File(["x"], "receipt.pdf", { type: "image/png" })],
        linkedEntity: { type: "commodity", id: "com-9" },
        onSetCover,
        commodityHasCover: false,
      })
      await upload(user)
      await waitFor(() => expect(onSetCover).toHaveBeenCalledWith("uploaded-1"))
      await user.click(screen.getByTestId("files-upload-duplicate-use-existing"))
      await waitFor(() => expect(onSetCover).toHaveBeenLastCalledWith("existing-1"))
    })

    it("keeps both files when the user says so", async () => {
      const { wasDeleted } = seedDuplicate()
      const user = userEvent.setup()
      renderDialog({
        initialFiles: [new File(["x"], "receipt.pdf", { type: "application/pdf" })],
        linkedEntity: { type: "commodity", id: "com-9" },
      })
      await upload(user)
      await user.click(screen.getByTestId("files-upload-duplicate-keep-both"))
      expect(screen.queryByTestId("files-upload-duplicate")).not.toBeInTheDocument()
      expect(wasDeleted()).toBe(false)
    })

    it("offers to remove the copy when nothing is attached", async () => {
      const { wasDeleted } = seedDuplicate()
      const user = userEvent.setup()
      renderDialog({
        initialFiles: [new File(["x"], "receipt.pdf", { type: "application/pdf" })],
      })
      await upload(user)
      const button = screen.getByTestId("files-upload-duplicate-use-existing")
      expect(button).toHaveTextContent(/remove this copy/i)
      await user.click(button)
      expect(await screen.findByTestId("files-upload-duplicate-done")).toHaveTextContent(
        /removed this copy/i
      )
      expect(wasDeleted()).toBe(true)
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
