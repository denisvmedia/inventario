import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { FileDetailSheet } from "@/components/files/FileDetailSheet"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { fileHandlers, groupHandlers } from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function renderSheet(fileId: string | null = "f1", onEdit = vi.fn()) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/files`,
    routes: (
      <Route
        path="/g/:groupSlug/files"
        element={
          <GroupProvider>
            <ConfirmProvider>
              <FileDetailSheet
                fileId={fileId}
                open={!!fileId}
                onOpenChange={() => {}}
                onEdit={onEdit}
              />
            </ConfirmProvider>
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

describe("<FileDetailSheet />", () => {
  it("renders the metadata block once the file resolves", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(
        SLUG,
        "f1",
        {
          id: "f1",
          title: "Receipt",
          // #1622: invoice files live in `documents` and carry the
          // conventional `invoice` tag.
          category: "documents",
          type: "document",
          path: "rec",
          ext: ".pdf",
          mime_type: "application/pdf",
          tags: ["q4", "invoice"],
          linked_entity_type: "commodity",
          linked_entity_meta: "invoices",
          created_at: "2026-04-30T10:00:00Z",
        },
        { url: "https://cdn.example/rec.pdf" }
      )
    )
    renderSheet("f1")
    await waitFor(() =>
      expect(screen.getByTestId("file-detail-filename")).toHaveTextContent("rec.pdf")
    )
    expect(screen.getByTestId("file-detail-category")).toHaveTextContent("documents")
    // Tag pill rendered.
    expect(screen.getByText("q4")).toBeInTheDocument()
  })

  it("renders non-previewable files through the Sheet, not a separate dialog (#1962)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(
        SLUG,
        "f1",
        {
          id: "f1",
          title: "Archive",
          category: "other",
          type: "archive",
          path: "stuff",
          ext: ".zip",
          mime_type: "application/zip",
          size_bytes: 1024,
          created_at: "2026-04-30T10:00:00Z",
        },
        { url: "https://cdn.example/stuff.zip" }
      )
    )
    renderSheet("f1")
    // The body shows the "cannot preview, download to view" card once the
    // file resolves — wait on it so the assertions don't race the fetch.
    expect(await screen.findByTestId("file-detail-no-preview")).toBeInTheDocument()
    // Same right-side Sheet as image/PDF — the legacy centered dialog is gone.
    expect(screen.getByTestId("file-detail-sheet")).toBeInTheDocument()
    expect(screen.queryByTestId("file-preview-other-dialog")).not.toBeInTheDocument()
    // … and the full Sheet metadata + action row are present.
    expect(screen.getByTestId("file-detail-filename")).toHaveTextContent("stuff.zip")
    expect(screen.getByTestId("file-detail-edit")).toBeInTheDocument()
    expect(screen.getByTestId("file-detail-download")).toHaveAttribute(
      "href",
      "https://cdn.example/stuff.zip"
    )
  })

  it("delete button renders enabled on the Sheet for non-previewable files", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "X",
        category: "other",
        type: "other",
        path: "x",
        ext: "",
        mime_type: "application/octet-stream",
      })
    )
    renderSheet("f1")
    await waitFor(() => expect(screen.getByTestId("file-detail-delete")).toBeInTheDocument())
    expect(screen.getByTestId("file-detail-delete")).not.toBeDisabled()
  })

  it("renders the dash fallback when path is an empty string (regression #1483)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "Receipt",
        category: "documents",
        type: "document",
        path: "",
        ext: ".pdf",
        mime_type: "application/pdf",
      })
    )
    renderSheet("f1")
    await waitFor(() => expect(screen.getByTestId("file-detail-filename")).toHaveTextContent("—"))
  })

  it("renders the dash fallback when path is undefined", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "Receipt",
        category: "documents",
        type: "document",
        mime_type: "application/pdf",
      })
    )
    renderSheet("f1")
    await waitFor(() => expect(screen.getByTestId("file-detail-filename")).toHaveTextContent("—"))
  })

  it("renders the bare path when ext is missing", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "Note",
        category: "documents",
        type: "document",
        path: "notes-2026",
        mime_type: "application/pdf",
      })
    )
    renderSheet("f1")
    await waitFor(() =>
      expect(screen.getByTestId("file-detail-filename")).toHaveTextContent("notes-2026")
    )
  })

  it("clicking Delete opens the confirm prompt (non-previewable file)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "Archive",
        category: "other",
        type: "archive",
        path: "stuff",
        ext: ".zip",
        mime_type: "application/zip",
      })
    )
    renderSheet("f1")
    const deleteBtn = await screen.findByTestId("file-detail-delete")
    await user.click(deleteBtn)
    expect(await screen.findByTestId("confirm-dialog")).toBeInTheDocument()
    expect(screen.getByTestId("confirm-accept")).toBeInTheDocument()
  })

  it("clicking Edit calls onEdit with the file id", async () => {
    const user = userEvent.setup()
    const onEdit = vi.fn()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "X",
        category: "documents",
        type: "document",
        path: "x",
        ext: ".pdf",
        mime_type: "application/pdf",
      })
    )
    renderSheet("f1", onEdit)
    await waitFor(() => expect(screen.getByTestId("file-detail-edit")).toBeInTheDocument())
    await user.click(screen.getByTestId("file-detail-edit"))
    expect(onEdit).toHaveBeenCalledWith("f1")
  })

  it("wires a fullscreen affordance on the inline PDF preview (#1963)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(
        SLUG,
        "f1",
        {
          id: "f1",
          title: "Manual",
          category: "documents",
          type: "document",
          path: "manual",
          ext: ".pdf",
          mime_type: "application/pdf",
        },
        { url: "https://cdn.example/manual.pdf" }
      )
    )
    renderSheet("f1")
    // The inline PdfViewer's toolbar surfaces the fullscreen button because
    // FileDetailSheet passes onRequestFullscreen…
    expect(await screen.findByTestId("pdf-viewer-fullscreen")).toBeInTheDocument()
    // …and the fullscreen PDF dialog stays closed until it's used.
    expect(screen.queryByTestId("file-detail-pdf-fullscreen")).not.toBeInTheDocument()
  })

  it("'Open in new tab' targets the inline URL while Download keeps the attachment URL (#1962)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(
        SLUG,
        "f1",
        {
          id: "f1",
          title: "Receipt",
          category: "documents",
          type: "document",
          path: "rec",
          ext: ".pdf",
          mime_type: "application/pdf",
        },
        {
          url: "https://cdn.example/rec.pdf?disposition=attachment",
          inline_url: "https://cdn.example/rec.pdf?disposition=inline",
        }
      )
    )
    renderSheet("f1")
    const open = await screen.findByTestId("file-detail-open")
    expect(open).toHaveAttribute("href", "https://cdn.example/rec.pdf?disposition=inline")
    expect(open).toHaveAttribute("target", "_blank")
    const download = screen.getByTestId("file-detail-download")
    expect(download).toHaveAttribute("href", "https://cdn.example/rec.pdf?disposition=attachment")
    expect(download).toHaveAttribute("download")
  })

  it("'Open in new tab' falls back to the attachment URL when no inline URL is provided", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(
        SLUG,
        "f1",
        {
          id: "f1",
          title: "Receipt",
          category: "documents",
          type: "document",
          path: "rec",
          ext: ".pdf",
          mime_type: "application/pdf",
        },
        { url: "https://cdn.example/rec.pdf" }
      )
    )
    renderSheet("f1")
    const open = await screen.findByTestId("file-detail-open")
    expect(open).toHaveAttribute("href", "https://cdn.example/rec.pdf")
  })

  it("opens the fullscreen image viewer above the Sheet on image preview click (#1962)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(
        SLUG,
        "f1",
        {
          id: "f1",
          title: "Living room",
          category: "images",
          type: "image",
          path: "photo-livingroom",
          ext: ".jpg",
          mime_type: "image/jpeg",
        },
        { url: "https://cdn.example/photo-livingroom.jpg" }
      )
    )
    renderSheet("f1")
    const trigger = await screen.findByTestId("file-preview-image-trigger")
    expect(screen.queryByTestId("file-image-viewer")).not.toBeInTheDocument()
    await user.click(trigger)
    // The viewer (a stacked Radix Dialog) portals to document.body so it
    // paints above the Sheet; screen queries the whole document, so it's
    // found here.
    expect(await screen.findByTestId("file-image-viewer")).toBeInTheDocument()
  })
})
