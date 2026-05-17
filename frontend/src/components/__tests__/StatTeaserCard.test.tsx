import { describe, expect, it } from "vitest"
import { Package } from "lucide-react"
import { screen } from "@testing-library/react"

import { StatTeaserCard } from "@/components/StatTeaserCard"
import { renderWithProviders } from "@/test/render"

describe("<StatTeaserCard />", () => {
  it("renders the label and value as plain text when no href is provided", () => {
    renderWithProviders({
      children: <StatTeaserCard label="Items tracked" value="12" icon={Package} testId="teaser" />,
    })
    const card = screen.getByTestId("teaser")
    expect(card.tagName).toBe("DIV")
    expect(card).toHaveTextContent("Items tracked")
    expect(card).toHaveTextContent("12")
  })

  it("becomes a router link when href is set, drilling to the target route", () => {
    renderWithProviders({
      children: <StatTeaserCard label="Items" value="12" href="/g/x/commodities" testId="teaser" />,
    })
    const link = screen.getByTestId("teaser")
    expect(link.tagName).toBe("A")
    expect(link).toHaveAttribute("href", "/g/x/commodities")
  })

  it("omits the icon mark when no icon prop is passed", () => {
    renderWithProviders({
      children: <StatTeaserCard label="Items" value="12" testId="teaser" />,
    })
    expect(screen.getByTestId("teaser").querySelector("svg")).toBeNull()
  })
})
