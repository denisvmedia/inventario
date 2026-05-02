import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "jest-axe"

import { ComingSoonBanner } from "@/components/coming-soon"

describe("<ComingSoonBanner />", () => {
  it("renders the surface title, description, and tracker link", () => {
    render(<ComingSoonBanner surface="twoFactor" />)
    expect(screen.getByText(/two-factor/i)).toBeInTheDocument()
    expect(screen.getByText(/authenticator-app/i)).toBeInTheDocument()
    const link = screen.getByRole("link")
    expect(link).toHaveAttribute("href", "https://github.com/denisvmedia/inventario/issues/1380")
    expect(link).toHaveAttribute("target", "_blank")
    expect(link).toHaveAttribute("rel", expect.stringContaining("noopener"))
  })

  it("uses the default data-testid based on the surface key", () => {
    render(<ComingSoonBanner surface="connectedAccounts" />)
    expect(screen.getByTestId("coming-soon-banner-connectedAccounts")).toBeInTheDocument()
  })

  it("accepts a testId override (used by TwoFactorStub for back-compat)", () => {
    render(<ComingSoonBanner surface="twoFactor" testId="two-factor-stub" />)
    expect(screen.getByTestId("two-factor-stub")).toBeInTheDocument()
  })

  it("exposes no actionable controls (per #1417 acceptance criteria)", () => {
    render(<ComingSoonBanner surface="oauth" />)
    expect(screen.queryByRole("button")).not.toBeInTheDocument()
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument()
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument()
  })

  it("has no axe violations", async () => {
    const { container } = render(<ComingSoonBanner surface="notificationPreferences" />)
    expect(await axe(container)).toHaveNoViolations()
  })
})
