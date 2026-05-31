import { describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"

import { CommitBadge } from "@/components/CommitBadge"
import { useSystemInfo } from "@/features/system/hooks"

vi.mock("@/features/system/hooks", () => ({
  useSystemInfo: vi.fn(),
}))

const mockUseSystemInfo = vi.mocked(useSystemInfo)

// Only `data` is read by the component; cast a minimal shape.
function withData(data: unknown) {
  mockUseSystemInfo.mockReturnValue({ data } as unknown as ReturnType<typeof useSystemInfo>)
}

describe("CommitBadge", () => {
  it("renders the 7-char short hash from a full SHA, full SHA + version in the tooltip", () => {
    withData({ commit: "e8143282abcdef0123456789abcdef0123456789", version: "1.2.3" })
    render(<CommitBadge />)
    const badge = screen.getByText("e814328")
    expect(badge).toBeInTheDocument()
    expect(badge).toHaveAttribute("title", "1.2.3 · e8143282abcdef0123456789abcdef0123456789")
  })

  it("stays a hidden-on-mobile, click-through watermark (acceptance criteria)", () => {
    withData({ commit: "e814328", version: "1.2.3" })
    render(<CommitBadge />)
    const badge = screen.getByText("e814328")
    // hidden by default, only shown from the `sm` breakpoint up
    expect(badge).toHaveClass("hidden", "sm:block")
    // never intercepts clicks on the content beneath it
    expect(badge).toHaveClass("pointer-events-none")
  })

  it("renders an already-short hash unchanged (local make build)", () => {
    withData({ commit: "e814328", version: "dev" })
    render(<CommitBadge />)
    expect(screen.getByText("e814328")).toBeInTheDocument()
  })

  it("renders nothing when the commit is unknown (dev/tests)", () => {
    withData({ commit: "unknown", version: "dev" })
    const { container } = render(<CommitBadge />)
    expect(container).toBeEmptyDOMElement()
  })

  it("renders nothing before the /system response lands", () => {
    withData(undefined)
    const { container } = render(<CommitBadge />)
    expect(container).toBeEmptyDOMElement()
  })
})
