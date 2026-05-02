import { describe, expect, it, beforeEach } from "vitest"
import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { ThemeProvider } from "@/components/theme-provider"
import { ModeToggle } from "@/components/ModeToggle"

beforeEach(() => {
  window.localStorage.clear()
  document.documentElement.classList.remove("light", "dark")
})

describe("ModeToggle", () => {
  it("flips the .dark class on <html> via the menu", async () => {
    const user = userEvent.setup()
    render(
      <ThemeProvider storageKey="test-theme">
        <ModeToggle />
      </ThemeProvider>
    )
    await user.click(screen.getByRole("button", { name: /toggle theme/i }))
    const menu = await screen.findByRole("menu")
    await user.click(within(menu).getByText(/^dark$/i))
    expect(document.documentElement.classList.contains("dark")).toBe(true)
  })

  it("system option resolves prefers-color-scheme", async () => {
    const user = userEvent.setup()
    render(
      <ThemeProvider storageKey="test-theme">
        <ModeToggle />
      </ThemeProvider>
    )
    await user.click(screen.getByRole("button", { name: /toggle theme/i }))
    const menu = await screen.findByRole("menu")
    await user.click(within(menu).getByText(/^system$/i))
    // The setup.ts matchMedia stub returns matches=false, which the theme
    // provider treats as "light".
    expect(document.documentElement.classList.contains("light")).toBe(true)
  })
})
