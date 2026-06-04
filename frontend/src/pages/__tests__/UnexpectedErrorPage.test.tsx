import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { UnexpectedErrorPage } from "@/pages/UnexpectedErrorPage"
import { useSystemInfo } from "@/features/system/hooks"
import { renderWithProviders } from "@/test/render"

// UnexpectedErrorPage reads system.debug to decide whether to reveal the
// stack panel. Mock the hook so tests don't hit the network and can drive
// the backend flag directly.
vi.mock("@/features/system/hooks", () => ({
  useSystemInfo: vi.fn(() => ({ data: undefined })),
}))
const mockUseSystemInfo = vi.mocked(useSystemInfo)
function mockSystem(data: { debug?: boolean } | undefined) {
  mockUseSystemInfo.mockReturnValue({ data } as unknown as ReturnType<typeof useSystemInfo>)
}

beforeEach(() => {
  mockSystem(undefined)
})
afterEach(() => {
  vi.unstubAllEnvs()
  try {
    window.localStorage.clear()
  } catch {
    /* storage may be unavailable; ignore */
  }
})

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
    // details panel must be present along with the messaging.
    expect(screen.getByTestId("unexpected-error-details")).toBeInTheDocument()
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

  it("omits the stack <pre> when error.stack is missing (details card still renders the message)", async () => {
    // Vitest runs with `import.meta.env.DEV=true`, so the details card is
    // always rendered. What this test pins is the inner stack and
    // componentStack `<pre>` blocks staying out when `error.stack` and
    // `errorInfo` are absent — the message line is still shown.
    const onReset = vi.fn()
    const error = new Error("bare")
    delete (error as Partial<Error>).stack
    renderWithProviders({
      children: <UnexpectedErrorPage error={error} errorInfo={null} onReset={onReset} />,
    })
    expect(await screen.findByText("bare")).toBeInTheDocument()
    // No stack rendered (the inner <pre> is gated on `error.stack`).
    expect(screen.queryByText(/at Foo/)).toBeNull()
  })
})

describe("<UnexpectedErrorPage /> debug-panel gating (#1965)", () => {
  it("hides the details panel for production users (DEV off, no system.debug, no override)", async () => {
    vi.stubEnv("DEV", false)
    renderWithProviders({
      children: (
        <UnexpectedErrorPage error={new Error("secret-stack")} errorInfo={null} onReset={vi.fn()} />
      ),
    })
    expect(await screen.findByTestId("page-unexpected-error")).toBeInTheDocument()
    // The friendly copy shows, but the error message + stack panel must NOT.
    expect(screen.queryByTestId("unexpected-error-details")).toBeNull()
    expect(screen.queryByText("secret-stack")).toBeNull()
  })

  it("shows the panel when the backend reports debug mode (preview/demo: system.debug=true)", async () => {
    vi.stubEnv("DEV", false)
    mockSystem({ debug: true })
    renderWithProviders({
      children: (
        <UnexpectedErrorPage error={new Error("preview-boom")} errorInfo={null} onReset={vi.fn()} />
      ),
    })
    expect(await screen.findByTestId("unexpected-error-details")).toBeInTheDocument()
    expect(screen.getByText("preview-boom")).toBeInTheDocument()
  })

  it("shows the panel via the ?debug=1 localStorage override even when DEV is off and system.debug is false", async () => {
    vi.stubEnv("DEV", false)
    mockSystem({ debug: false })
    window.localStorage.setItem("inventario:debug-ui", "1")
    renderWithProviders({
      children: (
        <UnexpectedErrorPage
          error={new Error("override-boom")}
          errorInfo={null}
          onReset={vi.fn()}
        />
      ),
    })
    expect(await screen.findByTestId("unexpected-error-details")).toBeInTheDocument()
    expect(screen.getByText("override-boom")).toBeInTheDocument()
  })
})
