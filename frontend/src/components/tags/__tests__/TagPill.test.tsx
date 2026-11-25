import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { describe, expect, it, vi } from "vitest"

import { TagPill } from "@/components/tags/TagPill"

describe("<TagPill />", () => {
  it("renders the label after a hash glyph in the tag's color", () => {
    render(<TagPill label="Garage" color="blue" data-testid="pill" />)
    const pill = screen.getByTestId("pill")
    expect(pill.tagName).toBe("SPAN")
    expect(pill).toHaveTextContent("Garage")
    expect(pill.querySelector("svg")).not.toBeNull()
    expect(pill.className).toContain("text-tag-blue")
    expect(pill.className).toContain("bg-tag-blue/15")
    expect(pill.className).toContain("border-tag-blue/30")
  })

  it("uses the mock's two sizes", () => {
    const { rerender } = render(<TagPill label="x" color="muted" data-testid="pill" />)
    expect(screen.getByTestId("pill").className).toContain("h-5")
    rerender(<TagPill label="x" color="muted" size="xs" data-testid="pill" />)
    expect(screen.getByTestId("pill").className).toContain("h-4")
  })

  it("is a link when given an href", () => {
    render(
      <MemoryRouter>
        <TagPill label="Garage" color="green" href="/g/home/commodities?tags=garage" />
      </MemoryRouter>
    )
    expect(screen.getByRole("link", { name: "Garage" })).toHaveAttribute(
      "href",
      "/g/home/commodities?tags=garage"
    )
  })

  it("calls onRemove from its remove button", async () => {
    const onRemove = vi.fn()
    render(<TagPill label="Garage" color="red" onRemove={onRemove} removeLabel="Remove Garage" />)
    await userEvent.click(screen.getByRole("button", { name: "Remove Garage" }))
    expect(onRemove).toHaveBeenCalledOnce()
  })
})
