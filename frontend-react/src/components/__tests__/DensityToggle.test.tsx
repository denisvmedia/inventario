import { describe, expect, it, beforeEach } from "vitest"
import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { DensityProvider } from "@/hooks/useDensity"
import { DensityToggle } from "@/components/DensityToggle"

beforeEach(() => {
  window.localStorage.clear()
  document.documentElement.removeAttribute("data-density")
})

describe("DensityToggle", () => {
  it("switches data-density on <html> via the menu", async () => {
    const user = userEvent.setup()
    render(
      <DensityProvider storageKey="test-density">
        <DensityToggle />
      </DensityProvider>
    )
    await user.click(screen.getByRole("button", { name: /toggle density/i }))
    const menu = await screen.findByRole("menu")
    await user.click(within(menu).getByText(/^compact$/i))
    expect(document.documentElement.getAttribute("data-density")).toBe("compact")
  })

  it("each menu option maps to its density level", async () => {
    const user = userEvent.setup()
    render(
      <DensityProvider storageKey="test-density">
        <DensityToggle />
      </DensityProvider>
    )
    for (const [label, value] of [
      ["Comfortable", "comfortable"],
      ["Cozy", "cozy"],
      ["Compact", "compact"],
    ] as const) {
      await user.click(screen.getByRole("button", { name: /toggle density/i }))
      const menu = await screen.findByRole("menu")
      await user.click(within(menu).getByText(label))
      expect(document.documentElement.getAttribute("data-density")).toBe(value)
    }
  })
})
