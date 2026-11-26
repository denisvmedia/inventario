import { useEffect, useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { act, fireEvent, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests, HttpError } from "@/lib/http"
import {
  apiUrl,
  customFieldHandlers,
  draftHandlers,
  groupHandlers,
  sellerHandlers,
} from "@/test/handlers"
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

function Probe() {
  const loc = useLocation()
  return <div data-testid="loc">{loc.pathname}</div>
}

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
      <>
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
        <Route path="/g/:groupSlug/commodities/:id" element={<Probe />} />
      </>
    ),
  })
  return { onSubmit, onOpenChange }
}

async function fillManually(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByTestId("commodity-form-ai-fill-manually"))
  return screen.findByLabelText(/^Name$/i)
}

// openResumed opens draft id in the form and waits until its name shows.
async function openResumed(id: string, name: string, props: Parameters<typeof renderForm>[0] = {}) {
  const rendered = renderForm({ ...props, resumeDraftId: id })
  const input = await screen.findByLabelText(/^Name$/i)
  await waitFor(() => expect(input).toHaveValue(name))
  return { ...rendered, input }
}

// openResumedOn opens draft id in the form and waits for the step it lands
// on: a draft that has a type opens past Basics, on the step it still needs.
async function openResumedOn(
  id: string,
  stepTestId: string,
  props: Parameters<typeof renderForm>[0] = {}
) {
  const rendered = renderForm({ ...props, resumeDraftId: id })
  await screen.findByTestId(stepTestId)
  return rendered
}

// toBasics goes back to Basics with the stepper and returns the Name field.
async function toBasics(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Basics" }))
  return screen.findByLabelText(/^Name$/i)
}

async function walkToFiles(
  user: ReturnType<typeof userEvent.setup>,
  steps = ["purchase", "warranty", "tags", "files"]
) {
  for (const step of steps) {
    await user.click(screen.getByTestId("commodity-form-next"))
    if (step === "purchase") await screen.findByLabelText(/Purchase date/i)
    if (step === "warranty") await screen.findByTestId("commodity-form-warranty-step")
    if (step === "tags") await screen.findByTestId("commodity-tags-input")
    if (step === "files") await screen.findByTestId("commodity-form-files-step")
  }
}

// promoteRefusal is the 409 the server answers a promotion with.
function refusal(code: string, meta: Record<string, string> = {}) {
  return new HttpError("refused", 409, `/g/${SLUG}/drafts/d1/promote`, {
    errors: [{ status: "409", code, meta }],
  })
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

  // Continue picks up where the draft was left (#2794): the first step with
  // a field the form refuses, else the step after the last one with values.
  it.each([
    ["Basics, while the type is missing", { name: "Lamp", short_name: "Lamp" }, "basics"],
    [
      "Purchase, after a filled Basics",
      { name: "Lamp", short_name: "Lamp", type: "furniture" },
      "purchase",
    ],
    [
      "Warranty, after a purchase price",
      { name: "Lamp", short_name: "Lamp", type: "furniture", original_price: "40" },
      "warranty",
    ],
    [
      "Files, after tags",
      { name: "Lamp", short_name: "Lamp", type: "furniture", tags: ["living-room"] },
      "files",
    ],
    [
      "Purchase, for a foreign price with no group-currency amount",
      {
        name: "Lamp",
        short_name: "Lamp",
        type: "furniture",
        original_price: "40",
        original_price_currency: "EUR",
        tags: ["living-room"],
      },
      "purchase",
    ],
  ])("opens a resumed draft on %s", async (_label, values, step) => {
    const fake = draftHandlers.backend(SLUG, [{ id: "d1", revision: 1, values }])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    await openResumedOn("d1", `commodity-form-${step}-step`)
    // The steps before it are one click away.
    if (step !== "basics") {
      expect(screen.getByRole("button", { name: "Basics" })).toBeEnabled()
    }
  })

  it("marks the field a resumed draft opens on because the form refuses it", async () => {
    const fake = draftHandlers.backend(SLUG, [
      {
        id: "d1",
        revision: 1,
        values: {
          name: "Lamp",
          short_name: "Lamp",
          type: "furniture",
          original_price: "40",
          original_price_currency: "EUR",
        },
      },
    ])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    await openResumedOn("d1", "commodity-form-purchase-step")
    await waitFor(() => expect(document.querySelector('[aria-invalid="true"]')).not.toBeNull())
  })

  it("marks nothing on the step a complete resumed draft opens on", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", revision: 1, values: { name: "Lamp", short_name: "Lamp", type: "furniture" } },
    ])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    await openResumedOn("d1", "commodity-form-purchase-step")
    await screen.findByLabelText(/Purchase date/i)
    expect(document.querySelector('[aria-invalid="true"]')).toBeNull()
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
    const { onSubmit } = await openResumedOn("d1", "commodity-form-purchase-step")
    // Basics is filled in, so the draft opens on the next step; Basics shows
    // its values and the switch off.
    const name = await toBasics(user)
    expect(name).toHaveValue("Couch")
    expect(screen.getByLabelText(/Save as draft/i)).not.toBeChecked()
    await walkToFiles(user)
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const [values, draft] = (onSubmit as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(values).toMatchObject({ name: "Couch", type: "furniture", draft: false })
    // The promotion names its key, kept for a retry until it succeeds.
    expect(draft).toEqual({
      id: "d1",
      revision: 3,
      promotionKey: expect.stringMatching(/^promote:/),
    })
  })

  it("saves the custom field values typed in the form with the draft", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(
      ...groupHandlers.list(groups),
      ...customFieldHandlers.list(SLUG, [
        customFieldHandlers.field({ id: "cf-plate", name: "Plate" }),
      ]),
      ...fake.handlers
    )
    const user = userEvent.setup()
    const { onOpenChange } = renderForm()
    await user.type(await fillManually(user), "Van")
    await user.click(screen.getByLabelText(/Save as draft/i))
    for (const step of ["purchase", "warranty", "tags"]) {
      await user.click(screen.getByTestId("commodity-form-next"))
      if (step === "purchase") await screen.findByLabelText(/Purchase date/i)
      if (step === "warranty") await screen.findByTestId("commodity-form-warranty-step")
      if (step === "tags") await screen.findByTestId("commodity-tags-input")
    }
    await user.click(await screen.findByTestId("custom-field-add"))
    await user.click(await screen.findByTestId("custom-field-add-cf-plate"))
    await user.type(screen.getByLabelText(/^Plate$/), "AB 123")
    await user.click(screen.getByTestId("commodity-form-save-draft"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(fake.drafts).toHaveLength(1)
    expect(fake.drafts[0].values.custom_fields).toEqual({
      "cf-plate": { value: "AB 123", currency: "" },
    })
  })

  it("resumes a draft's custom field values and promotes the item with them", async () => {
    const fake = draftHandlers.backend(SLUG, [
      {
        id: "d1",
        revision: 2,
        values: {
          name: "Van",
          short_name: "Van",
          type: "furniture",
          custom_fields: { "cf-plate": { value: "AB 123", currency: "" } },
        },
      },
    ])
    server.use(
      ...groupHandlers.list(groups),
      ...customFieldHandlers.list(SLUG, [
        customFieldHandlers.field({ id: "cf-plate", name: "Plate" }),
      ]),
      ...fake.handlers
    )
    const user = userEvent.setup()
    // The values are on Extras, so the draft opens on the step after it.
    const { onSubmit } = await openResumedOn("d1", "commodity-form-files-step")
    await user.click(screen.getByRole("button", { name: "Extras" }))
    expect(await screen.findByLabelText(/^Plate$/)).toHaveValue("AB 123")
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-files-step")
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const [values, draft] = (onSubmit as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(values.custom_fields).toEqual({ "cf-plate": "AB 123" })
    expect(draft).toMatchObject({ id: "d1", revision: 2 })
  })

  it("offers to create the seller a batch intake draft names", async () => {
    const fake = draftHandlers.backend(SLUG, [
      {
        id: "d1",
        values: {
          name: "Espresso Machine",
          short_name: "Espresso",
          type: "equipment",
          seller_name: "Sample Store",
        },
      },
    ])
    server.use(
      ...groupHandlers.list(groups),
      ...sellerHandlers.list(SLUG, [{ id: "s1", name: "Hardware store", item_count: 0 }]),
      ...fake.handlers
    )
    // Basics is complete, so the draft opens on Purchase, with the seller.
    await openResumedOn("d1", "commodity-form-purchase-step")
    expect(await screen.findByTestId("commodity-seller-suggestion")).toHaveTextContent(
      "Sample Store"
    )
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
    // The seller is on Purchase, so the draft opens on Warranty.
    await openResumedOn("d1", "commodity-form-warranty-step", { onSubmit })
    await user.click(screen.getByRole("button", { name: "Purchase" }))
    expect(await screen.findByTestId("commodity-seller-gone")).toBeInTheDocument()
    await walkToFiles(user, ["warranty", "tags", "files"])
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const [values, draft] = (onSubmit as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(values).toMatchObject({ name: "Drill", seller_id: "s-gone" })
    expect(draft).toMatchObject({ id: "d1", revision: 2 })
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

  it("shows a type the group no longer has on the type field", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", revision: 1, values: { name: "Dinghy", short_name: "Dinghy", type: "boat" } },
    ])
    server.use(
      ...groupHandlers.list(groups),
      http.put(apiUrl(`/g/${SLUG}/drafts/d1`), () =>
        HttpResponse.json(
          {
            errors: [
              {
                status: "Unprocessable Entity",
                error: {
                  error: { data: { attributes: { values: { type: "no item type" } } } },
                  errorCodes: {
                    data: { attributes: { values: { type: { code: "item_type_unknown" } } } },
                  },
                },
              },
            ],
          },
          { status: 422 }
        )
      ),
      ...fake.handlers
    )
    const user = userEvent.setup()
    const { onOpenChange } = await openResumedOn("d1", "commodity-form-purchase-step")
    const input = await toBasics(user)
    expect(input).toHaveValue("Dinghy")
    await user.type(input, " 2")
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-save-draft"))
    expect(await screen.findByTestId("draft-save-error")).toHaveTextContent(
      "This type is not in the group."
    )
    expect(screen.getAllByText("This type is not in the group.")).toHaveLength(2)
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(fake.drafts[0].revision).toBe(1)
  })

  it("finishes a save whose response was lost instead of reporting a conflict", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", revision: 1, values: { name: "Lamp", short_name: "Lamp" } },
    ])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onOpenChange, input } = await openResumed("d1", "Lamp")
    await user.type(input, " shade")
    await user.click(screen.getByLabelText(/Save as draft/i))
    // The server stores the save; the answer never arrives.
    fake.loseResponse.put = true
    await user.click(screen.getByTestId("commodity-form-save-draft"))
    await screen.findByTestId("draft-save-error")
    expect(fake.drafts[0].revision).toBe(2)
    // The retry is based on revision 1, which the server has moved past
    // with these same values: the save is done, not a conflict.
    await user.click(screen.getByTestId("draft-save-retry"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(screen.queryByTestId("draft-save-conflict")).not.toBeInTheDocument()
    expect(fake.drafts[0]).toMatchObject({ revision: 2, values: { name: "Lamp shade" } })
  })

  it("offers the item when the draft was finished elsewhere, and keeps the input as a new draft", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", revision: 1, values: { name: "Lamp", short_name: "Lamp" } },
    ])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onOpenChange, input } = await openResumed("d1", "Lamp")
    // Another device finishes the draft.
    fake.drafts[0].status = "promoted"
    fake.drafts[0].promoted_commodity_id = "item-7"
    await user.type(input, " shade")
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-save-draft"))
    const alert = await screen.findByTestId("draft-save-promoted")
    expect(alert).toHaveTextContent("This draft is already an item")
    expect(screen.queryByTestId("draft-save-retry")).not.toBeInTheDocument()
    expect(screen.getByTestId("draft-promoted-open-item")).toBeInTheDocument()
    await user.click(screen.getByTestId("draft-conflict-save-new"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(fake.drafts).toHaveLength(2)
    expect(fake.drafts[1]).toMatchObject({ status: "open", values: { name: "Lamp shade" } })
  })

  it("sends the same promotion key when Add item is clicked again", async () => {
    const fake = draftHandlers.backend(SLUG, [
      {
        id: "d1",
        revision: 1,
        values: { name: "Lamp", short_name: "Lamp", type: "furniture" },
      },
    ])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const onSubmit = vi
      .fn()
      .mockRejectedValueOnce(new HttpError("unavailable", 503, `/g/${SLUG}/drafts/d1/promote`, {}))
      .mockResolvedValueOnce({ id: "item-from-d1" })
    await openResumedOn("d1", "commodity-form-purchase-step", { onSubmit })
    await walkToFiles(user, ["warranty", "tags", "files"])
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    await user.click(await screen.findByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(2))
    const keys = (onSubmit as ReturnType<typeof vi.fn>).mock.calls.map(
      ([, draft]) => (draft as { promotionKey: string }).promotionKey
    )
    expect(keys[0]).toMatch(/^promote:/)
    expect(keys[1]).toBe(keys[0])
  })

  it("opens the item a draft became elsewhere", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", revision: 1, values: { name: "Lamp", short_name: "Lamp", type: "furniture" } },
    ])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const onSubmit = vi
      .fn()
      .mockRejectedValue(refusal("draft.promoted", { commodity_id: "item-7" }))
    const { onOpenChange } = await openResumedOn("d1", "commodity-form-purchase-step", {
      onSubmit,
    })
    await walkToFiles(user, ["warranty", "tags", "files"])
    await user.click(screen.getByTestId("commodity-form-submit"))
    await screen.findByTestId("draft-save-promoted")
    await user.click(screen.getByTestId("draft-promoted-open-item"))
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(await screen.findByTestId("loc")).toHaveTextContent(`/g/${SLUG}/commodities/item-7`)
  })

  it("keeps the input as a new draft when the draft was discarded elsewhere", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", revision: 1, values: { name: "Lamp", short_name: "Lamp" } },
    ])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { input } = await openResumed("d1", "Lamp")
    fake.drafts[0].status = "discarding"
    await user.type(input, " shade")
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-save-draft"))
    expect(await screen.findByTestId("draft-save-discarded")).toHaveTextContent(/discarded/i)
    await user.click(screen.getByTestId("draft-conflict-save-new"))
    await waitFor(() => expect(fake.drafts).toHaveLength(2))
    expect(fake.drafts[1].values).toMatchObject({ name: "Lamp shade" })
  })

  it("links the files saved with the old draft to the new one", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", revision: 1, values: { name: "Lamp", short_name: "Lamp" } },
    ])
    fake.files.push({
      id: "file-photo",
      path: "lamp",
      ext: ".jpg",
      size_bytes: 10,
      category: "images",
      tags: [],
      links: [
        {
          id: "link-1",
          file_id: "file-photo",
          entity_type: "draft",
          entity_id: "d1",
          meta: "images",
        },
      ],
    })
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onOpenChange, input } = await openResumed("d1", "Lamp")
    fake.drafts[0].revision = 2
    await user.type(input, " (mine)")
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-save-draft"))
    await screen.findByTestId("draft-save-conflict")
    await user.click(screen.getByTestId("draft-conflict-save-new"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    const created = fake.drafts[1]
    expect(created.values).toMatchObject({ name: "Lamp (mine)" })
    expect(fake.files[0].links.map((l) => l.entity_id)).toEqual(["d1", created.id])
  })

  it("opens a draft whose values the form cannot use", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", revision: 1, values: { name: "Lamp", short_name: null, count: 2, tags: "x" } },
    ])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    await openResumed("d1", "Lamp")
    expect(screen.getByLabelText(/^Short name$/i)).toHaveValue("")
  })

  // The form's schema follows the group's custom fields. Their arrival must
  // not send the form back to its first step and drop what was typed.
  it("keeps the input when the group's custom fields arrive", async () => {
    const fake = draftHandlers.backend(SLUG)
    let release = () => {}
    const held = new Promise<void>((resolve) => {
      release = resolve
    })
    let served = false
    server.use(
      ...groupHandlers.list(groups),
      ...fake.handlers,
      http.get(apiUrl(`/g/${SLUG}/custom-fields`), async () => {
        await held
        served = true
        return HttpResponse.json({
          data: [
            {
              id: "cf-plate",
              type: "custom_fields",
              attributes: customFieldHandlers.field({ id: "cf-plate", name: "Plate" }),
            },
          ],
          meta: { custom_fields: 1, limit: 200 },
        })
      })
    )
    const user = userEvent.setup()
    renderForm()
    await user.type(await fillManually(user), "Kettle")
    release()
    await waitFor(() => expect(served).toBe(true))
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(screen.queryByTestId("commodity-form-ai-step")).not.toBeInTheDocument()
    expect(screen.getByLabelText(/^Name$/i)).toHaveValue("Kettle")
  })

  it("runs one save when Enter and Save draft come together", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    renderForm()
    await user.type(await fillManually(user), "Kettle")
    await user.click(screen.getByLabelText(/Save as draft/i))
    await walkToFiles(user)
    const fileInput = screen
      .getByTestId("commodity-files-dropzone")
      .querySelector("input[type=file]") as HTMLInputElement
    fireEvent.keyDown(fileInput, { key: "Enter" })
    fireEvent.keyDown(fileInput, { key: "Enter" })
    fireEvent.click(screen.getByTestId("commodity-form-save-draft"))
    await waitFor(() => expect(fake.drafts).toHaveLength(1))
    await waitFor(() => expect(screen.queryByTestId("draft-save-error")).not.toBeInTheDocument())
    expect(fake.creates).toBe(1)
  })

  it("attaches a file uploaded by a failed save on retry, without uploading it again", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onOpenChange } = renderForm()
    await user.type(await fillManually(user), "Kettle")
    await user.click(screen.getByLabelText(/Save as draft/i))
    await walkToFiles(user)
    const fileInput = screen
      .getByTestId("commodity-files-dropzone")
      .querySelector("input[type=file]") as HTMLInputElement
    await user.upload(fileInput, new File(["png"], "kettle.png", { type: "image/png" }))
    fake.failAttach = 500
    await user.click(screen.getByTestId("commodity-form-save-draft"))
    expect(await screen.findByTestId("draft-save-error")).toHaveTextContent("kettle.png")
    expect(fake.uploads).toBe(1)
    fake.failAttach = null
    await user.click(screen.getByTestId("draft-save-retry"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(fake.uploads).toBe(1)
    expect(fake.files[0].links).toEqual([
      expect.objectContaining({ entity_type: "draft", entity_id: fake.drafts[0].id }),
    ])
  })
})

// The files of a draft that "Save as a new draft" could not link belong to
// that visit of the form. A form opened later for another item must not link
// them to its own draft.
describe("<CommodityFormDialog /> files carried to a new draft", () => {
  let harness: { close: () => void; openNew: () => void } | null = null

  // CarryHarness opens draft d1, and lets the test close the form and open
  // it again for a new item, as the Items page does.
  function CarryHarness() {
    const [state, setState] = useState<{ open: boolean; resume?: string }>({
      open: true,
      resume: "d1",
    })
    useEffect(() => {
      harness = {
        close: () => setState({ open: false }),
        openNew: () => setState({ open: true }),
      }
    }, [])
    return (
      <GroupProvider>
        <CommodityFormDialog
          open={state.open}
          onOpenChange={(open) => setState((prev) => ({ ...prev, open }))}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={vi.fn().mockResolvedValue({ id: "new-item" }) as never}
          draftKey={DRAFT_KEY}
          serverDrafts
          resumeDraftId={state.resume}
        />
      </GroupProvider>
    )
  }

  it.each(["discarded", "closed by the page"])(
    "forgets them once the form is %s",
    async (closing) => {
      const fake = draftHandlers.backend(SLUG, [
        { id: "d1", revision: 1, values: { name: "Lamp", short_name: "Lamp" } },
      ])
      fake.files.push({
        id: "file-photo",
        path: "lamp",
        ext: ".jpg",
        size_bytes: 10,
        category: "images",
        tags: [],
        links: [
          {
            id: "link-1",
            file_id: "file-photo",
            entity_type: "draft",
            entity_id: "d1",
            meta: "images",
          },
        ],
      })
      server.use(...groupHandlers.list(groups), ...fake.handlers)
      setAccessToken("good-token")
      const user = userEvent.setup()
      renderWithProviders({
        initialPath: `/g/${SLUG}/commodities/new`,
        routes: <Route path="/g/:groupSlug/commodities/new" element={<CarryHarness />} />,
      })
      const name = await screen.findByLabelText(/^Name$/i)
      await waitFor(() => expect(name).toHaveValue("Lamp"))

      // A conflict, then a new draft whose link to the old photo fails.
      fake.drafts[0].revision = 2
      await user.type(name, " (mine)")
      await user.click(screen.getByLabelText(/Save as draft/i))
      await user.click(screen.getByTestId("commodity-form-save-draft"))
      await screen.findByTestId("draft-save-conflict")
      fake.failAttach = 500
      await user.click(screen.getByTestId("draft-conflict-save-new"))
      expect(await screen.findByTestId("draft-save-error")).toBeInTheDocument()
      fake.failAttach = null

      if (closing === "discarded") {
        await user.click(screen.getByTestId("commodity-form-cancel"))
        await user.click(await screen.findByTestId("commodity-form-close-confirm-discard"))
      } else {
        act(() => harness?.close())
      }
      await waitFor(() => expect(screen.queryByLabelText(/^Name$/i)).not.toBeInTheDocument())

      act(() => harness?.openNew())
      await user.type(await fillManually(user), "Chair")
      await user.click(screen.getByLabelText(/Save as draft/i))
      await user.click(screen.getByTestId("commodity-form-save-draft"))
      await waitFor(() => expect(fake.drafts).toHaveLength(3))
      await waitFor(() => expect(screen.queryByLabelText(/^Name$/i)).not.toBeInTheDocument())
      const chair = fake.drafts[2]
      expect(chair.values).toMatchObject({ name: "Chair" })
      expect(fake.files[0].links.map((l) => l.entity_id)).not.toContain(chair.id)
    }
  )
})
