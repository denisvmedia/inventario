import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "jest-axe"

import { PasswordStrengthMeter, scorePassword } from "@/components/auth/PasswordStrengthMeter"

describe("scorePassword", () => {
  it("returns 0 for empty input", () => {
    expect(scorePassword("")).toBe(0)
  })

  it("rewards length and character-class diversity", () => {
    expect(scorePassword("short")).toBe(0)
    expect(scorePassword("eightchr")).toBeGreaterThanOrEqual(1)
    expect(scorePassword("Eightch1")).toBeGreaterThanOrEqual(2)
    expect(scorePassword("LongerOne1")).toBeGreaterThanOrEqual(3)
    expect(scorePassword("LongerOne1!")).toBeGreaterThanOrEqual(3)
    expect(scorePassword("LongerOne123!")).toBe(4)
  })
})

describe("<PasswordStrengthMeter />", () => {
  it("renders the empty hint when password is blank", () => {
    render(<PasswordStrengthMeter password="" />)
    expect(screen.getByText(/8\+ characters/i)).toBeInTheDocument()
  })

  it("exposes a meter role with the current score", () => {
    render(<PasswordStrengthMeter password="LongerOne123!" />)
    const meter = screen.getByRole("meter")
    expect(meter).toHaveAttribute("aria-valuenow", "4")
  })

  it("has no axe violations", async () => {
    const { container } = render(<PasswordStrengthMeter password="LongerOne1" />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
