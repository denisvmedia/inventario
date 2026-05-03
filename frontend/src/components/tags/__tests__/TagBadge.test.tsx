import { axe } from "jest-axe"
import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { TagBadge } from "@/components/tags/TagBadge"

describe("<TagBadge />", () => {
  it("renders the supplied label", () => {
    render(<TagBadge label="Kitchen" color="amber" />)
    expect(screen.getByText("Kitchen")).toBeInTheDocument()
  })

  it("applies the tone class for the requested colour", () => {
    const { container } = render(<TagBadge label="x" color="green" testId="tb" />)
    const badge = container.querySelector('[data-testid="tb"]')
    expect(badge?.className).toContain("text-tag-green")
    expect(badge?.className).toContain("border-tag-green")
    expect(badge?.className).toContain("bg-tag-green")
  })

  it("renders both size variants without crashing", () => {
    const { rerender } = render(<TagBadge label="x" color="muted" size="sm" testId="tb" />)
    expect(screen.getByTestId("tb").className).toContain("h-5")
    rerender(<TagBadge label="x" color="muted" size="md" testId="tb" />)
    expect(screen.getByTestId("tb").className).toContain("h-6")
  })

  it("is axe-clean", async () => {
    const { container } = render(<TagBadge label="Kitchen" color="amber" />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
