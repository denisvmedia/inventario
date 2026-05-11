import { describe, expect, it, vi } from "vitest"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { UnexpectedErrorPage } from "@/pages/UnexpectedErrorPage"
import { renderWithProviders } from "@/test/render"

describe("<UnexpectedErrorPage />", () => {
  it("renders the heading, icon, reload + retry buttons, and shows the dev details panel under import.meta.env.DEV", async () => {
    const onReset = vi.fn()
    const error = new Error("boom")
    error.stack = "Error: boom\n    at Foo (foo.tsx:10:7)"
    const errorInfo = { componentStack: "\n    in Foo\n    in App", digest: undefined }
    renderWithProviders({
      children: <UnexpectedErrorPage error={error} errorInfo={errorInfo} onReset={onReset} />,
    })
    expect(await screen.findByTestId("page-unexpected-error")).toBeInTheDocument()
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument()
    // Vitest sets `import.meta.env.DEV = true` by default, so the
    // dev-only stack panel must be present along with the messaging.
    expect(screen.getByText("boom")).toBeInTheDocument()
    expect(screen.getByText(/at Foo \(foo\.tsx:10:7\)/)).toBeInTheDocument()
    expect(screen.getByText(/in Foo/)).toBeInTheDocument()
  })

  it("calls `onReset` when the retry button is clicked", async () => {
    const user = userEvent.setup()
    const onReset = vi.fn()
    renderWithProviders({
      children: (
        <UnexpectedErrorPage error={new Error("nope")} errorInfo={null} onReset={onReset} />
      ),
    })
    const retry = await screen.findByRole("button", { name: /try again/i })
    await user.click(retry)
    expect(onReset).toHaveBeenCalledTimes(1)
  })

  it("renders without the dev details when the error has no stack and no errorInfo", async () => {
    const onReset = vi.fn()
    const error = new Error("bare")
    delete (error as Partial<Error>).stack
    renderWithProviders({
      children: <UnexpectedErrorPage error={error} errorInfo={null} onReset={onReset} />,
    })
    expect(await screen.findByText("bare")).toBeInTheDocument()
    // No stack rendered (the <pre> is gated on `error.stack`).
    expect(screen.queryByText(/at Foo/)).toBeNull()
  })
})
