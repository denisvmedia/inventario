import { describe, expect, it } from "vitest"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { InviteBanner } from "@/components/InviteBanner"
import { renderWithProviders } from "@/test/render"

describe("InviteBanner", () => {
  it("renders nothing when count is zero", () => {
    renderWithProviders({ children: <InviteBanner count={0} /> })
    expect(screen.queryByTestId("invite-banner")).toBeNull()
  })

  it("shows pluralised text for >1 invites", () => {
    renderWithProviders({ children: <InviteBanner count={3} /> })
    expect(screen.getByTestId("invite-banner")).toHaveTextContent("You have 3 pending invites.")
  })

  it("shows singular text for exactly one invite", () => {
    renderWithProviders({ children: <InviteBanner count={1} /> })
    expect(screen.getByTestId("invite-banner")).toHaveTextContent("You have 1 pending invite.")
  })

  it("hides the View action when viewHref is null (the default)", () => {
    renderWithProviders({ children: <InviteBanner count={2} /> })
    expect(screen.queryByRole("button", { name: /view/i })).toBeNull()
  })

  it("renders the View action when an explicit viewHref is passed", () => {
    renderWithProviders({
      children: <InviteBanner count={2} viewHref="/profile" />,
    })
    expect(screen.getByRole("button", { name: /view/i })).toBeInTheDocument()
  })

  it("dismisses on click of the X button (in-session only)", async () => {
    const user = userEvent.setup()
    renderWithProviders({ children: <InviteBanner count={2} /> })
    expect(screen.getByTestId("invite-banner")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: /dismiss invite banner/i }))
    expect(screen.queryByTestId("invite-banner")).toBeNull()
  })
})
