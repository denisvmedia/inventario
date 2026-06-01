import { describe, expect, it, vi } from "vitest"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { AssignLocationBanner } from "@/components/items/AssignLocationBanner"
import { renderWithProviders } from "@/test/render"

describe("AssignLocationBanner", () => {
  it("renders nothing when show is false", () => {
    renderWithProviders({ children: <AssignLocationBanner show={false} onAssign={() => {}} /> })
    expect(screen.queryByTestId("assign-location-banner")).toBeNull()
  })

  it("shows the prompt when show is true", () => {
    renderWithProviders({ children: <AssignLocationBanner show onAssign={() => {}} /> })
    expect(screen.getByTestId("assign-location-banner")).toHaveTextContent(
      "Place this item in a location?"
    )
  })

  it("fires onAssign when the CTA is clicked", async () => {
    const user = userEvent.setup()
    const onAssign = vi.fn()
    renderWithProviders({ children: <AssignLocationBanner show onAssign={onAssign} /> })
    await user.click(screen.getByTestId("assign-location-banner-cta"))
    expect(onAssign).toHaveBeenCalledTimes(1)
  })

  it("dismisses on click of the X button (in-session only)", async () => {
    const user = userEvent.setup()
    renderWithProviders({ children: <AssignLocationBanner show onAssign={() => {}} /> })
    expect(screen.getByTestId("assign-location-banner")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: /dismiss/i }))
    expect(screen.queryByTestId("assign-location-banner")).toBeNull()
  })
})
