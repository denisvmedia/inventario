import { useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, draftHandlers, groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

// Files added while editing an existing item (#2916): they are uploaded and
// attached before the changes are saved, with the dialog open, and a file
// that fails stays for a retry instead of being lost or left unattached.

const SLUG = "household"
const ITEM = "c1"
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]
const areas = [{ id: "a1", name: "Garage", location_id: "l1" }]
const locations = [{ id: "l1", name: "Home" }]
const existing = {
  id: ITEM,
  name: "Kettle",
  short_name: "Kettle",
  type: "other" as const,
  area_id: "a1",
  status: "in_use" as const,
  count: 1,
  draft: false,
  purchase_date: "2024-03-01",
  original_price: 40,
  original_price_currency: "USD",
}

type FakeBackend = ReturnType<typeof draftHandlers.backend>

// The files of the fake linked to the item.
function itemFiles(fake: FakeBackend) {
  return fake.files.filter((f) =>
    f.links.some((l) => l.entity_type === "commodity" && l.entity_id === ITEM)
  )
}

// renderEdit opens the item in the dialog's edit mode inside a host that
// closes it the way the item page does. onSubmit records which files the
// item had when the changes were sent.
function renderEdit(fake: FakeBackend) {
  setAccessToken("good-token")
  const filesAtSave: string[][] = []
  const onSubmit = vi.fn(async () => {
    filesAtSave.push(itemFiles(fake).map((f) => f.id))
  })
  const onOpenChange = vi.fn()
  function Host() {
    const [open, setOpen] = useState(true)
    return (
      <CommodityFormDialog
        open={open}
        onOpenChange={(next) => {
          onOpenChange(next)
          setOpen(next)
        }}
        mode="edit"
        initialValues={existing}
        areas={areas}
        locations={locations}
        defaultCurrency="USD"
        onSubmit={onSubmit}
      />
    )
  }
  renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/${ITEM}/edit`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id/edit"
        element={
          <GroupProvider>
            <Host />
          </GroupProvider>
        }
      />
    ),
  })
  return { onSubmit, onOpenChange, filesAtSave }
}

async function stageOnFiles(user: ReturnType<typeof userEvent.setup>, files: File[]) {
  await screen.findByLabelText(/^Name$/i)
  const stepper = screen.getByRole("list", { name: /form steps/i })
  await user.click(within(stepper).getByRole("button", { name: "Files" }))
  const input = (await screen.findByTestId("commodity-files-dropzone")).querySelector(
    "input[type=file]"
  ) as HTMLInputElement
  await user.upload(input, files)
}

const png = (name: string) => new File(["png"], name, { type: "image/png" })
const pendingRows = () => screen.queryAllByTestId("commodity-files-pending")

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  window.localStorage.clear()
  window.sessionStorage.clear()
})

describe("<CommodityFormDialog /> files added while editing (#2916)", () => {
  it("attaches the new files to the item before it saves the changes", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.nextUploads = [{ name: "receipt.png", size: 3 }]
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onSubmit, onOpenChange, filesAtSave } = renderEdit(fake)
    await stageOnFiles(user, [png("receipt.png")])
    await user.click(screen.getByTestId("commodity-form-submit"))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    // The file was on the item when the changes went out.
    expect(filesAtSave[0]).toHaveLength(1)
    expect(itemFiles(fake)).toHaveLength(1)
    expect(itemFiles(fake)[0].title).toBe("receipt")
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(fake.trashed).toEqual([])
  })

  it("keeps the changes unsaved and the file staged when its upload fails", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.failUploads = 500
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onSubmit } = renderEdit(fake)
    await stageOnFiles(user, [png("receipt.png")])
    await user.click(screen.getByTestId("commodity-form-submit"))

    const alert = await screen.findByTestId("draft-save-error")
    expect(alert).toHaveTextContent(/Changes not saved/i)
    expect(alert).toHaveTextContent(/Not uploaded: receipt\.png/i)
    expect(onSubmit).not.toHaveBeenCalled()
    // The dialog is open on the file, marked as failed.
    expect(pendingRows()).toHaveLength(1)
    expect(pendingRows()[0]).toHaveAttribute("data-status", "failed")

    // Retry goes on from there and then saves the changes.
    fake.failUploads = null
    await user.click(within(alert).getByTestId("draft-save-retry"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(fake.uploads).toBe(1)
    expect(itemFiles(fake)).toHaveLength(1)
  })

  it("attaches the same upload on Retry after a failed attach", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.failAttach = 500
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onSubmit } = renderEdit(fake)
    await stageOnFiles(user, [png("receipt.png")])
    await user.click(screen.getByTestId("commodity-form-submit"))

    const alert = await screen.findByTestId("draft-save-error")
    expect(onSubmit).not.toHaveBeenCalled()
    expect(fake.uploads).toBe(1)
    expect(itemFiles(fake)).toHaveLength(0)

    fake.failAttach = null
    await user.click(within(alert).getByTestId("draft-save-retry"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    // Not sent a second time.
    expect(fake.uploads).toBe(1)
    expect(fake.files).toHaveLength(1)
    expect(itemFiles(fake)).toHaveLength(1)
  })

  it("moves an upload whose attach failed to Trash when the dialog closes", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.failAttach = 500
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onSubmit, onOpenChange } = renderEdit(fake)
    await stageOnFiles(user, [png("receipt.png")])
    await user.click(screen.getByTestId("commodity-form-submit"))
    await screen.findByTestId("draft-save-error")
    const uploaded = fake.files[0].id

    await user.click(screen.getByTestId("commodity-form-cancel"))
    expect(onOpenChange).toHaveBeenCalledWith(false)
    await waitFor(() => expect(fake.trashed).toEqual([uploaded]))
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("leaves the files that made it on the item when a later one fails", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.nextUploads = [{ name: "a.png", size: 3 }]
    // The second upload fails; the first goes through to the fake.
    let calls = 0
    server.use(
      ...groupHandlers.list(groups),
      http.post(apiUrl(`/g/${SLUG}/uploads/file`), () => {
        calls++
        if (calls === 2) return HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
        return undefined
      }),
      ...fake.handlers
    )
    const user = userEvent.setup()
    const { onSubmit } = renderEdit(fake)
    await stageOnFiles(user, [png("a.png"), png("b.png")])
    await user.click(screen.getByTestId("commodity-form-submit"))

    const alert = await screen.findByTestId("draft-save-error")
    expect(alert).toHaveTextContent(/Not uploaded: b\.png/i)
    expect(onSubmit).not.toHaveBeenCalled()
    expect(itemFiles(fake)).toHaveLength(1)
    // Only the file still missing stays on the form.
    await waitFor(() => expect(pendingRows()).toHaveLength(1))
    expect(pendingRows()[0]).toHaveTextContent("b.png")
  })
})
