import { describe, expect, it, vi } from "vitest"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { DeleteWithItemsDialog } from "@/components/locations/DeleteWithItemsDialog"
import { renderWithProviders } from "@/test/render"

describe("<DeleteWithItemsDialog />", () => {
  it("defaults to unlink and resolves with the chosen strategy when confirmed", async () => {
    const user = userEvent.setup()
    const onResolve = vi.fn()
    renderWithProviders({
      children: (
        <DeleteWithItemsDialog
          open
          kind="area"
          name="Workshop"
          itemCount={3}
          onResolve={onResolve}
        />
      ),
    })

    // Confirm without changing the radio → the safe default (unlink).
    await user.click(screen.getByTestId("delete-with-items-confirm"))
    expect(onResolve).toHaveBeenCalledTimes(1)
    expect(onResolve).toHaveBeenCalledWith("unlink")
  })

  it("resolves with cascade once the destructive option is picked", async () => {
    const user = userEvent.setup()
    const onResolve = vi.fn()
    renderWithProviders({
      children: (
        <DeleteWithItemsDialog
          open
          kind="location"
          name="Garage"
          itemCount={5}
          areaCount={2}
          onResolve={onResolve}
        />
      ),
    })

    await user.click(screen.getByTestId("delete-with-items-cascade"))
    await user.click(screen.getByTestId("delete-with-items-confirm"))
    expect(onResolve).toHaveBeenCalledWith("cascade")
  })

  it("resolves with null when cancelled", async () => {
    const user = userEvent.setup()
    const onResolve = vi.fn()
    renderWithProviders({
      children: (
        <DeleteWithItemsDialog
          open
          kind="area"
          name="Workshop"
          itemCount={1}
          onResolve={onResolve}
        />
      ),
    })

    await user.click(screen.getByTestId("delete-with-items-cancel"))
    expect(onResolve).toHaveBeenCalledTimes(1)
    expect(onResolve).toHaveBeenCalledWith(null)
  })

  it("renders the location unlink copy (areas removed, items survive)", () => {
    renderWithProviders({
      children: (
        <DeleteWithItemsDialog
          open
          kind="location"
          name="Garage"
          itemCount={4}
          areaCount={2}
          onResolve={vi.fn()}
        />
      ),
    })

    // The unlink option must make the "areas removed / items survive"
    // contract explicit (#2137 copy requirement).
    expect(screen.getByText(/areas are removed \(the items survive\)/i)).toBeInTheDocument()
  })
})
