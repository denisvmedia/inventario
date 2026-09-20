import { describe, expect, it, vi } from "vitest"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { ListLoadError } from "@/components/common/ListLoadError"
import { renderWithProviders } from "@/test/render"

// A failed list must not fall through to the empty state: "No items" after a
// dropped connection reads as data loss (#2098).
describe("<ListLoadError />", () => {
  it("says the list failed rather than that it is empty", () => {
    renderWithProviders({ children: <ListLoadError testId="x-error" /> })

    expect(screen.getByTestId("x-error")).toBeInTheDocument()
    expect(screen.getByText(/does not mean the list is empty/i)).toBeInTheDocument()
  })

  it("retries on request", async () => {
    const onRetry = vi.fn()
    renderWithProviders({ children: <ListLoadError testId="x-error" onRetry={onRetry} /> })

    await userEvent.click(screen.getByRole("button", { name: /try again/i }))

    expect(onRetry).toHaveBeenCalledOnce()
  })

  it("omits the button when there is nothing to retry", () => {
    renderWithProviders({ children: <ListLoadError testId="x-error" /> })

    expect(screen.queryByRole("button")).not.toBeInTheDocument()
  })
})
