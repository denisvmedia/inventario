import { useState } from "react"
import { describe, expect, it, vi } from "vitest"
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { HttpError } from "@/lib/http"
import { renderWithProviders } from "@/test/render"
import { pickRadixSelect } from "@/test/radix"

const areas = [
  { id: "a1", name: "Garage", location_id: "l1" },
  { id: "a2", name: "Kitchen", location_id: "l1" },
]
const locations = [{ id: "l1", name: "Home" }]

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
  // and never lands here. Post-#1720 the AI step owns its own
  // primary actions inline; "Fill manually" carries a distinct
  // testid (`commodity-form-ai-fill-manually`) so the wizard footer's
  // `commodity-form-next` testid only appears on form steps.
  await screen.findByTestId("commodity-form-ai-step")
  await user.click(screen.getByTestId("commodity-form-ai-fill-manually"))
  await screen.findByLabelText(/^Name$/i)
}

// Type/Area/Status are now Radix Select primitives (button trigger +
// portal-rendered listbox). userEvent.selectOptions() doesn't work
// because there is no <select> element. Drive them through the
// shared @/test/radix helper, which clicks the trigger, picks the
// option inside the freshly-mounted listbox, and waits for the
// portal to unmount so sequential picks don't race.
async function pickSelect(
  user: ReturnType<typeof userEvent.setup>,
  triggerLabel: RegExp,
  optionLabel: RegExp
) {
  await pickRadixSelect(user, triggerLabel, { optionLabel })
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
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    expect(await screen.findByTestId("commodity-form-ai-step")).toBeInTheDocument()
    // Post-#1720 the AI step ships with a live dropzone instead of
    // the inert tracker line — assert the dropzone testid is mounted
    // so a future regression that hides it gets caught.
    expect(screen.getByTestId("commodity-form-ai-dropzone")).toBeInTheDocument()
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
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    await user.click(await screen.findByTestId("commodity-form-ai-fill-manually"))
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
          locations={locations}
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

  it("walks through three steps and submits with mapped values", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={onSubmit}
        />
      ),
    })
    await walkPastAi(user)
    await user.type(await screen.findByLabelText(/^Name$/i), "Couch")
    await pickSelect(user, /^Type$/i, /^Furniture$/i)
    // Save as a draft: this test is about step navigation, not about
    // the rules a reviewed item has to meet.
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
      status: "in_use",
      count: 1,
      original_price_currency: "USD",
      draft: true,
    })
  })

  it("adds and removes tags via the chip input on the extras step", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={onSubmit}
        />
      ),
    })
    // Walk to extras step by filling required basics + skipping purchase.
    await walkPastAi(user)
    await user.type(await screen.findByLabelText(/^Name$/i), "Stand")
    await pickSelect(user, /^Type$/i, /^Furniture$/i)
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

  it("steps Back from purchase to basics", async () => {
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    await walkPastAi(user)
    await user.type(await screen.findByLabelText(/^Name$/i), "X")
    await pickSelect(user, /^Type$/i, /^Other$/i)
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByLabelText(/Purchase date/i)
    // Back returns to basics; the Name field is still populated.
    await user.click(screen.getByRole("button", { name: /back/i }))
    expect(await screen.findByLabelText(/^Name$/i)).toHaveValue("X")
  })

  it("prefills edit mode from initial values", async () => {
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
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    expect(await screen.findByLabelText(/^Name$/i)).toHaveValue("Existing")
    expect(screen.getByLabelText(/^Quantity$/i)).toHaveValue(2)
    // Radix Select trigger reflects the current value as its accessible
    // text content rather than a `value` attribute.
    expect(screen.getByLabelText(/^Status$/i)).toHaveTextContent(/Sold/i)
  })

  describe("status in edit mode (#2915)", () => {
    // A sold item: its status date, note and sale price are not on the form.
    const sold = {
      id: "c1",
      name: "Bike",
      short_name: "Bike",
      type: "other" as const,
      status: "sold" as const,
      status_date: "2026-05-17",
      status_note: "Sold to Bob",
      sale_price: 99.5,
      count: 1,
      draft: false,
      purchase_date: "2024-03-01",
      original_price: 400,
      original_price_currency: "USD",
    }

    function renderSold(onSubmit = vi.fn().mockResolvedValue(undefined)) {
      renderWithProviders({
        children: (
          <CommodityFormDialog
            open
            onOpenChange={() => {}}
            mode="edit"
            initialValues={sold}
            areas={areas}
            locations={locations}
            defaultCurrency="USD"
            onSubmit={onSubmit}
          />
        ),
      })
      return onSubmit
    }

    it("shows the status without offering a change, and names Change Status", async () => {
      renderSold()
      const status = await screen.findByLabelText(/^Status$/i)
      expect(status).toHaveTextContent(/Sold/i)
      expect(status).toBeDisabled()
      expect(status).toHaveAccessibleDescription(/Change Status on the item page/i)
    })

    it("sends back the status date, note and sale price the item has", async () => {
      const user = userEvent.setup()
      const onSubmit = renderSold()
      const name = await screen.findByLabelText(/^Name$/i)
      await user.clear(name)
      await user.type(name, "Road bike")
      await user.click(screen.getByTestId("commodity-form-submit"))
      await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
      expect(onSubmit.mock.calls[0][0]).toMatchObject({
        name: "Road bike",
        status: "sold",
        status_date: "2026-05-17",
        status_note: "Sold to Bob",
        sale_price: 99.5,
      })
    })

    it("explains a refused status in its own words, not the server's", async () => {
      const user = userEvent.setup()
      // The item went back to In Use meanwhile; the form still sends Sold,
      // and the server asks for a date the item never had.
      const refused = new HttpError("invalid", 422, "/commodities/c1", {
        errors: [
          {
            status: "Unprocessable Entity",
            detail: "status_date: status date is required when leaving in_use.",
            error: {
              type: "validation.Errors",
              error: {
                data: {
                  attributes: { status_date: "status date is required when leaving in_use" },
                },
              },
              errorCodes: {
                data: {
                  attributes: { status_date: { code: "status_date_required_on_transition" } },
                },
              },
            },
          },
        ],
      })
      renderSold(vi.fn().mockRejectedValue(refused))
      await screen.findByLabelText(/^Name$/i)
      await user.click(screen.getByTestId("commodity-form-submit"))
      expect(
        await screen.findByText(/status changed after you opened this form/i)
      ).toBeInTheDocument()
      expect(screen.queryByText(/leaving in_use/i)).toBeNull()
    })
  })

  describe("edit mode (#2795)", () => {
    const existing = {
      id: "c1",
      name: "Lamp",
      short_name: "Lamp",
      type: "other" as const,
      area_id: "a1",
      status: "in_use" as const,
      count: 1,
      draft: false,
      // A non-draft item needs its purchase details to save.
      purchase_date: "2024-03-01",
      original_price: 40,
      original_price_currency: "USD",
    }

    function renderEdit(onSubmit = vi.fn().mockResolvedValue(undefined)) {
      renderWithProviders({
        children: (
          <CommodityFormDialog
            open
            onOpenChange={() => {}}
            mode="edit"
            initialValues={existing}
            areas={areas}
            locations={locations}
            defaultCurrency="USD"
            onSubmit={onSubmit}
          />
        ),
      })
      return onSubmit
    }

    it("lets every step be opened straight from the stepper", async () => {
      const user = userEvent.setup()
      renderEdit()
      await screen.findByLabelText(/^Name$/i)
      const stepper = screen.getByRole("list", { name: /form steps/i })
      expect(within(stepper).getByRole("button", { name: "Basics" })).toHaveAttribute(
        "aria-current",
        "step"
      )
      for (const title of ["Purchase", "Warranty", "Extras", "Files"]) {
        expect(within(stepper).getByRole("button", { name: title })).toBeEnabled()
      }
      await user.click(within(stepper).getByRole("button", { name: "Extras" }))
      expect(await screen.findByTestId("commodity-tags-input")).toBeInTheDocument()
      expect(within(stepper).getByRole("button", { name: "Extras" })).toHaveAttribute(
        "aria-current",
        "step"
      )
    })

    it("offers Save on every step and saves from the first one", async () => {
      const user = userEvent.setup()
      const onSubmit = renderEdit()
      await screen.findByLabelText(/^Name$/i)
      // Continue still walks forward, next to Save.
      expect(screen.getByTestId("commodity-form-next")).toBeInTheDocument()
      expect(screen.getByTestId("commodity-form-submit")).toBeInTheDocument()
      await user.click(screen.getByTestId("commodity-form-next"))
      await screen.findByLabelText(/Purchase date/i)
      expect(screen.getByTestId("commodity-form-submit")).toBeInTheDocument()
      await user.click(
        within(screen.getByRole("list", { name: /form steps/i })).getByRole("button", {
          name: "Basics",
        })
      )
      await user.clear(await screen.findByLabelText(/^Name$/i))
      await user.type(screen.getByLabelText(/^Name$/i), "Desk lamp")
      await user.click(screen.getByTestId("commodity-form-submit"))
      await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
      expect(onSubmit.mock.calls[0][0]).toMatchObject({ name: "Desk lamp", area_id: "a1" })
    })

    it("opens the step with the invalid field when Save fails validation", async () => {
      const user = userEvent.setup()
      const onSubmit = renderEdit()
      await user.clear(await screen.findByLabelText(/^Name$/i))
      await user.click(
        within(screen.getByRole("list", { name: /form steps/i })).getByRole("button", {
          name: "Warranty",
        })
      )
      await screen.findByTestId("commodity-form-warranty-step")
      await user.click(screen.getByTestId("commodity-form-submit"))
      // Back on Basics, where the empty name is.
      expect(await screen.findByLabelText(/^Name$/i)).toHaveAttribute("aria-invalid", "true")
      expect(onSubmit).not.toHaveBeenCalled()
    })
  })

  it("does not render the Location/Area picker in create mode (#1987)", async () => {
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    await walkPastAi(user)
    // The create flow no longer asks for a location — items are created
    // unassigned and a location is offered afterwards via a banner.
    expect(screen.queryByRole("combobox", { name: /^Location$/i })).toBeNull()
    expect(screen.queryByRole("combobox", { name: /^Area$/i })).toBeNull()
  })

  it("clears the location in edit mode and omits area_id on submit (#1986/#1987)", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="edit"
          initialValues={{
            id: "c1",
            name: "Lamp",
            short_name: "Lamp",
            type: "other",
            area_id: "a1",
            status: "in_use",
            count: 1,
            draft: true,
          }}
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={onSubmit}
        />
      ),
    })
    // Edit opens on Basics with the picker populated (area a1 → Home).
    expect(await screen.findByRole("combobox", { name: /^Location$/i })).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: /clear location/i }))
    // Clearing un-assigns: the Clear affordance disappears with the value.
    expect(screen.queryByRole("button", { name: /clear location/i })).toBeNull()
    // Walk Basics → Purchase → Warranty → Extras → Files → submit.
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByLabelText(/Purchase date/i)
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-warranty-step")
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-tags-input")
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-submit")
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    // area_id is omitted (undefined) so the BE clears the assignment.
    expect(onSubmit.mock.calls[0][0].area_id).toBeUndefined()
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
          locations={locations}
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
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
          draftKey={draftKey}
        />
      ),
    })
    await walkPastAi(user)
    expect(await screen.findByLabelText(/^Name$/i)).toHaveValue("Persisted")
    expect(screen.getByLabelText(/^Quantity$/i)).toHaveValue(3)
    // The draft persistence path now flows through the close-confirm
    // dialog — clicking "Discard" there clears the draft and closes
    // the wizard. To get the confirm to open the form must be marked
    // dirty; rehydrated values are not "dirty" (they're the form's
    // initial state from RHF's perspective), so type into Name to
    // mark the form dirty before requesting close.
    await user.type(screen.getByLabelText(/^Name$/i), "x")
    await user.click(screen.getByTestId("commodity-form-cancel"))
    await user.click(await screen.findByTestId("commodity-form-close-confirm-discard"))
    // After Discard, the draft key should no longer reflect the
    // "Persisted" name — clearDraft removes the entry entirely.
    const after = window.localStorage.getItem(draftKey)
    expect(after).toBeNull()
  })

  it("does not write the values back to the device after the form closes", async () => {
    const user = userEvent.setup()
    const draftKey = "commodity-draft:test:create"
    let setOpen: (open: boolean) => void = () => {}
    function Harness() {
      const [open, set] = useState(true)
      setOpen = set
      return (
        <CommodityFormDialog
          open={open}
          onOpenChange={set}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
          draftKey={draftKey}
        />
      )
    }
    renderWithProviders({ children: <Harness /> })
    await walkPastAi(user)
    const name = await screen.findByLabelText(/^Name$/i)
    // A change and the close in the same frame, as when a save clears the
    // stored draft and closes the form.
    act(() => {
      fireEvent.change(name, { target: { value: "Late lamp" } })
      setOpen(false)
      window.localStorage.removeItem(draftKey)
    })
    await act(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())))
    expect(window.localStorage.getItem(draftKey)).toBeNull()
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
          locations={locations}
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
          locations={locations}
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

  // Regression: clicking Cancel on a dirty create-mode wizard must
  // surface the save-as-draft confirmation, not exit silently. The
  // user reported the prompt stopped appearing — likely either the
  // dirty flag isn't propagating through RHF or the close button
  // bypasses requestClose.
  it("Cancel on a dirty create-mode wizard opens the save-as-draft confirm", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={onOpenChange}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
          draftKey="commodity-draft:test:create"
        />
      ),
    })
    await walkPastAi(user)
    // Mark form dirty by typing into Name (any visible field works).
    await user.type(await screen.findByLabelText(/^Name$/i), "X")
    await user.click(screen.getByTestId("commodity-form-cancel"))
    // Confirm dialog should mount; outer Dialog stays open.
    await screen.findByTestId("commodity-form-close-confirm")
    expect(screen.getByTestId("commodity-form-close-confirm-save")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-form-close-confirm-discard")).toBeInTheDocument()
    // We never propagated a close to the parent.
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  // Regression companion: pristine create-mode Cancel must close
  // immediately without prompting.
  it("Cancel on a pristine create-mode wizard closes immediately", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={onOpenChange}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
          draftKey="commodity-draft:test:create"
        />
      ),
    })
    await walkPastAi(user)
    await user.click(screen.getByTestId("commodity-form-cancel"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(screen.queryByTestId("commodity-form-close-confirm")).not.toBeInTheDocument()
  })

  // #1627: below `sm` the wizard fills the screen; the save-as-draft
  // confirm stays a centered card and shows above it. Escape on the
  // wizard still asks first.
  it("fills the screen on a phone and keeps the save-as-draft confirm above it", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={onOpenChange}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
          draftKey="commodity-draft:test:create"
        />
      ),
    })
    const wizard = await screen.findByTestId("commodity-form-dialog")
    expect(wizard).toHaveAttribute("data-variant", "fullscreen-mobile")
    expect(wizard.className).toContain("max-sm:h-dvh")
    expect(wizard.className).toContain("sm:max-h-[90vh]")
    await walkPastAi(user)
    await user.type(await screen.findByLabelText(/^Name$/i), "X")

    await user.keyboard("{Escape}")
    const confirm = await screen.findByTestId("commodity-form-close-confirm")
    expect(confirm).toHaveAttribute("data-variant", "default")
    expect(confirm.className).not.toContain("max-sm:")
    expect(wizard.compareDocumentPosition(confirm) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(onOpenChange).not.toHaveBeenCalled()

    await user.click(screen.getByTestId("commodity-form-close-confirm-cancel"))
    await waitFor(() =>
      expect(screen.queryByTestId("commodity-form-close-confirm")).not.toBeInTheDocument()
    )
    expect(screen.getByTestId("commodity-form-dialog")).toBeInTheDocument()
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  // Regression: a single dirty input on Basics is enough to make
  // Cancel prompt rather than silently discard. Together with the
  // pristine-Cancel test above and the Files-step variant below this
  // pins the dirty-detection contract of the close-confirm flow.
  it("Cancel after dirtying Basics prompts instead of silently closing", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={onOpenChange}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
          draftKey="commodity-draft:test:create"
        />
      ),
    })
    await walkPastAi(user)
    await user.type(await screen.findByLabelText(/^Name$/i), "X")
    await user.click(screen.getByTestId("commodity-form-cancel"))
    await screen.findByTestId("commodity-form-close-confirm")
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  // Regression: cancelling on the Files step with upstream-only dirty
  // edits must still surface the save-as-draft confirm. The dirty
  // check has to look at the cumulative form state, not just the
  // active step's inputs — a Basics edit that the user walked away
  // from is still a dirty draft. PR #1621 reviewer flagged this gap
  // as the Files-step variant of the Basics test above; #1629
  // re-enabled the Radix-Select-driven walk that gets the test to
  // Files in the first place.
  it("Cancel on the Files step with upstream steps dirty still prompts", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={onOpenChange}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
          draftKey="commodity-draft:test-files-cancel:create"
        />
      ),
    })
    // Walk past AI → fill Basics minimally (draft mode keeps the
    // schema's whenNotDraft block off so Continue stays unblocked).
    await walkPastAi(user)
    await user.type(await screen.findByLabelText(/^Name$/i), "Couch")
    await pickSelect(user, /^Type$/i, /^Furniture$/i)
    await user.click(screen.getByLabelText(/Save as draft/i))
    // Step through Basics → Purchase → Warranty → Extras → Files.
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByLabelText(/Purchase date/i)
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-warranty-step")
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-tags-input")
    await user.click(screen.getByTestId("commodity-form-next"))
    // On Files step — the Cancel button still surfaces (the dialog
    // shares the footer across all steps). Click it without touching
    // the Files dropzone so the dirty signal comes from upstream
    // Basics only.
    await screen.findByTestId("commodity-form-submit")
    await user.click(screen.getByTestId("commodity-form-cancel"))
    await screen.findByTestId("commodity-form-close-confirm")
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  // Regression: dialog reopens with values restored from a
  // previously-saved localStorage draft. RHF treats the rehydrated
  // values as the form's new defaults so `isDirty` is false even
  // though the form is visibly populated. Cancel must STILL prompt
  // — otherwise the user sees their draft "ghost-loaded" and clicks
  // Cancel thinking nothing happens, losing the draft silently.
  it("Cancel after a rehydrated draft prompts even with isDirty=false", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    const draftKey = "commodity-draft:test-rehydrate:create"
    // Seed localStorage with a partial draft — same shape readDraft
    // expects (matches buildDefaults' field set).
    window.localStorage.setItem(
      draftKey,
      JSON.stringify({
        name: "Persisted Item",
        short_name: "Persist",
        type: "furniture",
        area_id: "a1",
        status: "in_use",
        count: "1",
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
        draft: false,
        warranty_expires_at: "",
        warranty_notes: "",
      })
    )
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={onOpenChange}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
          draftKey={draftKey}
        />
      ),
    })
    await walkPastAi(user)
    // Verify the draft was rehydrated — form is populated but the
    // user hasn't typed anything in this session.
    expect(await screen.findByLabelText(/^Name$/i)).toHaveValue("Persisted Item")
    // Click Cancel WITHOUT typing first. isDirty is false here
    // (RHF anchors on the rehydrated values).
    await user.click(screen.getByTestId("commodity-form-cancel"))
    // Confirm must still appear so the user can choose between
    // keeping the draft and discarding it.
    await screen.findByTestId("commodity-form-close-confirm")
    expect(onOpenChange).not.toHaveBeenCalled()
    // Cleanup so the next test starts clean.
    window.localStorage.removeItem(draftKey)
  })

  it("has no axe violations", async () => {
    const { container } = renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    await screen.findByTestId("commodity-form-ai-step")
    expect(await axe(container)).toHaveNoViolations()
  })
})
