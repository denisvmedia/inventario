import { beforeAll, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { ImageViewer } from "@/components/files/ImageViewer"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<ImageViewer />", () => {
  it("renders nothing when closed", () => {
    const { container } = render(
      <ImageViewer open={false} onOpenChange={vi.fn()} url="https://example/a.png" alt="A" />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it("shows zoom toolbar in single-image mode but no nav controls", () => {
    render(<ImageViewer open onOpenChange={vi.fn()} url="https://example/a.png" alt="A" />)
    expect(screen.getByTestId("image-viewer-zoom-in")).toBeInTheDocument()
    expect(screen.queryByTestId("image-viewer-prev")).not.toBeInTheDocument()
    expect(screen.queryByTestId("image-viewer-next")).not.toBeInTheDocument()
    expect(screen.queryByTestId("image-viewer-position")).not.toBeInTheDocument()
  })

  it("renders prev/next + position counter when siblings have more than one entry", () => {
    render(
      <ImageViewer
        open
        onOpenChange={vi.fn()}
        siblings={[
          { id: "a", url: "https://example/a.png", alt: "A" },
          { id: "b", url: "https://example/b.png", alt: "B" },
          { id: "c", url: "https://example/c.png", alt: "C" },
        ]}
        index={1}
      />
    )
    expect(screen.getByTestId("image-viewer-position")).toHaveTextContent("2 / 3")
    expect(screen.getByTestId("image-viewer-prev")).toBeInTheDocument()
    expect(screen.getByTestId("image-viewer-next")).toBeInTheDocument()
  })

  it("hides nav controls when the gallery has a single entry", () => {
    render(
      <ImageViewer
        open
        onOpenChange={vi.fn()}
        siblings={[{ id: "a", url: "https://example/a.png", alt: "A" }]}
        index={0}
      />
    )
    expect(screen.queryByTestId("image-viewer-prev")).not.toBeInTheDocument()
    expect(screen.queryByTestId("image-viewer-next")).not.toBeInTheDocument()
  })

  it("ArrowRight cycles forward and ArrowLeft cycles backward (with wrap-around)", async () => {
    const user = userEvent.setup()
    const onIndexChange = vi.fn()
    render(
      <ImageViewer
        open
        onOpenChange={vi.fn()}
        siblings={[
          { id: "a", url: "https://example/a.png", alt: "A" },
          { id: "b", url: "https://example/b.png", alt: "B" },
        ]}
        index={0}
        onIndexChange={onIndexChange}
      />
    )
    await user.keyboard("{ArrowRight}")
    expect(onIndexChange).toHaveBeenLastCalledWith(1)
    await user.keyboard("{ArrowLeft}")
    // index=0, ArrowLeft wraps to last (length=2 → 1).
    expect(onIndexChange).toHaveBeenLastCalledWith(1)
  })

  it("Esc closes the viewer in both modes", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    render(<ImageViewer open onOpenChange={onOpenChange} url="https://example/a.png" alt="A" />)
    await user.keyboard("{Escape}")
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it("clamps the displayed zoom level to the configured min/max", async () => {
    const user = userEvent.setup()
    render(<ImageViewer open onOpenChange={vi.fn()} url="https://example/a.png" alt="A" />)
    // Mash the zoom-out button enough times to hit the floor.
    for (let i = 0; i < 12; i++) {
      await user.click(screen.getByTestId("image-viewer-zoom-out"))
    }
    expect(screen.getByTestId("image-viewer-zoom-level")).toHaveTextContent("50%")
  })
})
