import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { FileEditPage } from "@/pages/files/FileEditPage"
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

function renderEdit(id: string) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/files/${id}/edit`,
    routes: (
      <Route
        path="/g/:groupSlug/files/:id/edit"
        element={
          <GroupProvider>
            <ConfirmProvider>
              <FileEditPage />
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

describe("<FileEditPage />", () => {
  it("loads metadata and lets the user save edited values", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "Old title",
        description: "",
        path: "old-path",
        category: "documents",
        tags: ["receipt"],
      }),
      ...fileHandlers.update(SLUG, "f1", {})
    )
    renderEdit("f1")

    const titleInput = await screen.findByTestId("file-edit-title")
    await waitFor(() => expect(titleInput).toHaveValue("Old title"))

    await user.clear(titleInput)
    await user.type(titleInput, "New title")

    const save = screen.getByTestId("file-edit-save")
    await waitFor(() => expect(save).not.toBeDisabled())
    await user.click(save)
  })

  it("blocks save until a required field validation passes", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.detail(SLUG, "f1", {
        id: "f1",
        title: "",
        path: "kept",
        category: "other",
        tags: [],
      })
    )
    renderEdit("f1")

    const path = await screen.findByTestId("file-edit-path")
    await waitFor(() => expect(path).toHaveValue("kept"))
    await user.clear(path)
    // Path is required by the schema; clearing it should keep save
    // disabled (form is dirty + has validation errors).
    const save = screen.getByTestId("file-edit-save")
    await user.click(save)
    // After click validation runs; the form must surface an error
    // (i18n-translated; the schema emits a key, the page resolves it
    // through t()).
    expect(await screen.findByText(/path is required/i)).toBeInTheDocument()
  })
})
