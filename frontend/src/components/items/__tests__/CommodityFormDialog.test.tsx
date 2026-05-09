import { describe, expect, it, vi } from "vitest"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { renderWithProviders } from "@/test/render"

const areas = [
  { id: "a1", name: "Garage", location_id: "l1" },
  { id: "a2", name: "Kitchen", location_id: "l1" },
]

// In create mode the wizard now opens on the "Fill with AI"
// placeholder step (#1540 tracker). Every existing test that
// interacts with Basics needs one Next click upfront — the AI step
// has no fields, so validation passes immediately. Edit mode skips
// the AI step entirely (the wizard restarts at Basics) and those
// tests don't need walking.
async function walkPastAi(user: ReturnType<typeof userEvent.setup>) {
  // Radix Dialog renders into a portal; the AI step isn't queryable
  // synchronously after render, so wait for it before clicking. Used
  // exclusively from create-mode tests — edit mode skips the AI step
  // and never lands here.
  await screen.findByTestId("commodity-form-ai-step")
  await user.click(screen.getByTestId("commodity-form-next"))
  await screen.findByLabelText(/^Name$/i)
}

// Type/Area/Status are now Radix Select primitives (button trigger +
// portal-rendered listbox). userEvent.selectOptions() doesn't work
// because there is no <select> element. userEvent.click on the
// trigger doesn't fully open Radix's listbox under jsdom either, so
// we drive it with keyboard activation (Enter on a focused trigger
// opens, ArrowDown navigates, Enter picks).
async function pickSelect(
  user: ReturnType<typeof userEvent.setup>,
  triggerLabel: RegExp,
  optionLabel: RegExp
) {
  const trigger = screen.getByRole("combobox", { name: triggerLabel })
  trigger.focus()
  await user.keyboard("{Enter}")
  const listbox = await screen.findByRole("listbox")
  const option = within(listbox).getByRole("option", { name: optionLabel })
  await user.click(option)
}

describe("<CommodityFormDialog />", () => {
  it("renders the AI step on first open in create mode", async () => {
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    expect(await screen.findByTestId("commodity-form-ai-step")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-form-ai-coming-soon")).toBeInTheDocument()
  })

  it("walks past the AI step into Basics on Next", async () => {
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    await user.click(await screen.findByTestId("commodity-form-next"))
    expect(await screen.findByLabelText(/^Name$/i)).toBeInTheDocument()
  })

  it("blocks Next when required basics fields are missing", async () => {
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    await walkPastAi(user)
    await user.click(screen.getByTestId("commodity-form-next"))
    // Stayed on basics — purchase-step heading shouldn't appear.
    await waitFor(() => expect(screen.getAllByText(/Required|Pick/i).length).toBeGreaterThan(0))
  })

  // TODO: re-enable once we have a Radix-Select-friendly userEvent
  // path in JSDOM. Trigger.click + keyboard activation both fail to
  // mount the portal listbox under jsdom; same pattern as the three
  // skipped cases below — covered by Playwright e2e in the meantime.
  it.skip("walks through three steps and submits with mapped values", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          defaultCurrency="USD"
          onSubmit={onSubmit}
        />
      ),
    })
    await walkPastAi(user)
    await user.type(await screen.findByLabelText(/^Name$/i), "Couch")
    await user.type(screen.getByLabelText(/^Short name$/i), "Couch")
    // Type select uses a native <select>; selectOptions targets the
    // renderered option text from the type catalog.
    await pickSelect(user, /^Type$/i, /^Furniture$/i)
    await pickSelect(user, /^Area$/i, /^Garage$/i)
    // Tick the draft toggle so the schema's whenNotDraft block doesn't
    // require purchase_date + the price triad — keeps this test focused
    // on step navigation rather than every field.
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-next"))
    // Purchase step: just advance — every purchase field is optional.
    await waitFor(() => expect(screen.getByLabelText(/Purchase date/i)).toBeInTheDocument())
    await user.click(screen.getByTestId("commodity-form-next"))
    // Walk through Warranty (stub) → Extras → Files (final).
    await screen.findByTestId("commodity-form-warranty-step")
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-tags-input")
    await user.click(screen.getByTestId("commodity-form-next"))
    await waitFor(() => expect(screen.getByTestId("commodity-form-submit")).toBeInTheDocument())
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const [arg] = onSubmit.mock.calls[0]
    expect(arg).toMatchObject({
      name: "Couch",
      short_name: "Couch",
      type: "furniture",
      area_id: "a1",
      status: "in_use",
      count: 1,
      original_price_currency: "USD",
      draft: true,
    })
  })

  it.skip("adds and removes tags via the chip input on the extras step", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          defaultCurrency="USD"
          onSubmit={onSubmit}
        />
      ),
    })
    // Walk to extras step by filling required basics + skipping purchase.
    await walkPastAi(user)
    await user.type(await screen.findByLabelText(/^Name$/i), "Stand")
    await user.type(screen.getByLabelText(/^Short name$/i), "Stand")
    await pickSelect(user, /^Type$/i, /^Furniture$/i)
    await pickSelect(user, /^Area$/i, /^Garage$/i)
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByLabelText(/Purchase date/i)
    // Step through Purchase → Warranty (stub) → Extras.
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-warranty-step")
    await user.click(screen.getByTestId("commodity-form-next"))
    // Tag chip input lives on the extras step.
    const tagsInput = await screen.findByTestId("commodity-tags-input")
    await user.type(tagsInput, "office,wood,")
    await waitFor(() => expect(screen.getAllByTestId("commodity-tags-chip").length).toBe(2))
    // Remove the first chip.
    const firstChip = screen.getAllByTestId("commodity-tags-chip")[0]
    await user.click(within(firstChip).getByRole("button", { name: /remove/i }))
    await waitFor(() => expect(screen.getAllByTestId("commodity-tags-chip").length).toBe(1))
  })

  it.skip("steps Back from purchase to basics", async () => {
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    await walkPastAi(user)
    await user.type(await screen.findByLabelText(/^Name$/i), "X")
    await user.type(screen.getByLabelText(/^Short name$/i), "X")
    await pickSelect(user, /^Type$/i, /^Other$/i)
    await pickSelect(user, /^Area$/i, /^Garage$/i)
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByLabelText(/Purchase date/i)
    // Back returns to basics; the Name field is still populated.
    await user.click(screen.getByRole("button", { name: /back/i }))
    expect(await screen.findByLabelText(/^Name$/i)).toHaveValue("X")
  })

  it("renders the status field in edit mode and prefills from initial values", async () => {
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="edit"
          initialValues={{
            id: "c1",
            name: "Existing",
            type: "electronics",
            area_id: "a1",
            status: "sold",
            count: 2,
          }}
          areas={areas}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    expect(await screen.findByLabelText(/^Name$/i)).toHaveValue("Existing")
    expect(screen.getByLabelText(/^Quantity$/i)).toHaveValue(2)
    // Status select only renders in edit mode — the create flow defaults
    // every new commodity to in_use.
    // Radix Select trigger reflects the current value as its accessible
    // text content rather than a `value` attribute.
    expect(screen.getByLabelText(/^Status$/i)).toHaveTextContent(/Sold/i)
  })

  it("removes a tag chip with Backspace on an empty input", async () => {
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="edit"
          initialValues={{
            id: "c1",
            name: "x",
            short_name: "x",
            type: "other",
            area_id: "a1",
            status: "in_use",
            count: 1,
            tags: ["one", "two"],
            draft: true,
          }}
          areas={areas}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    // Skip basics → purchase → warranty (stub) to reach extras.
    await user.click(await screen.findByTestId("commodity-form-next"))
    await screen.findByLabelText(/Purchase date/i)
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-warranty-step")
    await user.click(screen.getByTestId("commodity-form-next"))
    const input = await screen.findByTestId("commodity-tags-input")
    expect(screen.getAllByTestId("commodity-tags-chip").length).toBe(2)
    await user.click(input)
    await user.keyboard("{Backspace}")
    await waitFor(() => expect(screen.getAllByTestId("commodity-tags-chip").length).toBe(1))
  })

  it("rehydrates form values from localStorage on open (draft persistence)", async () => {
    const user = userEvent.setup()
    const draftKey = "commodity-draft:test:create"
    window.localStorage.setItem(
      draftKey,
      JSON.stringify({
        name: "Persisted",
        short_name: "Pers",
        type: "furniture",
        area_id: "a1",
        status: "in_use",
        count: "3",
        original_price: "",
        original_price_currency: "USD",
        converted_original_price: "",
        current_price: "",
        serial_number: "",
        extra_serial_numbers: [],
        part_numbers: [],
        tags: [],
        purchase_date: "",
        urls: [],
        comments: "",
        draft: true,
      })
    )
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          defaultCurrency="USD"
          onSubmit={async () => {}}
          draftKey={draftKey}
        />
      ),
    })
    await walkPastAi(user)
    expect(await screen.findByLabelText(/^Name$/i)).toHaveValue("Persisted")
    expect(screen.getByLabelText(/^Quantity$/i)).toHaveValue(3)
    // Discard resets the form to defaults — the persisted "Persisted"
    // name is gone, replaced by an empty input.
    await user.click(screen.getByTestId("commodity-form-discard-draft"))
    await waitFor(() => expect(screen.getByLabelText(/^Name$/i)).toHaveValue(""))
    // localStorage may have been re-populated by the auto-save tick
    // that fires when reset(defaults) runs — but the persisted value
    // is no longer "Persisted".
    const after = window.localStorage.getItem(draftKey)
    if (after) {
      expect(JSON.parse(after).name).toBe("")
    }
  })

  // Issue #1554: bundle commodities (count > 1) cannot carry warranty
  // / loan / service. The dialog surfaces a banner the moment the
  // user enters a count > 1, and the Warranty step disables its
  // inputs.
  it("shows the bundle banner when count > 1", async () => {
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    await walkPastAi(user)
    const countInput = await screen.findByLabelText(/^Quantity$/i)
    expect(screen.queryByTestId("commodity-form-bundle-banner")).not.toBeInTheDocument()
    await user.clear(countInput)
    await user.type(countInput, "12")
    await waitFor(() =>
      expect(screen.getByTestId("commodity-form-bundle-banner")).toBeInTheDocument()
    )
  })

  it("disables the Warranty step inputs when count > 1", async () => {
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="edit"
          initialValues={{
            id: "c1",
            name: "Pack of bulbs",
            short_name: "bulbs",
            type: "other",
            area_id: "a1",
            status: "in_use",
            count: 12,
            draft: true,
          }}
          areas={areas}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    // Walk to warranty step.
    await user.click(await screen.findByTestId("commodity-form-next"))
    await screen.findByLabelText(/Purchase date/i)
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-warranty-step")

    expect(screen.getByTestId("commodity-form-warranty-expires-at")).toBeDisabled()
    expect(screen.getByTestId("commodity-form-warranty-notes")).toBeDisabled()
  })

  it("has no axe violations", async () => {
    const { container } = renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    await screen.findByTestId("commodity-form-ai-step")
    expect(await axe(container)).toHaveNoViolations()
  })
})
