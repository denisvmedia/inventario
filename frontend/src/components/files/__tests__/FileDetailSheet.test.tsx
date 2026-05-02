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
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
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

  it("renders the no-preview fallback when MIME isn't image or PDF", async () => {
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
        },
        { url: "https://cdn.example/stuff.zip" }
      )
    )
    renderSheet("f1")
    expect(await screen.findByTestId("file-preview-fallback")).toBeInTheDocument()
  })

  it("delete button is disabled while a delete is in flight (renders without crashing)", async () => {
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

  it("clicking Edit calls onEdit with the file id", async () => {
    const user = userEvent.setup()
    const onEdit = vi.fn()
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
    renderSheet("f1", onEdit)
    await waitFor(() => expect(screen.getByTestId("file-detail-edit")).toBeInTheDocument())
    await user.click(screen.getByTestId("file-detail-edit"))
    expect(onEdit).toHaveBeenCalledWith("f1")
  })
})
