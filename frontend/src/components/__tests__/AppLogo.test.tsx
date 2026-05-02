import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "jest-axe"

import { AppLogo } from "@/components/AppLogo"

describe("AppLogo", () => {
  it("renders the brand wordmark", () => {
    render(<AppLogo />)
    expect(screen.getByText("Inventario")).toBeInTheDocument()
  })

  it("forwards a className override onto the root", () => {
    const { container } = render(<AppLogo className="custom-cls" />)
    expect(container.firstChild).toHaveClass("custom-cls")
  })

  it("passes axe (decorative SVG is aria-hidden)", async () => {
    const { container } = render(<AppLogo />)
    expect(await axe(container)).toHaveNoViolations()
  })
})
