import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { AttachExistingFileDialog } from "@/components/files/AttachExistingFileDialog"
import { GroupProvider } from "@/features/group/GroupContext"
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

const files = [
  {
    id: "receipt",
    attributes: {
      title: "Store receipt",
      category: "documents",
      links: [{ id: "l1", entity_type: "commodity", entity_id: "tv", entity_name: "TV" }],
    },
  },
  {
    id: "manual",
    attributes: {
      title: "Soundbar manual",
      category: "documents",
      links: [{ id: "l2", entity_type: "commodity", entity_id: "soundbar" }],
    },
  },
  { id: "loose", attributes: { title: "Loose scan", category: "other", links: [] } },
]

function renderDialog() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/soundbar`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <GroupProvider>
            <AttachExistingFileDialog
              open
              onOpenChange={() => {}}
              entity={{ type: "commodity", id: "soundbar", name: "Soundbar" }}
            />
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

describe("<AttachExistingFileDialog />", () => {
  it("lists the group's files and marks the ones already attached here", async () => {
    server.use(...groupHandlers.list(groupFixture), ...fileHandlers.list(SLUG, files))
    renderDialog()
    expect(
      await screen.findByRole("heading", { name: /attach an existing file to soundbar/i })
    ).toBeInTheDocument()
    const receiptRow = await screen.findByTestId("attach-existing-row-receipt")
    expect(receiptRow).toHaveTextContent("TV")
    // Without a name the row falls back to the kind of entity.
    expect(screen.getByTestId("attach-existing-row-manual")).toHaveTextContent("Item")
    expect(screen.getByTestId("attach-existing-attach-receipt")).toBeEnabled()
    // Already attached to this item: no Attach button.
    expect(screen.queryByTestId("attach-existing-attach-manual")).not.toBeInTheDocument()
    expect(screen.getByTestId("attach-existing-row-manual")).toHaveTextContent("Attached")
    expect(screen.getByTestId("attach-existing-row-loose")).toHaveTextContent(
      "Not attached to anything"
    )
  })

  it("attaches the chosen file to the entity", async () => {
    const bodies: Array<Record<string, unknown>> = []
    server.use(
      ...groupHandlers.list(groupFixture),
      ...fileHandlers.list(SLUG, files),
      ...fileHandlers.attach(SLUG, "receipt", { onBody: (attrs) => bodies.push(attrs) })
    )
    const user = userEvent.setup()
    renderDialog()
    await user.click(await screen.findByTestId("attach-existing-attach-receipt"))
    await waitFor(() => expect(bodies).toHaveLength(1))
    // No meta: the BE puts the file in the entity's default bucket.
    expect(bodies[0]).toEqual({ entity_type: "commodity", entity_id: "soundbar" })
  })

  it("searches the group's files", async () => {
    server.use(...groupHandlers.list(groupFixture), ...fileHandlers.list(SLUG, files))
    const user = userEvent.setup()
    renderDialog()
    await screen.findByTestId("attach-existing-row-receipt")
    await user.type(screen.getByTestId("attach-existing-search"), "loose")
    await waitFor(() =>
      expect(screen.queryByTestId("attach-existing-row-receipt")).not.toBeInTheDocument()
    )
    expect(screen.getByTestId("attach-existing-row-loose")).toBeInTheDocument()
  })
})
