import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "jest-axe"

import { ComingSoonPage, SURFACES, type SurfaceKey } from "@/components/coming-soon"

describe("<ComingSoonPage />", () => {
  it("renders the heading from the registry's i18n key", () => {
    render(<ComingSoonPage surface="plans" />)
    expect(
      screen.getByRole("heading", { level: 1, name: /subscription plans/i })
    ).toBeInTheDocument()
    expect(screen.getByText(/subscription tiers/i)).toBeInTheDocument()
  })

  it("links to the GitHub tracker issue with target=_blank rel=noopener", () => {
    render(<ComingSoonPage surface="helpCenter" />)
    const link = screen.getByRole("link")
    expect(link).toHaveAttribute("href", "https://github.com/denisvmedia/inventario/issues/1384")
    expect(link).toHaveAttribute("target", "_blank")
    expect(link.getAttribute("rel")).toContain("noopener")
  })

  it("uses the default data-testid based on the surface key", () => {
    render(<ComingSoonPage surface="whatsNew" />)
    expect(screen.getByTestId("coming-soon-page-whatsNew")).toBeInTheDocument()
  })

  it("renders no enabled controls", () => {
    render(<ComingSoonPage surface="insuranceReport" />)
    expect(screen.queryByRole("button")).not.toBeInTheDocument()
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument()
  })

  it("renders every page-kind surface without throwing", () => {
    const pageSurfaces = (Object.keys(SURFACES) as SurfaceKey[]).filter(
      (k) => SURFACES[k].kind === "page" || SURFACES[k].kind === "both"
    )
    for (const surface of pageSurfaces) {
      const { unmount } = render(<ComingSoonPage surface={surface} />)
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument()
      unmount()
    }
  })

  it("has no axe violations", async () => {
    const { container } = render(<ComingSoonPage surface="plans" />)
    expect(await axe(container)).toHaveNoViolations()
  })
})
