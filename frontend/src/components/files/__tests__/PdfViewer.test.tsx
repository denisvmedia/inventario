import { beforeAll, describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { initI18n } from "@/i18n"

// Drive the canvas size off `scale` so a zoom change is observable as a
// change in the canvas's *displayed* (CSS) size. The component renders at
// `scale * devicePixelRatio` intrinsic pixels but pins the CSS size to the
// scale-only viewport, which (in jsdom, dpr === 1) is 600×800 per unit scale.
const BASE_WIDTH = 600
const BASE_HEIGHT = 800

const cancelMock = vi.fn()
const renderMock = vi.fn(() => ({ promise: Promise.resolve(), cancel: cancelMock }))
const getViewportMock = vi.fn(({ scale }: { scale: number }) => ({
  width: BASE_WIDTH * scale,
  height: BASE_HEIGHT * scale,
}))

vi.mock("@/lib/pdfjs", () => {
  const pageProxy = {
    getViewport: getViewportMock,
    render: renderMock,
  }
  const doc = {
    numPages: 3,
    getPage: vi.fn(() => Promise.resolve(pageProxy)),
  }
  return {
    pdfjsLib: {
      GlobalWorkerOptions: { workerSrc: "" },
      getDocument: vi.fn(() => ({
        promise: Promise.resolve(doc),
        destroy: vi.fn(),
      })),
    },
  }
})

beforeAll(async () => {
  await initI18n({ lng: "en" })
  // jsdom has no 2d canvas backend; the viewer bails when getContext is
  // null, so hand it a stub context to let the render path run.
  HTMLCanvasElement.prototype.getContext = vi.fn(() => ({})) as never
})

async function renderViewer() {
  const { PdfViewer } = await import("@/components/files/PdfViewer")
  render(<PdfViewer url="https://example.test/doc.pdf" />)
  return (await screen.findByTestId("pdf-viewer-canvas")) as HTMLCanvasElement
}

describe("<PdfViewer />", () => {
  it("does not clamp the canvas display width (regression for #1963)", async () => {
    const canvas = await renderViewer()
    // The historical bug pinned the canvas with `max-w-full`, capping the
    // on-screen size at the container width regardless of zoom.
    expect(canvas.className).not.toContain("max-w-full")
  })

  it("grows the canvas display size when zooming in past fit-width", async () => {
    const user = userEvent.setup()
    const canvas = await renderViewer()

    // DEFAULT_SCALE = 1.5 → 600 * 1.5 = 900 CSS px.
    await waitFor(() => expect(canvas.style.width).toBe("900px"))

    const zoomIn = screen.getByTestId("pdf-viewer-zoom-in")
    await user.click(zoomIn) // 1.5 → 1.75
    await waitFor(() => expect(canvas.style.width).toBe("1050px"))

    // Keep zooming well past the old ~125% fit cap; the displayed size must
    // keep tracking the zoom indicator instead of stalling.
    await user.click(zoomIn) // 1.75 → 2.0
    await user.click(zoomIn) // 2.0  → 2.25
    await user.click(zoomIn) // 2.25 → 2.5
    await user.click(zoomIn) // 2.5  → 2.75
    await user.click(zoomIn) // 2.75 → 3.0 (MAX_SCALE)
    await waitFor(() => expect(canvas.style.width).toBe("1800px"))
    expect(screen.getByTestId("pdf-viewer-zoom-level")).toHaveTextContent("300%")
  })

  it("cancels the in-flight pdf.js render on cleanup (no overlapping render)", async () => {
    cancelMock.mockClear()
    const { PdfViewer } = await import("@/components/files/PdfViewer")
    const { unmount } = render(<PdfViewer url="https://example.test/doc.pdf" />)
    const canvas = (await screen.findByTestId("pdf-viewer-canvas")) as HTMLCanvasElement
    // Once the canvas has been sized, the render task has been assigned and is
    // cancellable; unmount must tear it down so a follow-up render can't start
    // a second render() on the same canvas.
    await waitFor(() => expect(canvas.style.width).toBe("900px"))
    unmount()
    expect(cancelMock).toHaveBeenCalled()
  })

  // jsdom has no layout engine, so fake the overflow geometry + a writable
  // scroll position to exercise the drag-to-pan handlers.
  function stubScroller(
    el: HTMLElement,
    dims: { scrollWidth: number; clientWidth: number; scrollHeight: number; clientHeight: number }
  ) {
    let left = 0
    let top = 0
    Object.defineProperty(el, "scrollWidth", { configurable: true, value: dims.scrollWidth })
    Object.defineProperty(el, "clientWidth", { configurable: true, value: dims.clientWidth })
    Object.defineProperty(el, "scrollHeight", { configurable: true, value: dims.scrollHeight })
    Object.defineProperty(el, "clientHeight", { configurable: true, value: dims.clientHeight })
    Object.defineProperty(el, "scrollLeft", {
      configurable: true,
      get: () => left,
      set: (v) => {
        left = v
      },
    })
    Object.defineProperty(el, "scrollTop", {
      configurable: true,
      get: () => top,
      set: (v) => {
        top = v
      },
    })
  }

  it("drags the overflowing page to pan it (#1963 follow-up)", async () => {
    await renderViewer()
    const scroller = screen.getByTestId("pdf-viewer-scroll")
    stubScroller(scroller, {
      scrollWidth: 1836,
      clientWidth: 669,
      scrollHeight: 2376,
      clientHeight: 800,
    })

    fireEvent.pointerDown(scroller, {
      pointerId: 1,
      pointerType: "mouse",
      button: 0,
      clientX: 200,
      clientY: 200,
    })
    fireEvent.pointerMove(scroller, { pointerId: 1, clientX: 150, clientY: 160 })
    // Dragging the page left/up reveals content to its right/bottom:
    // scrollLeft = 0 - (150 - 200) = 50, scrollTop = 0 - (160 - 200) = 40.
    expect(scroller.scrollLeft).toBe(50)
    expect(scroller.scrollTop).toBe(40)

    fireEvent.pointerUp(scroller, { pointerId: 1 })
    // After release the page no longer follows the pointer.
    fireEvent.pointerMove(scroller, { pointerId: 1, clientX: 0, clientY: 0 })
    expect(scroller.scrollLeft).toBe(50)
    expect(scroller.scrollTop).toBe(40)
  })

  it("does not pan when the page already fits (no overflow)", async () => {
    await renderViewer()
    const scroller = screen.getByTestId("pdf-viewer-scroll")
    stubScroller(scroller, {
      scrollWidth: 300,
      clientWidth: 669,
      scrollHeight: 396,
      clientHeight: 800,
    })

    fireEvent.pointerDown(scroller, {
      pointerId: 1,
      pointerType: "mouse",
      button: 0,
      clientX: 200,
      clientY: 200,
    })
    fireEvent.pointerMove(scroller, { pointerId: 1, clientX: 100, clientY: 100 })
    expect(scroller.scrollLeft).toBe(0)
    expect(scroller.scrollTop).toBe(0)
  })
})
