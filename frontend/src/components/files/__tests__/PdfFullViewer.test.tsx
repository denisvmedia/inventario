import { beforeAll, describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { initI18n } from "@/i18n"

const BASE_WIDTH = 600
const BASE_HEIGHT = 800

vi.mock("@/lib/pdfjs", () => {
  const page = {
    getViewport: vi.fn(({ scale }: { scale: number }) => ({
      width: BASE_WIDTH * scale,
      height: BASE_HEIGHT * scale,
    })),
    render: vi.fn(() => ({ promise: Promise.resolve(), cancel: vi.fn() })),
  }
  const doc = { numPages: 3, getPage: vi.fn(() => Promise.resolve(page)) }
  return {
    pdfjsLib: {
      GlobalWorkerOptions: { workerSrc: "" },
      getDocument: vi.fn(() => ({
        promise: Promise.resolve(doc),
        destroy: vi.fn(),
        onProgress: undefined,
      })),
    },
  }
})

beforeAll(async () => {
  await initI18n({ lng: "en" })
  HTMLCanvasElement.prototype.getContext = vi.fn(() => ({})) as never
})

async function renderViewer(onClose = vi.fn()) {
  const { PdfFullViewer } = await import("@/components/files/PdfFullViewer")
  render(<PdfFullViewer url="https://example.test/doc.pdf" title="Manual" onClose={onClose} />)
  await screen.findByTestId("pdf-full-page-1")
  return onClose
}

describe("<PdfFullViewer />", () => {
  it("renders the thumbnail rail, all pages (continuous), and the toolbar", async () => {
    await renderViewer()
    // jsdom has no IntersectionObserver, so the continuous strip renders every
    // page eagerly — all three page slots + thumbnails are present.
    expect(screen.getByTestId("pdf-full-page-2")).toBeInTheDocument()
    expect(screen.getByTestId("pdf-full-page-3")).toBeInTheDocument()
    expect(screen.getByTestId("pdf-full-sidebar")).toBeInTheDocument()
    expect(screen.getByTestId("pdf-full-thumb-1")).toBeInTheDocument()
    expect(screen.getByTestId("pdf-full-thumb-3")).toBeInTheDocument()
    // Chrome-like controls, no print / draw / rotate.
    expect(screen.getByTestId("pdf-full-fit")).toBeInTheDocument()
    expect(screen.getByTestId("pdf-full-mode")).toBeInTheDocument()
    expect(screen.getByTestId("pdf-full-download")).toBeInTheDocument()
  })

  it("switches to single-page view and renders only the current page", async () => {
    const user = userEvent.setup()
    await renderViewer()
    await user.click(screen.getByTestId("pdf-full-mode"))
    expect(screen.getByTestId("pdf-full-page-1")).toBeInTheDocument()
    expect(screen.queryByTestId("pdf-full-page-2")).not.toBeInTheDocument()
    expect(screen.queryByTestId("pdf-full-page-3")).not.toBeInTheDocument()
  })

  it("page navigation updates the indicator and the rendered page in single mode", async () => {
    const user = userEvent.setup()
    await renderViewer()
    await user.click(screen.getByTestId("pdf-full-mode")) // paged
    await user.click(screen.getByTestId("pdf-full-next"))
    expect(screen.getByTestId("pdf-full-page-input")).toHaveValue(2)
    expect(screen.getByTestId("pdf-full-page-2")).toBeInTheDocument()
    expect(screen.queryByTestId("pdf-full-page-1")).not.toBeInTheDocument()
  })

  it("manual zoom clears the fit mode and changes the zoom level", async () => {
    const user = userEvent.setup()
    await renderViewer()
    // Area is 0×0 in jsdom (ResizeObserver is a no-op stub), so the fit scale
    // never computes and the level starts at 100%.
    expect(screen.getByTestId("pdf-full-zoom-level")).toHaveTextContent("100%")
    await user.click(screen.getByTestId("pdf-full-zoom-in"))
    expect(screen.getByTestId("pdf-full-zoom-level")).toHaveTextContent("125%")
  })

  it("toggles the thumbnail rail and invokes onClose", async () => {
    const user = userEvent.setup()
    const onClose = await renderViewer()
    await user.click(screen.getByTestId("pdf-full-sidebar-toggle"))
    await waitFor(() => expect(screen.queryByTestId("pdf-full-sidebar")).not.toBeInTheDocument())
    await user.click(screen.getByTestId("pdf-full-close"))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it("drags the page area to pan it when it overflows", async () => {
    await renderViewer()
    const scroller = screen.getByTestId("pdf-full-scroll")
    // jsdom has no layout, so fake overflow geometry + a writable scroll pos.
    let left = 0
    let top = 0
    Object.defineProperty(scroller, "scrollWidth", { configurable: true, value: 1800 })
    Object.defineProperty(scroller, "clientWidth", { configurable: true, value: 700 })
    Object.defineProperty(scroller, "scrollHeight", { configurable: true, value: 2400 })
    Object.defineProperty(scroller, "clientHeight", { configurable: true, value: 800 })
    Object.defineProperty(scroller, "scrollLeft", {
      configurable: true,
      get: () => left,
      set: (v) => {
        left = v
      },
    })
    Object.defineProperty(scroller, "scrollTop", {
      configurable: true,
      get: () => top,
      set: (v) => {
        top = v
      },
    })
    fireEvent.pointerDown(scroller, {
      pointerId: 1,
      pointerType: "mouse",
      button: 0,
      clientX: 300,
      clientY: 300,
    })
    fireEvent.pointerMove(scroller, { pointerId: 1, clientX: 240, clientY: 250 })
    // Dragging up/left reveals content to the right/bottom: 0 - (240-300) = 60,
    // 0 - (250-300) = 50.
    expect(scroller.scrollLeft).toBe(60)
    expect(scroller.scrollTop).toBe(50)
    fireEvent.pointerUp(scroller, { pointerId: 1 })
  })
})
