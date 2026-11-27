import { axe } from "jest-axe"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeAll, describe, expect, it, vi } from "vitest"

import { StatusTransitionDialog } from "@/components/items/StatusTransitionDialog"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<StatusTransitionDialog />", () => {
  it("submits with the date default + empty note for a non-sold target", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(
      <StatusTransitionDialog
        open
        targetStatus="lost"
        purchaseCurrency="USD"
        onSubmit={onSubmit}
        onOpenChange={vi.fn()}
      />
    )

    await user.click(screen.getByTestId("status-transition-confirm"))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    const call = onSubmit.mock.calls[0]?.[0]
    expect(call.status_date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(call.status_note).toBe("")
    expect(call.sale_price).toBeUndefined()
  })

  it("marks an emptied date invalid and describes it by its message", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(
      <StatusTransitionDialog
        open
        targetStatus="lost"
        purchaseCurrency="USD"
        onSubmit={onSubmit}
        onOpenChange={vi.fn()}
      />
    )

    const date = screen.getByTestId("status-transition-date")
    await user.clear(date)
    await user.click(screen.getByTestId("status-transition-confirm"))

    expect(await screen.findByTestId("status-transition-date-error")).toBeInTheDocument()
    expect(date).toHaveAttribute("aria-invalid", "true")
    expect(date).toHaveAccessibleDescription("Date is required.")
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it.each([
    ["donated", "Mark as Donated", "Record that you gave this item away.", "Given to…"],
    ["stolen", "Mark as Stolen", /theft of this item/, "Taken from…"],
  ] as const)(
    "asks %s for a date and a note, without a sale price",
    async (status, title, description, placeholder) => {
      const user = userEvent.setup()
      const onSubmit = vi.fn().mockResolvedValue(undefined)
      render(
        <StatusTransitionDialog
          open
          targetStatus={status}
          purchaseCurrency="USD"
          onSubmit={onSubmit}
          onOpenChange={vi.fn()}
        />
      )
      expect(screen.getByRole("heading", { name: title })).toBeInTheDocument()
      expect(screen.getByText(description)).toBeInTheDocument()
      expect(screen.getByTestId("status-transition-note")).toHaveAttribute(
        "placeholder",
        placeholder
      )
      expect(screen.queryByTestId("status-transition-sale-price")).toBeNull()

      await user.type(screen.getByTestId("status-transition-note"), "School fair")
      await user.click(screen.getByTestId("status-transition-confirm"))
      expect(onSubmit).toHaveBeenCalledTimes(1)
      const call = onSubmit.mock.calls[0]?.[0]
      expect(call.status_note).toBe("School fair")
      expect(call.sale_price).toBeUndefined()
    }
  )

  it("renders the sale-price field with the purchase currency symbol when target=sold", async () => {
    render(
      <StatusTransitionDialog
        open
        targetStatus="sold"
        purchaseCurrency="EUR"
        onSubmit={vi.fn()}
        onOpenChange={vi.fn()}
      />
    )
    const priceInput = screen.getByTestId("status-transition-sale-price")
    expect(priceInput).toBeInTheDocument()
    // The € symbol prefix is rendered as a sibling span; verify it
    // appears in the surrounding markup so a currency swap does ripple
    // through to the input adornment.
    expect(screen.getByText("€")).toBeInTheDocument()
  })

  it("blocks submit when sale_price is empty and target=sold", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(
      <StatusTransitionDialog
        open
        targetStatus="sold"
        purchaseCurrency="USD"
        onSubmit={onSubmit}
        onOpenChange={vi.fn()}
      />
    )
    await user.click(screen.getByTestId("status-transition-confirm"))

    expect(await screen.findByTestId("status-transition-sale-price-error")).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("threads the captured sale_price + note when submitting a sold transition", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(
      <StatusTransitionDialog
        open
        targetStatus="sold"
        purchaseCurrency="USD"
        onSubmit={onSubmit}
        onOpenChange={vi.fn()}
      />
    )
    await user.type(screen.getByTestId("status-transition-sale-price"), "99.5")
    await user.type(screen.getByTestId("status-transition-note"), "Sold to Bob")
    await user.click(screen.getByTestId("status-transition-confirm"))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    const call = onSubmit.mock.calls[0]?.[0]
    expect(call.sale_price).toBe(99.5)
    expect(call.status_note).toBe("Sold to Bob")
  })

  it("renders nothing when targetStatus is null", () => {
    const { container } = render(
      <StatusTransitionDialog
        open
        targetStatus={null}
        purchaseCurrency="USD"
        onSubmit={vi.fn()}
        onOpenChange={vi.fn()}
      />
    )
    expect(container.firstChild).toBeNull()
  })

  it("is axe-clean while open", async () => {
    const { baseElement } = render(
      <StatusTransitionDialog
        open
        targetStatus="sold"
        purchaseCurrency="USD"
        onSubmit={vi.fn()}
        onOpenChange={vi.fn()}
      />
    )
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
