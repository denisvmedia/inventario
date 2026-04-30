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

describe("<CommodityFormDialog />", () => {
  it("renders the basics step on first open", async () => {
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
    expect(await screen.findByLabelText(/^Name$/i)).toBeInTheDocument()
    expect(screen.getByText(/Basics/i)).toBeInTheDocument()
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
    await user.click(await screen.findByTestId("commodity-form-next"))
    // Stayed on basics — purchase-step heading shouldn't appear.
    await waitFor(() =>
      expect(screen.getAllByText(/Required|Pick/i).length).toBeGreaterThan(0)
    )
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
          defaultCurrency="USD"
          onSubmit={onSubmit}
        />
      ),
    })
    await user.type(await screen.findByLabelText(/^Name$/i), "Couch")
    await user.type(screen.getByLabelText(/^Short name$/i), "Couch")
    // Type select uses a native <select>; selectOptions targets the
    // renderered option text from the type catalog.
    await user.selectOptions(screen.getByLabelText(/^Type$/i), "furniture")
    await user.selectOptions(screen.getByLabelText(/^Area$/i), "a1")
    // Tick the draft toggle so the schema's whenNotDraft block doesn't
    // require purchase_date + the price triad — keeps this test focused
    // on step navigation rather than every field.
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-next"))
    // Purchase step: just advance — every purchase field is optional.
    await waitFor(() =>
      expect(screen.getByLabelText(/Purchase date/i)).toBeInTheDocument()
    )
    await user.click(screen.getByTestId("commodity-form-next"))
    // Extras step has the submit button.
    await waitFor(() =>
      expect(screen.getByTestId("commodity-form-submit")).toBeInTheDocument()
    )
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
          defaultCurrency="USD"
          onSubmit={onSubmit}
        />
      ),
    })
    // Walk to extras step by filling required basics + skipping purchase.
    await user.type(await screen.findByLabelText(/^Name$/i), "Stand")
    await user.type(screen.getByLabelText(/^Short name$/i), "Stand")
    await user.selectOptions(screen.getByLabelText(/^Type$/i), "furniture")
    await user.selectOptions(screen.getByLabelText(/^Area$/i), "a1")
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByLabelText(/Purchase date/i)
    await user.click(screen.getByTestId("commodity-form-next"))
    // Tag chip input lives on the extras step.
    const tagsInput = await screen.findByTestId("commodity-tags-input")
    await user.type(tagsInput, "office,wood,")
    await waitFor(() =>
      expect(screen.getAllByTestId("commodity-tags-chip").length).toBe(2)
    )
    // Remove the first chip.
    const firstChip = screen.getAllByTestId("commodity-tags-chip")[0]
    await user.click(within(firstChip).getByRole("button", { name: /remove/i }))
    await waitFor(() =>
      expect(screen.getAllByTestId("commodity-tags-chip").length).toBe(1)
    )
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
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    await user.type(await screen.findByLabelText(/^Name$/i), "X")
    await user.type(screen.getByLabelText(/^Short name$/i), "X")
    await user.selectOptions(screen.getByLabelText(/^Type$/i), "other")
    await user.selectOptions(screen.getByLabelText(/^Area$/i), "a1")
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
    expect(screen.getByLabelText(/^Status$/i)).toHaveValue("sold")
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
    // Skip basics + purchase to reach extras.
    await user.click(await screen.findByTestId("commodity-form-next"))
    await screen.findByLabelText(/Purchase date/i)
    await user.click(screen.getByTestId("commodity-form-next"))
    const input = await screen.findByTestId("commodity-tags-input")
    expect(screen.getAllByTestId("commodity-tags-chip").length).toBe(2)
    await user.click(input)
    await user.keyboard("{Backspace}")
    await waitFor(() =>
      expect(screen.getAllByTestId("commodity-tags-chip").length).toBe(1)
    )
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
    await screen.findByLabelText(/^Name$/i)
    expect(await axe(container)).toHaveNoViolations()
  })
})
