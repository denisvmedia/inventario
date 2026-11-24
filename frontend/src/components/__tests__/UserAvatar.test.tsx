import { afterEach, describe, expect, it, vi } from "vitest"
import { act, render, screen, within } from "@testing-library/react"
import { axe } from "jest-axe"

import { UserAvatar } from "@/components/UserAvatar"
import { stubImageLoading } from "@/test/image"

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("<UserAvatar />", () => {
  it("shows the initials when there is no photo", () => {
    render(<UserAvatar name="Alex Johnson" email="alex@example.com" testId="avatar" />)
    const avatar = screen.getByTestId("avatar")
    expect(within(avatar).getByTestId("user-avatar-initials")).toHaveTextContent("AJ")
    expect(within(avatar).queryByTestId("user-avatar-image")).not.toBeInTheDocument()
  })

  it("shows the photo once it has loaded", async () => {
    stubImageLoading()
    render(<UserAvatar name="Alex Johnson" src="/api/v1/avatars/u1?sig=a" testId="avatar" />)
    const image = await screen.findByTestId("user-avatar-image")
    expect(image).toHaveAttribute("src", "/api/v1/avatars/u1?sig=a")
    expect(screen.queryByTestId("user-avatar-initials")).not.toBeInTheDocument()
  })

  it("keeps the initials when the photo fails to load", async () => {
    // An expired URL, or a photo removed since the list was fetched.
    stubImageLoading(() => true)
    render(<UserAvatar name="Alex Johnson" src="/api/v1/avatars/u1?sig=old" testId="avatar" />)
    // Let the stub fire its error event before looking.
    await act(() => Promise.resolve())
    expect(screen.getByTestId("user-avatar-initials")).toHaveTextContent("AJ")
    expect(screen.queryByTestId("user-avatar-image")).not.toBeInTheDocument()
  })

  it("is hidden from assistive technology, since the name is printed next to it", async () => {
    stubImageLoading()
    const { container } = render(
      <UserAvatar name="Alex Johnson" src="/api/v1/avatars/u1?sig=a" testId="avatar" />
    )
    await screen.findByTestId("user-avatar-image")
    expect(screen.getByTestId("avatar")).toHaveAttribute("aria-hidden", "true")
    expect(await axe(container)).toHaveNoViolations()
  })
})
