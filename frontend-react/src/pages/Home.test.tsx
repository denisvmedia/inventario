import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "jest-axe"

import { HomePage } from "./Home"

describe("HomePage", () => {
  it("renders the Inventario heading and CTA", () => {
    render(<HomePage />)
    expect(screen.getByRole("heading", { name: "Inventario", level: 1 })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /get started/i })).toBeInTheDocument()
  })

  it("has no axe violations", async () => {
    const { container } = render(<HomePage />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
