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
          category: "invoices",
          type: "document",
          path: "rec",
          ext: ".pdf",
          mime_type: "application/pdf",
          tags: ["q4"],
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
    expect(screen.getByTestId("file-detail-category")).toHaveTextContent("invoices")
    // Tag pill rendered.
    expect(screen.getByText("q4")).toBeInTheDocument()
  })

  it("routes to the small Dialog (not the Sheet) when MIME isn't image or PDF", async () => {
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
    expect(await screen.findByTestId("file-preview-other-dialog")).toBeInTheDocument()
    // Sheet-only metadata block and action row should not render.
    expect(screen.queryByTestId("file-detail-sheet")).not.toBeInTheDocument()
    expect(screen.queryByTestId("file-detail-edit")).not.toBeInTheDocument()
    // Dialog shows the filename header, size, and Download CTA.
    expect(screen.getByTestId("file-preview-other-filename")).toHaveTextContent("Archive")
    expect(screen.getByTestId("file-preview-other-download")).toHaveAttribute(
      "href",
      "https://cdn.example/stuff.zip"
    )
  })

  it("delete button renders enabled on the small Dialog branch", async () => {
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
    await waitFor(() => expect(screen.getByTestId("file-preview-other-delete")).toBeInTheDocument())
    expect(screen.getByTestId("file-preview-other-delete")).not.toBeDisabled()
  })

  it("renders the dash fallback when path is an empty string (regression #1483)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "Receipt",
        category: "invoices",
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
        category: "invoices",
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
      // Use a PDF so the file stays on the Sheet path (which renders
      // the metadata block with the filename row). Non-previewable
      // mimes route to the small Dialog after #1541.
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

  it("clicking Delete on the small Dialog opens the confirm prompt", async () => {
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
    const deleteBtn = await screen.findByTestId("file-preview-other-delete")
    await user.click(deleteBtn)
    expect(await screen.findByTestId("confirm-dialog")).toBeInTheDocument()
    expect(screen.getByTestId("confirm-accept")).toBeInTheDocument()
  })

  it("clicking Edit calls onEdit with the file id", async () => {
    const user = userEvent.setup()
    const onEdit = vi.fn()
    server.use(
      ...groupHandlers.list(groupFixture),
      // Edit affordance lives on the Sheet path — use a PDF so the
      // mime routing keeps the test on that surface.
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
})
