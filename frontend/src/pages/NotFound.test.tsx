import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { axe } from "jest-axe"

import { NotFoundPage } from "./NotFound"

describe("NotFoundPage", () => {
  it("renders the heading, copy, and a Go home link", () => {
    render(
      <MemoryRouter>
        <NotFoundPage />
      </MemoryRouter>
    )
    expect(screen.getByRole("heading", { name: /page not found/i, level: 1 })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: /go home/i })).toHaveAttribute("href", "/")
  })

  it("has no axe violations", async () => {
    const { container } = render(
      <MemoryRouter>
        <NotFoundPage />
      </MemoryRouter>
    )
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
