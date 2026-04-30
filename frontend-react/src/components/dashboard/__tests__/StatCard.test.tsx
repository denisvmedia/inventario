import { describe, expect, it } from "vitest"
import { Package } from "lucide-react"
import { screen } from "@testing-library/react"
import { axe } from "jest-axe"

import { StatCard } from "@/components/dashboard/StatCard"
import { renderWithProviders } from "@/test/render"

describe("<StatCard />", () => {
  it("renders the label, value, and sub line", () => {
    renderWithProviders({
      children: (
        <StatCard label="Total items" value={42} sub="across all locations" icon={Package} />
      ),
    })
    expect(screen.getByText("Total items")).toBeInTheDocument()
    expect(screen.getByText("42")).toBeInTheDocument()
    expect(screen.getByText("across all locations")).toBeInTheDocument()
  })

  it("hides the sub line when omitted", () => {
    renderWithProviders({
      children: <StatCard label="Total items" value={5} />,
    })
    expect(screen.queryByText("across all locations")).not.toBeInTheDocument()
  })

  it("wraps the card in a <Link> when `to` is provided", () => {
    renderWithProviders({
      children: <StatCard label="Total items" value={5} to="/g/foo/commodities" />,
    })
    const link = screen.getByRole("link")
    expect(link).toHaveAttribute("href", "/g/foo/commodities")
  })

  it("renders skeletons in place of value+sub while loading", () => {
    const { container } = renderWithProviders({
      children: <StatCard label="Total items" value="—" sub="across all locations" isLoading />,
    })
    expect(screen.queryByText("—")).not.toBeInTheDocument()
    expect(screen.queryByText("across all locations")).not.toBeInTheDocument()
    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThanOrEqual(1)
  })

  it("connects the value to the label via aria-labelledby", () => {
    renderWithProviders({
      children: <StatCard label="Total items" value={5} testId="stat-total-items" />,
    })
    const card = screen.getByTestId("stat-total-items")
    const labelledBy = card.getAttribute("aria-labelledby")
    expect(labelledBy).toBe("stat-total-items-label")
    expect(document.getElementById(labelledBy!)).toHaveTextContent("Total items")
  })

  it("has no axe violations", async () => {
    const { container } = renderWithProviders({
      children: <StatCard label="Total items" value={5} icon={Package} />,
    })
    expect(await axe(container)).toHaveNoViolations()
  })
})
