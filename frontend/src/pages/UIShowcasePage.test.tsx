import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"

import { UIShowcasePage } from "./UIShowcasePage"

describe("UIShowcasePage", () => {
  it("renders the showcase with the dev-only badge and tab strip", () => {
    render(
      <MemoryRouter>
        <UIShowcasePage />
      </MemoryRouter>
    )
    expect(screen.getByRole("heading", { name: /ui showcase/i, level: 1 })).toBeInTheDocument()
    expect(screen.getByText(/dev only/i)).toBeInTheDocument()
    // Tab triggers — verifying the 7 section tabs are rendered.
    for (const label of [
      "Buttons",
      "Forms",
      "Overlays",
      "Feedback",
      "Layout",
      "Typography",
      "Tokens",
    ]) {
      expect(screen.getByRole("tab", { name: label })).toBeInTheDocument()
    }
  })
})
