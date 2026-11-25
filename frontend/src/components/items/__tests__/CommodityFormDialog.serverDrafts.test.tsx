import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests, HttpError } from "@/lib/http"
import { apiUrl, draftHandlers, groupHandlers, sellerHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

// Server drafts in the item form (#2812): Save draft stores the form on
// the server without the item rules, a resumed draft is promoted, and a
// save based on an old revision is refused instead of overwriting.

const SLUG = "household"
const DRAFT_KEY = `commodity-draft:${SLUG}:create`
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]
const areas = [{ id: "a1", name: "Garage", location_id: "l1" }]
const locations = [{ id: "l1", name: "Home" }]

function renderForm(
  props: {
    resumeDraftId?: string
    onSubmit?: (...args: unknown[]) => Promise<unknown>
    onOpenChange?: (open: boolean) => void
  } = {}
) {
  setAccessToken("good-token")
  const onSubmit = props.onSubmit ?? vi.fn().mockResolvedValue({ id: "new-item" })
  const onOpenChange = props.onOpenChange ?? vi.fn()
  renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/new`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/new"
        element={
          <GroupProvider>
            <CommodityFormDialog
              open
              onOpenChange={onOpenChange}
              mode="create"
              areas={areas}
              locations={locations}
              defaultCurrency="USD"
              onSubmit={onSubmit as never}
              draftKey={DRAFT_KEY}
              serverDrafts
              resumeDraftId={props.resumeDraftId}
            />
          </GroupProvider>
        }
      />
    ),
  })
  return { onSubmit, onOpenChange }
}

async function fillManually(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByTestId("commodity-form-ai-fill-manually"))
  return screen.findByLabelText(/^Name$/i)
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  window.localStorage.clear()
  window.sessionStorage.clear()
})

describe("<CommodityFormDialog /> server drafts", () => {
  it("saves a draft with only a name and a suggested short name", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onSubmit, onOpenChange } = renderForm()
    await user.type(await fillManually(user), "Bosch cordless drill GSR 18V")
    expect(screen.getByLabelText(/^Short name$/i)).toHaveValue("Bosch cordless drill")
    // No type, price, purchase date or location: a draft takes the form
    // as it is.
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-save-draft"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(onSubmit).not.toHaveBeenCalled()
    expect(fake.drafts).toHaveLength(1)
    expect(fake.drafts[0].values).toMatchObject({
      name: "Bosch cordless drill GSR 18V",
      short_name: "Bosch cordless drill",
    })
    // Saved on the server, so nothing is left to recover locally.
    expect(window.localStorage.getItem(DRAFT_KEY)).toBeNull()
  })

  it("keeps a short name the user typed when the name changes", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    renderForm()
    const name = await fillManually(user)
    await user.type(name, "Drill")
    const shortName = screen.getByLabelText(/^Short name$/i)
    await user.clear(shortName)
    await user.type(shortName, "Garage drill")
    await user.type(name, " with case")
    expect(shortName).toHaveValue("Garage drill")
  })

  it("promotes a resumed draft instead of creating an item", async () => {
    const fake = draftHandlers.backend(SLUG, [
      {
        id: "d1",
        revision: 3,
        values: { name: "Couch", short_name: "Couch", type: "furniture", draft: true },
      },
    ])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onSubmit } = renderForm({ resumeDraftId: "d1" })
    // A resumed draft opens on Basics with its values and the switch off.
    const name = await screen.findByLabelText(/^Name$/i)
    await waitFor(() => expect(name).toHaveValue("Couch"))
    expect(screen.getByLabelText(/Save as draft/i)).not.toBeChecked()
    for (const step of ["Purchase date", "warranty", "tags", "files"]) {
      await user.click(screen.getByTestId("commodity-form-next"))
      if (step === "Purchase date") await screen.findByLabelText(/Purchase date/i)
      if (step === "warranty") await screen.findByTestId("commodity-form-warranty-step")
      if (step === "tags") await screen.findByTestId("commodity-tags-input")
      if (step === "files") await screen.findByTestId("commodity-form-files-step")
    }
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const [values, draft] = (onSubmit as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(values).toMatchObject({ name: "Couch", type: "furniture", draft: false })
    expect(draft).toEqual({ id: "d1", revision: 3 })
  })

  it("shows a seller deleted while the draft was open on the Seller field", async () => {
    const fake = draftHandlers.backend(SLUG, [
      {
        id: "d1",
        revision: 2,
        values: { name: "Drill", short_name: "Drill", type: "equipment", seller_id: "s-gone" },
      },
    ])
    server.use(
      ...groupHandlers.list(groups),
      ...sellerHandlers.list(SLUG, [{ id: "s1", name: "Hardware store", item_count: 0 }]),
      ...fake.handlers
    )
    // The promotion's 422 on the field, as the server answers it.
    const envelope = {
      errors: [
        {
          status: "Unprocessable Entity",
          error: {
            type: "validation.Errors",
            error: {
              data: { attributes: { seller_id: "no seller with this ID in the group" } },
            },
          },
        },
      ],
    }
    const onSubmit = vi
      .fn()
      .mockRejectedValue(new HttpError("boom", 422, `/g/${SLUG}/drafts/d1/promote`, envelope))
    const user = userEvent.setup()
    renderForm({ resumeDraftId: "d1", onSubmit })
    const name = await screen.findByLabelText(/^Name$/i)
    await waitFor(() => expect(name).toHaveValue("Drill"))
    await user.click(screen.getByTestId("commodity-form-next"))
    expect(await screen.findByTestId("commodity-seller-gone")).toBeInTheDocument()
    for (const step of ["warranty", "tags", "files"]) {
      await user.click(screen.getByTestId("commodity-form-next"))
      if (step === "warranty") await screen.findByTestId("commodity-form-warranty-step")
      if (step === "tags") await screen.findByTestId("commodity-tags-input")
      if (step === "files") await screen.findByTestId("commodity-form-files-step")
    }
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const [values, draft] = (onSubmit as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(values).toMatchObject({ name: "Drill", seller_id: "s-gone" })
    expect(draft).toEqual({ id: "d1", revision: 2 })
    // The form goes back to Purchase, where the field still says why.
    expect(await screen.findByTestId("commodity-seller-gone")).toBeInTheDocument()
  })

  it("refuses to overwrite a draft saved elsewhere and loads the newer version", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", revision: 1, values: { name: "Lamp", short_name: "Lamp" } },
    ])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onOpenChange } = renderForm({ resumeDraftId: "d1" })
    const name = await screen.findByLabelText(/^Name$/i)
    await waitFor(() => expect(name).toHaveValue("Lamp"))
    // Another device saves the draft meanwhile.
    fake.drafts[0].values = { name: "Desk lamp", short_name: "Desk lamp" }
    fake.drafts[0].revision = 2
    await user.type(name, " (old)")
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-save-draft"))
    await screen.findByTestId("draft-save-conflict")
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(fake.drafts[0].values).toEqual({ name: "Desk lamp", short_name: "Desk lamp" })
    await user.click(screen.getByTestId("draft-conflict-load"))
    await waitFor(() => expect(screen.getByLabelText(/^Name$/i)).toHaveValue("Desk lamp"))
    expect(screen.queryByTestId("draft-save-conflict")).not.toBeInTheDocument()
  })

  it("keeps the input when the save fails and saves it on retry", async () => {
    const fake = draftHandlers.backend(SLUG)
    let fail = true
    server.use(
      ...groupHandlers.list(groups),
      http.post(apiUrl(`/g/${SLUG}/drafts`), () => {
        if (fail) return HttpResponse.json({ errors: [{ status: "503" }] }, { status: 503 })
        return undefined
      }),
      ...fake.handlers
    )
    const user = userEvent.setup()
    const { onOpenChange } = renderForm()
    await user.type(await fillManually(user), "Kettle")
    // Closing offers to save the draft on the server.
    await user.click(screen.getByTestId("commodity-form-cancel"))
    expect(await screen.findByTestId("commodity-form-close-confirm")).toHaveTextContent(
      /any device/i
    )
    await user.click(screen.getByTestId("commodity-form-close-confirm-save"))
    expect(await screen.findByTestId("draft-save-error")).toHaveTextContent(/this device/i)
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(screen.getByLabelText(/^Name$/i)).toHaveValue("Kettle")
    fail = false
    await user.click(screen.getByTestId("draft-save-retry"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(fake.drafts.map((d) => d.values.name)).toEqual(["Kettle"])
  })
})
