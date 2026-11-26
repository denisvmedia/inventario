import { useEffect, useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { delay, http, HttpResponse } from "msw"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import type { CreateCommodityRequest } from "@/features/commodities/api"
import { promoteDraft } from "@/features/drafts/api"
import type { DraftRef } from "@/features/drafts/hooks"
import { itemDraftOf } from "@/features/drafts/save"
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
import { pickRadixSelect } from "@/test/radix"
import { renderWithProviders } from "@/test/render"
import { toastSpies } from "@/test/global-mocks"
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

// renderClosableForm renders a new-item form whose host can close and open
// it, as browser Back does on the Items page.
function renderClosableForm(
  onSubmit: (values: CreateCommodityRequest, draft?: DraftRef) => Promise<unknown>
) {
  setAccessToken("good-token")
  const host = { setOpen: (_open: boolean) => {} }
  function Host() {
    const [open, setOpenState] = useState(true)
    useEffect(() => {
      host.setOpen = setOpenState
    }, [])
    return (
      <CommodityFormDialog
        open={open}
        onOpenChange={setOpenState}
        mode="create"
        areas={areas}
        locations={locations}
        defaultCurrency="USD"
        onSubmit={onSubmit as never}
        draftKey={DRAFT_KEY}
        serverDrafts
      />
    )
  }
  renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/new`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/new"
        element={
          <GroupProvider>
            <Host />
          </GroupProvider>
        }
      />
    ),
  })
  return {
    close: () => act(() => host.setOpen(false)),
    reopen: () => act(() => host.setOpen(true)),
  }
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
    // The promotion carries the values and the revision the form loaded;
    // Add item leaves the saved draft as it was (#1626). It names its key,
    // kept for a retry until it succeeds.
    expect(draft).toEqual({
      id: "d1",
      revision: 3,
      promotionKey: expect.stringMatching(/^promote:/),
    })
    expect(fake.drafts[0]).toMatchObject({ revision: 3, values: { draft: true } })
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
      title: "lamp",
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
        title: "lamp",
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

// Add item goes through the server draft (#1626): every staged file is
// uploaded to the draft first, and the draft becomes the item only once it
// holds them all. Until then the form stays open with each file's state.
describe("<CommodityFormDialog /> Add item through the draft", () => {
  // promoteThrough submits the way the Items page does: it promotes the draft
  // the form hands over.
  function promoteThrough() {
    return vi.fn(async (values: CreateCommodityRequest, draft?: DraftRef) => {
      if (!draft?.promotionKey) throw new Error("Add item must hand over the draft")
      return promoteDraft(draft.id, draft.revision, draft.promotionKey, values)
    })
  }

  const complete = {
    id: "d1",
    revision: 1,
    values: { name: "Kettle", short_name: "Kettle", type: "furniture" },
  }

  async function stage(user: ReturnType<typeof userEvent.setup>, files: File[]) {
    const input = screen
      .getByTestId("commodity-files-dropzone")
      .querySelector("input[type=file]") as HTMLInputElement
    await user.upload(input, files)
  }

  const pendingRows = () => screen.queryAllByTestId("commodity-files-pending")

  it("uploads every file to the draft before it becomes the item", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.nextUploads = [
      { name: "receipt.pdf", size: 3 },
      { name: "kettle.png", size: 3 },
    ]
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const promote = promoteThrough()
    let linkedWhenPromoted: string[] = []
    const onSubmit = vi.fn(async (values: CreateCommodityRequest, draft?: DraftRef) => {
      linkedWhenPromoted = fake.files.flatMap((f) => f.links.map((l) => l.entity_type))
      return promote(values, draft)
    })
    renderForm({ onSubmit: onSubmit as never })
    await user.type(await fillManually(user), "Kettle")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    await walkToFiles(user)
    await stage(user, [
      new File(["pdf"], "receipt.pdf", { type: "application/pdf" }),
      new File(["png"], "kettle.png", { type: "image/png" }),
    ])
    const [receipt] = pendingRows()
    const title = within(receipt).getByLabelText("Title of receipt.pdf")
    expect(title).toHaveValue("receipt")
    await user.clear(title)
    await user.type(title, "Shop receipt")
    await user.click(screen.getByTestId("commodity-form-submit"))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(linkedWhenPromoted).toEqual(["draft", "draft"])
    const [, draft] = onSubmit.mock.calls[0]
    expect(draft).toEqual({
      id: fake.drafts[0].id,
      revision: 1,
      promotionKey: expect.stringMatching(/^promote:/),
    })
    await waitFor(() => expect(pendingRows()).toHaveLength(0))
    const itemId = `item-from-${fake.drafts[0].id}`
    expect(fake.drafts[0].status).toBe("promoted")
    expect(fake.files.map((f) => [f.title, f.links.map((l) => l.entity_id)])).toEqual([
      ["Shop receipt", [itemId]],
      ["kettle", [itemId]],
    ])
    expect(fake.uploads).toBe(2)
  })

  it("adds nothing while a file fails, and Retry carries on without picking it again", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    fake.nextUploads = [{ name: "label.png", size: 3 }]
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    // The first upload, of receipt.png, fails; label.png goes through.
    server.use(
      http.post(
        apiUrl(`/g/${SLUG}/uploads/file`),
        () => HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 }),
        { once: true }
      )
    )
    const user = userEvent.setup()
    const onSubmit = promoteThrough()
    const { onOpenChange } = await openResumedOn("d1", "commodity-form-purchase-step", {
      onSubmit: onSubmit as never,
    })
    await walkToFiles(user, ["warranty", "tags", "files"])
    await stage(user, [
      new File(["png"], "receipt.png", { type: "image/png" }),
      new File(["png"], "label.png", { type: "image/png" }),
    ])
    await user.click(screen.getByTestId("commodity-form-submit"))

    const alert = await screen.findByTestId("draft-save-error")
    expect(alert).toHaveTextContent("Item not added")
    expect(alert).toHaveTextContent("Not uploaded: receipt.png")
    expect(onSubmit).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalled()
    // The failed file stays staged, marked; the one that made it is listed
    // with the draft's files.
    const rows = pendingRows()
    expect(rows).toHaveLength(1)
    expect(rows[0]).toHaveAttribute("data-status", "failed")
    expect(rows[0]).toHaveTextContent("receipt.png")
    expect(rows[0]).toHaveTextContent("Upload failed")
    expect(screen.getByTestId("draft-saved-files")).toHaveTextContent("label.png")
    expect(screen.getByTestId("commodity-form-submit")).toBeEnabled()

    fake.nextUploads = [{ name: "receipt.png", size: 3 }]
    await user.click(screen.getByTestId("draft-save-retry"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    // The links move with the promotion, which the call only starts.
    await onSubmit.mock.results[0].value
    expect(fake.uploads).toBe(2)
    expect(fake.files.map((f) => f.links.map((l) => l.entity_id))).toEqual([
      ["item-from-d1"],
      ["item-from-d1"],
    ])
    expect(screen.queryByTestId("draft-save-error")).not.toBeInTheDocument()
  })

  it("keeps the files on the draft when the promotion fails, and Retry only promotes", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    fake.nextUploads = [{ name: "receipt.png", size: 3 }]
    fake.failPromote = 503
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const onSubmit = promoteThrough()
    await openResumedOn("d1", "commodity-form-purchase-step", { onSubmit: onSubmit as never })
    await walkToFiles(user, ["warranty", "tags", "files"])
    await stage(user, [new File(["png"], "receipt.png", { type: "image/png" })])
    await user.click(screen.getByTestId("commodity-form-submit"))

    expect(await screen.findByTestId("commodity-form-error")).toBeInTheDocument()
    expect(fake.drafts[0].status).toBe("open")
    await waitFor(() => expect(pendingRows()).toHaveLength(0))
    expect(screen.getByTestId("draft-saved-files")).toHaveTextContent("receipt.png")

    fake.failPromote = null
    await user.click(screen.getByTestId("commodity-form-error-retry"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(fake.drafts[0].status).toBe("promoted"))
    expect(fake.uploads).toBe(1)
    expect(fake.files[0].links.map((l) => l.entity_id)).toEqual(["item-from-d1"])
    const keys = onSubmit.mock.calls.map(([, draft]) => draft?.promotionKey)
    expect(keys[1]).toBe(keys[0])
  })

  it("gets the item a lost promotion made when Add item is clicked again", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    fake.loseResponse.promote = true
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const onSubmit = promoteThrough()
    await openResumedOn("d1", "commodity-form-purchase-step", { onSubmit: onSubmit as never })
    await walkToFiles(user, ["warranty", "tags", "files"])
    await user.click(screen.getByTestId("commodity-form-submit"))
    expect(await screen.findByTestId("commodity-form-error")).toBeInTheDocument()
    expect(fake.drafts[0].status).toBe("promoted")

    await user.click(screen.getByTestId("commodity-form-error-retry"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(2))
    await expect(onSubmit.mock.results[1].value).resolves.toMatchObject({ id: "item-from-d1" })
    expect(screen.queryByTestId("draft-save-promoted")).not.toBeInTheDocument()
  })

  it("shows each file's state and locks the form while the files upload", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    let release = () => {}
    const held = new Promise<void>((resolve) => {
      release = resolve
    })
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    // The second upload waits until the test lets it go.
    let calls = 0
    server.use(
      http.post(apiUrl(`/g/${SLUG}/uploads/file`), async () => {
        calls++
        if (calls === 2) await held
        return undefined
      })
    )
    const user = userEvent.setup()
    const onSubmit = promoteThrough()
    const { onOpenChange } = await openResumedOn("d1", "commodity-form-purchase-step", {
      onSubmit: onSubmit as never,
    })
    await walkToFiles(user, ["warranty", "tags", "files"])
    await stage(user, [
      new File(["png"], "a.png", { type: "image/png" }),
      new File(["png"], "b.png", { type: "image/png" }),
    ])
    await user.click(screen.getByTestId("commodity-form-submit"))

    await waitFor(() => expect(pendingRows()[1]).toHaveAttribute("data-status", "uploading"))
    expect(pendingRows()[0]).toHaveAttribute("data-status", "uploaded")
    expect(pendingRows()[0]).toHaveTextContent("a.png uploaded")
    expect(screen.getByTestId("commodity-form-submit")).toBeDisabled()
    expect(screen.getByTestId("commodity-form-submit")).toHaveTextContent("Uploading 1 of 2…")
    expect(screen.getByTestId("commodity-form-progress")).toHaveTextContent("Uploading 1 of 2…")
    expect(screen.getByTestId("commodity-form-cancel")).toBeDisabled()
    expect(screen.getByLabelText("Title of a.png")).toBeDisabled()
    expect(within(pendingRows()[0]).queryByRole("button", { name: "Delete" })).toBeNull()
    expect(onSubmit).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalled()

    release()
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
  })

  it("discards the draft Add item made and trashes its uploads when the form is discarded", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.nextUploads = [{ name: "label.png", size: 3 }]
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    // receipt.png fails to upload, so nothing is added.
    server.use(
      http.post(
        apiUrl(`/g/${SLUG}/uploads/file`),
        () => HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 }),
        { once: true }
      )
    )
    const user = userEvent.setup()
    const { onOpenChange } = renderForm({ onSubmit: promoteThrough() as never })
    await user.type(await fillManually(user), "Kettle")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    await walkToFiles(user)
    await stage(user, [
      new File(["png"], "receipt.png", { type: "image/png" }),
      new File(["png"], "label.png", { type: "image/png" }),
    ])
    await user.click(screen.getByTestId("commodity-form-submit"))
    await screen.findByTestId("draft-save-error")
    expect(fake.drafts).toHaveLength(1)

    await user.click(screen.getByTestId("commodity-form-cancel"))
    await user.click(await screen.findByTestId("commodity-form-close-confirm-discard"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    // The draft went, and with it the link of label.png; the fake keeps the
    // file as the server would move it to Trash.
    expect(fake.drafts).toHaveLength(0)
    expect(fake.files.flatMap((f) => f.links)).toEqual([])
  })

  it("moves a file a failed save uploaded but did not attach to Trash on Discard", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    fake.nextUploads = [{ name: "receipt.png", size: 3 }]
    fake.failAttach = 500
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onOpenChange } = await openResumedOn("d1", "commodity-form-purchase-step", {
      onSubmit: promoteThrough() as never,
    })
    await walkToFiles(user, ["warranty", "tags", "files"])
    await stage(user, [new File(["png"], "receipt.png", { type: "image/png" })])
    await user.click(screen.getByTestId("commodity-form-submit"))
    expect(await screen.findByTestId("draft-save-error")).toHaveTextContent("receipt.png")
    const uploaded = fake.files[0].id

    await user.click(screen.getByTestId("commodity-form-cancel"))
    await user.click(await screen.findByTestId("commodity-form-close-confirm-discard"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(fake.trashed).toEqual([uploaded])
    // The resumed draft is the user's own: Discard leaves it as last saved.
    expect(fake.drafts).toHaveLength(1)
  })

  it("moves an uploaded file to Trash when its row is removed", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    fake.nextUploads = [{ name: "receipt.png", size: 3 }]
    fake.failAttach = 500
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    await openResumedOn("d1", "commodity-form-purchase-step", {
      onSubmit: promoteThrough() as never,
    })
    await walkToFiles(user, ["warranty", "tags", "files"])
    await stage(user, [new File(["png"], "receipt.png", { type: "image/png" })])
    await user.click(screen.getByTestId("commodity-form-submit"))
    await screen.findByTestId("draft-save-error")
    const uploaded = fake.files[0].id

    await user.click(within(pendingRows()[0]).getByRole("button", { name: "Delete" }))
    await waitFor(() => expect(fake.trashed).toEqual([uploaded]))
    expect(pendingRows()).toHaveLength(0)
  })

  it("leaves a file linked elsewhere where it is", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    fake.nextUploads = [{ name: "receipt.png", size: 3 }]
    fake.failAttach = 500
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const { onOpenChange } = await openResumedOn("d1", "commodity-form-purchase-step", {
      onSubmit: promoteThrough() as never,
    })
    await walkToFiles(user, ["warranty", "tags", "files"])
    await stage(user, [new File(["png"], "receipt.png", { type: "image/png" })])
    await user.click(screen.getByTestId("commodity-form-submit"))
    await screen.findByTestId("draft-save-error")
    // Another tab attached the upload to an item meanwhile.
    fake.files[0].links.push({
      id: "link-elsewhere",
      file_id: fake.files[0].id,
      entity_type: "commodity",
      entity_id: "item-9",
    })

    await user.click(screen.getByTestId("commodity-form-cancel"))
    await user.click(await screen.findByTestId("commodity-form-close-confirm-discard"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(fake.trashed).toEqual([])
    expect(fake.files).toHaveLength(1)
  })

  // failNewItem runs Add item on a new form whose first file fails to
  // upload: the draft Add item made holds label.png, and no item exists.
  async function failNewItem(user: ReturnType<typeof userEvent.setup>) {
    const fake = draftHandlers.backend(SLUG)
    fake.nextUploads = [{ name: "label.png", size: 3 }]
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    server.use(
      http.post(
        apiUrl(`/g/${SLUG}/uploads/file`),
        () => HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 }),
        { once: true }
      )
    )
    const rendered = renderForm({ onSubmit: promoteThrough() as never })
    await user.type(await fillManually(user), "Kettle")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    await walkToFiles(user)
    await stage(user, [
      new File(["png"], "receipt.png", { type: "image/png" }),
      new File(["png"], "label.png", { type: "image/png" }),
    ])
    await user.click(screen.getByTestId("commodity-form-submit"))
    await screen.findByTestId("draft-save-error")
    expect(fake.drafts).toHaveLength(1)
    return { fake, ...rendered }
  }

  async function discard(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByTestId("commodity-form-cancel"))
    await user.click(await screen.findByTestId("commodity-form-close-confirm-discard"))
  }

  it("keeps the draft a failed Add item made once the user saved it elsewhere", async () => {
    const user = userEvent.setup()
    const { fake, onOpenChange } = await failNewItem(user)
    // The user opened the draft from the Inbox and saved it there.
    fake.drafts[0].revision = 2

    await discard(user)
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(fake.drafts).toHaveLength(1)
    expect(fake.files[0].links).toEqual([
      expect.objectContaining({ entity_type: "draft", entity_id: fake.drafts[0].id }),
    ])
    expect(toastSpies.error).not.toHaveBeenCalled()
  })

  it("says the item was added when the draft became one elsewhere before Discard", async () => {
    const user = userEvent.setup()
    const { fake, onOpenChange } = await failNewItem(user)
    fake.drafts[0].status = "promoted"
    fake.drafts[0].promoted_commodity_id = "item-9"

    await discard(user)
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(fake.drafts).toHaveLength(1)
    expect(toastSpies.info.mock.calls.map(([message]) => message)).toEqual([
      "The item was already added to your inventory.",
    ])
    expect(toastSpies.error).not.toHaveBeenCalled()
  })

  it("leaves a resumed draft as saved when Add item fails, with the uploaded file on it", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    fake.nextUploads = [{ name: "label.png", size: 3 }]
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    server.use(
      http.post(
        apiUrl(`/g/${SLUG}/uploads/file`),
        () => HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 }),
        { once: true }
      )
    )
    const user = userEvent.setup()
    const { onOpenChange } = await openResumedOn("d1", "commodity-form-purchase-step", {
      onSubmit: promoteThrough() as never,
    })
    const name = await toBasics(user)
    await user.clear(name)
    await user.type(name, "Electric kettle")
    await walkToFiles(user)
    await stage(user, [
      new File(["png"], "receipt.png", { type: "image/png" }),
      new File(["png"], "label.png", { type: "image/png" }),
    ])
    await user.click(screen.getByTestId("commodity-form-submit"))
    await screen.findByTestId("draft-save-error")
    // Add item does not save the values; the promotion would carry them.
    expect(fake.drafts[0]).toMatchObject({ revision: 1, values: { name: "Kettle" } })
    expect(screen.getByTestId("draft-saved-files")).toHaveTextContent("label.png")

    await discard(user)
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(fake.drafts[0]).toMatchObject({
      status: "open",
      revision: 1,
      values: { name: "Kettle" },
    })
    expect(fake.files[0].links).toEqual([
      expect.objectContaining({ entity_type: "draft", entity_id: "d1" }),
    ])
  })

  it("adds no item when the dialog closes during the uploads, and the next opening goes on", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.nextUploads = [
      { name: "a.png", size: 3 },
      { name: "b.png", size: 3 },
    ]
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    let release = () => {}
    const held = new Promise<void>((resolve) => {
      release = resolve
    })
    // The first upload waits until the test lets it go.
    let calls = 0
    server.use(
      http.post(apiUrl(`/g/${SLUG}/uploads/file`), async () => {
        calls++
        if (calls === 1) await held
        return undefined
      })
    )
    const user = userEvent.setup()
    const onSubmit = promoteThrough()
    const { close, reopen } = renderClosableForm(onSubmit)
    const files = () => [
      new File(["png"], "a.png", { type: "image/png" }),
      new File(["png"], "b.png", { type: "image/png" }),
    ]
    await user.type(await fillManually(user), "Kettle")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    await walkToFiles(user)
    await stage(user, files())
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(pendingRows()[0]).toHaveAttribute("data-status", "uploading"))
    // The form knows its draft before the first upload ends, so a Discard
    // after a reload still finds it.
    expect(itemDraftOf(DRAFT_KEY)).toBe(fake.drafts[0].id)

    close()
    release()
    await waitFor(() => expect(fake.files[0]?.links).toHaveLength(1))

    // The next opening has the input; Add item attaches the rest and adds
    // the item, without uploading a.png again.
    reopen()
    const name = await fillManually(user)
    expect(name).toHaveValue("Kettle")
    await walkToFiles(user)
    await stage(user, files())
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    await onSubmit.mock.results[0].value
    expect(fake.drafts[0].status).toBe("promoted")
    expect(fake.uploads).toBe(2)
    expect(fake.files.map((f) => f.links.map((l) => l.entity_id))).toEqual([
      [`item-from-${fake.drafts[0].id}`],
      [`item-from-${fake.drafts[0].id}`],
    ])
  })

  it("stops an Add item the dialog was closed and opened again during", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.nextUploads = [
      { name: "a.png", size: 3 },
      { name: "b.png", size: 3 },
    ]
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    let release = () => {}
    const held = new Promise<void>((resolve) => {
      release = resolve
    })
    let calls = 0
    server.use(
      http.post(apiUrl(`/g/${SLUG}/uploads/file`), async () => {
        calls++
        if (calls === 1) await held
        return undefined
      })
    )
    const user = userEvent.setup()
    const onSubmit = promoteThrough()
    const { close, reopen } = renderClosableForm(onSubmit)
    const files = () => [
      new File(["png"], "a.png", { type: "image/png" }),
      new File(["png"], "b.png", { type: "image/png" }),
    ]
    await user.type(await fillManually(user), "Kettle")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    await walkToFiles(user)
    await stage(user, files())
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(pendingRows()[0]).toHaveAttribute("data-status", "uploading"))

    // Closed and open again before the first upload answers. The form
    // stays locked while the earlier Add item runs.
    close()
    reopen()
    expect(await fillManually(user)).toHaveValue("Kettle")
    const next = screen.getByTestId("commodity-form-next")
    expect(next).toBeDisabled()
    release()
    // The earlier Add item ends after a.png, and nothing was promoted.
    await waitFor(() => expect(next).toBeEnabled())
    expect(onSubmit).not.toHaveBeenCalled()
    expect(fake.promotions).toHaveLength(0)
    expect(fake.drafts[0].status).toBe("open")
    expect(calls).toBe(1)

    // This opening's Add item finishes it, without uploading a.png again.
    await walkToFiles(user)
    await stage(user, files())
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    await expect(onSubmit.mock.results[0].value).resolves.toMatchObject({
      id: `item-from-${fake.drafts[0].id}`,
    })
    expect(fake.promotions).toHaveLength(1)
    expect(fake.uploads).toBe(2)
  })

  it("gets the item a lost promotion made, also after a reload", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.loseResponse.promote = true
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    renderForm({ onSubmit: promoteThrough() as never })
    await user.type(await fillManually(user), "Kettle")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    await walkToFiles(user)
    await user.click(screen.getByTestId("commodity-form-submit"))
    expect(await screen.findByTestId("commodity-form-error")).toBeInTheDocument()
    expect(fake.drafts[0].status).toBe("promoted")

    // A reload: the form comes back from this device with the same keys.
    cleanup()
    const onSubmit = promoteThrough()
    renderForm({ onSubmit: onSubmit as never })
    expect(await fillManually(user)).toHaveValue("Kettle")
    await walkToFiles(user)
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    await expect(onSubmit.mock.results[0].value).resolves.toMatchObject({
      id: `item-from-${fake.drafts[0].id}`,
    })
    expect(screen.queryByTestId("draft-save-promoted")).not.toBeInTheDocument()
  })

  it("does not add the item when Enter is pressed in a file title", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    const onSubmit = promoteThrough()
    renderForm({ onSubmit: onSubmit as never })
    await user.type(await fillManually(user), "Kettle")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    await walkToFiles(user)
    await stage(user, [new File(["pdf"], "receipt.pdf", { type: "application/pdf" })])
    const title = within(pendingRows()[0]).getByLabelText("Title of receipt.pdf")
    const submitted = vi.fn((e: Event) => e.preventDefault())
    title.closest("form")?.addEventListener("submit", submitted)
    await user.clear(title)
    await user.type(title, "Shop receipt{Enter}")
    expect(title).toHaveValue("Shop receipt")
    expect(submitted).not.toHaveBeenCalled()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("cuts a long file title at 255 characters without splitting one", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    renderForm({ onSubmit: promoteThrough() as never })
    await user.type(await fillManually(user), "Kettle")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    await walkToFiles(user)
    await stage(user, [new File(["pdf"], "receipt.pdf", { type: "application/pdf" })])
    const title = within(pendingRows()[0]).getByLabelText("Title of receipt.pdf")
    fireEvent.change(title, { target: { value: "📦".repeat(300) } })
    expect(title).toHaveValue("📦".repeat(255))
  })

  // savedLamp is a file batch intake saved with draft d1, titled and tagged
  // by the scan (#1626).
  function savedLamp(
    fake: ReturnType<typeof draftHandlers.backend>,
    extra: { description?: string; tags?: string[] } = {}
  ) {
    fake.files.push({
      id: "file-lamp",
      title: "Lamp invoice",
      path: "IMG_0042",
      ext: ".jpg",
      size_bytes: 10,
      category: "images",
      tags: extra.tags ?? ["invoice"],
      ...(extra.description !== undefined ? { description: extra.description } : {}),
      links: [
        {
          id: "link-1",
          file_id: "file-lamp",
          entity_type: "draft",
          entity_id: "d1",
          meta: "images",
        },
      ],
    })
  }

  it("shows the title and tags of a file saved with the draft and saves a change to them", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    savedLamp(fake)
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    await openResumedOn("d1", "commodity-form-purchase-step")
    await walkToFiles(user, ["warranty", "tags", "files"])

    const title = await screen.findByTestId("draft-saved-file-title-file-lamp")
    expect(title).toHaveValue("Lamp invoice")
    const tags = screen.getByTestId("draft-saved-file-tags-file-lamp")
    expect(within(tags).getByTestId("draft-saved-file-tags-file-lamp-chip")).toHaveTextContent(
      /invoice/i
    )

    await user.clear(title)
    await user.type(title, "Desk lamp receipt")
    await user.tab()
    await waitFor(() => expect(fake.files[0].title).toBe("Desk lamp receipt"))
    expect(fake.files[0].tags).toEqual(["invoice"])

    await user.click(within(tags).getByRole("button", { name: /^remove invoice/i }))
    await waitFor(() => expect(fake.files[0].tags).toEqual([]))
    expect(fake.files[0].title).toBe("Desk lamp receipt")
    await waitFor(() =>
      expect(screen.getByTestId("draft-saved-file-title-file-lamp")).toHaveValue(
        "Desk lamp receipt"
      )
    )
  })

  it("keeps a saved file's description when its title changes", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    savedLamp(fake, { description: "Bought at the fair" })
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    await openResumedOn("d1", "commodity-form-purchase-step")
    await walkToFiles(user, ["warranty", "tags", "files"])
    const title = await screen.findByTestId("draft-saved-file-title-file-lamp")
    await user.clear(title)
    await user.type(title, "Desk lamp receipt")
    await user.tab()
    await waitFor(() => expect(fake.files[0].title).toBe("Desk lamp receipt"))
    expect(fake.files[0].description).toBe("Bought at the fair")

    // The second save sends the description the first one returned.
    await user.click(
      within(screen.getByTestId("draft-saved-file-tags-file-lamp")).getByRole("button", {
        name: /^remove invoice/i,
      })
    )
    await waitFor(() => expect(fake.files[0].tags).toEqual([]))
    expect(fake.files[0].description).toBe("Bought at the fair")
  })

  it("saves two quick changes to a saved file in the order they were made", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    savedLamp(fake, { tags: ["invoice", "warranty"] })
    let puts = 0
    let answered = 0
    const onResponse = ({ request }: { request: Request }) => {
      if (request.method === "PUT" && request.url.includes("/files/")) answered++
    }
    server.events.on("response:mocked", onResponse)
    server.use(
      ...groupHandlers.list(groups),
      // The first save is slow: sent at once, the second would land first
      // and the first, landing last, would undo it.
      http.put(apiUrl(`/g/${SLUG}/files/:id`), async () => {
        puts++
        if (puts === 1) await delay(150)
        return undefined
      }),
      ...fake.handlers
    )
    try {
      const user = userEvent.setup()
      await openResumedOn("d1", "commodity-form-purchase-step")
      await walkToFiles(user, ["warranty", "tags", "files"])
      const tags = await screen.findByTestId("draft-saved-file-tags-file-lamp")
      await user.click(within(tags).getByRole("button", { name: /^remove invoice/i }))
      await user.click(within(tags).getByRole("button", { name: /^remove warranty/i }))

      await waitFor(() => expect(answered).toBe(2))
      expect(fake.files[0].tags).toEqual([])
      expect(within(tags).queryAllByTestId("draft-saved-file-tags-file-lamp-chip")).toHaveLength(0)
    } finally {
      server.events.removeListener("response:mocked", onResponse)
    }
  })

  it("gives a saved file its name back when its title is cleared", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    savedLamp(fake)
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const user = userEvent.setup()
    await openResumedOn("d1", "commodity-form-purchase-step")
    await walkToFiles(user, ["warranty", "tags", "files"])
    const title = await screen.findByTestId("draft-saved-file-title-file-lamp")
    await user.clear(title)
    await user.tab()
    await waitFor(() => expect(fake.files[0].title).toBe("IMG_0042"))
  })

  it("shows the file's title again when saving it fails", async () => {
    const fake = draftHandlers.backend(SLUG, [complete])
    savedLamp(fake)
    server.use(
      ...groupHandlers.list(groups),
      http.put(apiUrl(`/g/${SLUG}/files/:id`), () =>
        HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
      ),
      ...fake.handlers
    )
    const user = userEvent.setup()
    await openResumedOn("d1", "commodity-form-purchase-step")
    await walkToFiles(user, ["warranty", "tags", "files"])
    const title = await screen.findByTestId("draft-saved-file-title-file-lamp")
    await user.clear(title)
    await user.type(title, "Mine")
    await user.tab()
    await waitFor(() => expect(toastSpies.error).toHaveBeenCalled())
    await waitFor(() => expect(title).toHaveValue("Lamp invoice"))
    expect(fake.files[0].title).toBe("Lamp invoice")
  })
})
