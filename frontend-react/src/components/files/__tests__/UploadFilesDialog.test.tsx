import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { UploadFilesDialog } from "@/components/files/UploadFilesDialog"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { groupHandlers } from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
]

function renderDialog() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/files`,
    routes: (
      <Route
        path="/g/:groupSlug/files"
        element={
          <GroupProvider>
            <UploadFilesDialog open onOpenChange={() => {}} />
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
