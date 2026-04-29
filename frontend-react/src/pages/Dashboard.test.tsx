import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { axe } from "jest-axe"

import { DashboardPage } from "./Dashboard"

describe("DashboardPage", () => {
  it("renders the Inventario heading and CTA", () => {
    render(
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>
    )
    expect(screen.getByRole("heading", { name: "Inventario", level: 1 })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /get started/i })).toBeInTheDocument()
  })

  it("has no axe violations", async () => {
    const { container } = render(
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>
    )
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
