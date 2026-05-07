import { beforeAll, describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"

import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<WarrantyBadge />", () => {
  it("renders the i18n label for the precomputed status", () => {
    render(<WarrantyBadge status="active" data-testid="badge" />)
    const badge = screen.getByTestId("badge")
    expect(badge).toHaveTextContent("Active")
    expect(badge).toHaveAttribute("data-status", "active")
    expect(badge).toHaveClass("text-status-active")
  })

  it("derives the status from a raw commodity slice when `source` is given", () => {
    render(<WarrantyBadge source={{ warranty_expires_at: "1999-01-01" }} data-testid="badge" />)
    const badge = screen.getByTestId("badge")
    expect(badge).toHaveAttribute("data-status", "expired")
    expect(badge).toHaveTextContent("Expired")
  })

  it("falls back to the legacy warranty:YYYY-MM-DD tag when the field is missing", () => {
    render(<WarrantyBadge source={{ tags: ["warranty:2099-01-01"] }} data-testid="badge" />)
    expect(screen.getByTestId("badge")).toHaveAttribute("data-status", "active")
  })

  it("renders the `none` bucket when neither field nor tag is set", () => {
    render(<WarrantyBadge source={{}} data-testid="badge" />)
    expect(screen.getByTestId("badge")).toHaveAttribute("data-status", "none")
    expect(screen.getByTestId("badge")).toHaveTextContent("No warranty")
  })

  it("hides the leading icon when `showIcon` is false", () => {
    const { container } = render(<WarrantyBadge status="active" showIcon={false} />)
    expect(container.querySelector("svg")).toBeNull()
  })
})
