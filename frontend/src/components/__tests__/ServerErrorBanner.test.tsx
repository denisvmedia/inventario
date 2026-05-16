import { describe, expect, it, vi } from "vitest"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { renderWithProviders } from "@/test/render"

describe("<ServerErrorBanner />", () => {
  it("renders nothing when error is null", () => {
    const { container } = renderWithProviders({
      children: <ServerErrorBanner error={null} testId="b" />,
    })
    expect(container).toBeEmptyDOMElement()
  })

  it("shows the network title and a Retry button when onRetry is provided", async () => {
    const user = userEvent.setup()
    const onRetry = vi.fn()
    renderWithProviders({
      children: (
        <ServerErrorBanner
          error={{ kind: "network", message: "Failed to fetch" }}
          onRetry={onRetry}
          testId="b"
        />
      ),
    })
    expect(screen.getByText("Can't reach the server")).toBeInTheDocument()
    expect(screen.getByText("Failed to fetch")).toBeInTheDocument()
    const retry = screen.getByRole("button", { name: /try again/i })
    await user.click(retry)
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it("hides Retry for validation kind even when onRetry is provided", () => {
    // Validation/conflict need the user to edit before a re-submit
    // could succeed — see lib/server-error::isRetryableKind. The
    // banner must not surface a Retry that would just stack failures.
    renderWithProviders({
      children: (
        <ServerErrorBanner
          error={{ kind: "validation", message: "Name is required" }}
          onRetry={() => {}}
          testId="b"
        />
      ),
    })
    expect(screen.getByText("Please check the highlighted fields")).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /try again/i })).not.toBeInTheDocument()
  })

  it("hides Retry for conflict kind", () => {
    renderWithProviders({
      children: (
        <ServerErrorBanner
          error={{ kind: "conflict", message: "Already taken" }}
          onRetry={() => {}}
          testId="b"
        />
      ),
    })
    expect(screen.getByText("This conflicts with another change")).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /try again/i })).not.toBeInTheDocument()
  })

  it("hides Retry when onRetry is omitted, even for retryable kinds", () => {
    renderWithProviders({
      children: <ServerErrorBanner error={{ kind: "unknown", message: "Boom" }} testId="b" />,
    })
    expect(screen.getByText("Something went wrong")).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /try again/i })).not.toBeInTheDocument()
  })

  it("disables Retry while isRetrying is true", () => {
    renderWithProviders({
      children: (
        <ServerErrorBanner
          error={{ kind: "network", message: "Failed to fetch" }}
          onRetry={() => {}}
          isRetrying
          testId="b"
        />
      ),
    })
    expect(screen.getByRole("button", { name: /try again/i })).toBeDisabled()
  })

  it("exposes data-error-kind on the alert for assertions and styling hooks", () => {
    renderWithProviders({
      children: <ServerErrorBanner error={{ kind: "validation", message: "x" }} testId="b" />,
    })
    expect(screen.getByTestId("b")).toHaveAttribute("data-error-kind", "validation")
  })
})
