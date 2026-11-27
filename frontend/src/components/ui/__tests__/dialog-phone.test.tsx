import { StrictMode, useState } from "react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { BrowserRouter, Route, Routes, useLocation, useNavigate } from "react-router-dom"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  type DialogContentVariant,
} from "@/components/ui/dialog"
import { TOAST_BOTTOM_INSET_PROPERTY } from "@/components/ui/dialog-phone"

// What a "fullscreen-mobile" dialog does on a phone (#2931): the back gesture
// closes it, it fills the part of the screen the on-screen keyboard leaves,
// and toasts keep clear of its footer. jsdom has no keyboard and no layout,
// so a fake visualViewport stands in for the keyboard; the back gesture is a
// step back in jsdom's history. e2e/tests/phone-dialogs.spec.ts runs the
// back gesture and the toasts in a browser at 390x844.

const BELOW_SM = "(width < 40rem)"
const START_URL = window.location.href

function setWidth(belowSm: boolean) {
  vi.spyOn(window, "matchMedia").mockImplementation((query: string) => ({
    matches: belowSm && query === BELOW_SM,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

class FakeVisualViewport extends EventTarget {
  offsetLeft = 0
  offsetTop = 0
  pageLeft = 0
  pageTop = 0
  width = 390
  height = 844
  scale = 1
  onresize = null
  onscroll = null
  onscrollend = null
}

function installVisualViewport() {
  const viewport = new FakeVisualViewport()
  Object.defineProperty(window, "visualViewport", { configurable: true, value: viewport })
  Object.defineProperty(window, "innerHeight", { configurable: true, value: 844 })
  return viewport
}

function moveViewport(viewport: FakeVisualViewport, change: Partial<FakeVisualViewport>) {
  act(() => {
    Object.assign(viewport, change)
    viewport.dispatchEvent(new Event("resize"))
  })
}

function dialogEntries(): string[] {
  const state = window.history.state as { inventarioDialogs?: string[] } | null
  return state?.inventarioDialogs ?? []
}

// A dialog whose owner decides whether a close goes through, as a form with
// an unsaved-changes question does.
function Harness({
  variant = "fullscreen-mobile",
  refuseClose = false,
  onClose = () => {},
}: {
  variant?: DialogContentVariant
  refuseClose?: boolean
  onClose?: () => void
}) {
  const [open, setOpen] = useState(true)
  return (
    <>
      <p>The page under the dialog</p>
      <Dialog
        open={open}
        onOpenChange={(next) => {
          if (next) return setOpen(true)
          onClose()
          if (!refuseClose) setOpen(false)
        }}
      >
        <DialogContent variant={variant} data-testid="content">
          <DialogHeader data-testid="header">
            <DialogTitle>New location</DialogTitle>
            <DialogDescription>Where your things live.</DialogDescription>
          </DialogHeader>
          <form>
            <label htmlFor="name">Name</label>
            <input id="name" />
            <DialogFooter data-testid="footer">
              <Button type="button">Cancel</Button>
              <Button type="submit">Create</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}

beforeEach(() => {
  // Each test starts on an entry of its own, so a step back never leaves
  // the test's page.
  window.history.pushState({ test: true }, "")
})

afterEach(async () => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  // Unmount here and let a dialog's step back over its history entry land,
  // so no test, and no later file in this worker, starts with one under way.
  cleanup()
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)))
  window.history.replaceState(null, "", START_URL)
  Reflect.deleteProperty(window, "visualViewport")
  Object.defineProperty(window, "innerHeight", { configurable: true, value: 768 })
})

describe("full-screen dialog and the back gesture", () => {
  it("closes on a step back and stays on the page", async () => {
    setWidth(true)
    const onClose = vi.fn()
    const path = window.location.pathname
    const length = window.history.length
    render(<Harness onClose={onClose} />)
    expect(window.history.length).toBe(length + 1)
    expect(dialogEntries()).toHaveLength(1)

    act(() => window.history.back())

    await waitFor(() => expect(screen.queryByTestId("content")).not.toBeInTheDocument())
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(window.location.pathname).toBe(path)
    expect(screen.getByText("The page under the dialog")).toBeInTheDocument()
    expect(dialogEntries()).toEqual([])
  })

  it("asks again on the next step back when the close is refused", async () => {
    setWidth(true)
    const onClose = vi.fn()
    render(<Harness refuseClose onClose={onClose} />)

    act(() => window.history.back())
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1))
    // Still open, with its history entry back in place.
    await waitFor(() => expect(dialogEntries()).toHaveLength(1))
    expect(screen.getByTestId("content")).toBeInTheDocument()

    act(() => window.history.back())
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(2))
    expect(screen.getByTestId("content")).toBeInTheDocument()
  })

  it("takes its history entry off when it closes with the close button", async () => {
    setWidth(true)
    const user = userEvent.setup()
    const length = window.history.length
    render(<Harness />)
    expect(window.history.length).toBe(length + 1)

    await user.click(within(screen.getByTestId("header")).getByRole("button", { name: "Close" }))

    expect(screen.queryByTestId("content")).not.toBeInTheDocument()
    await waitFor(() => expect(dialogEntries()).toEqual([]))
    expect((window.history.state as { test?: boolean }).test).toBe(true)
  })

  it("lets a page the app opens on close take the dialog's entry", async () => {
    setWidth(true)
    const user = userEvent.setup()
    const length = window.history.length
    render(<Harness onClose={() => window.history.pushState({ page: "item" }, "", "/item-1")} />)

    await user.click(within(screen.getByTestId("header")).getByRole("button", { name: "Close" }))

    // The item page replaced the dialog's entry instead of stacking on it.
    expect(window.history.length).toBe(length + 1)
    expect(window.location.pathname).toBe("/item-1")
    expect((window.history.state as { page?: string }).page).toBe("item")
    // So one step back returns to the page the dialog was opened on.
    act(() => window.history.back())
    await waitFor(() => expect((window.history.state as { test?: boolean }).test).toBe(true))
  })

  it("moves a URL the app replaces on close to the entry below", async () => {
    setWidth(true)
    const user = userEvent.setup()
    const router = vi.fn()
    window.addEventListener("popstate", router)
    render(<Harness onClose={() => window.history.replaceState({ page: "list" }, "", "/list")} />)

    await user.click(within(screen.getByTestId("header")).getByRole("button", { name: "Close" }))

    await waitFor(() => expect(dialogEntries()).toEqual([]))
    expect(window.location.pathname).toBe("/list")
    expect(window.history.state).toEqual({ page: "list" })
    // The dialog's own step back is never seen by the rest of the page.
    expect(router).not.toHaveBeenCalled()
    window.removeEventListener("popstate", router)
  })

  it("keeps a URL the app replaces while the dialog steps back", async () => {
    setWidth(true)
    const user = userEvent.setup()
    const back = window.history.back.bind(window.history)
    // The app replaces its URL after the dialog asked for the step back and
    // before the step lands, as a route modal does after its exit animation.
    vi.spyOn(window.history, "back").mockImplementation(() => {
      window.history.replaceState({ page: "item" }, "", "/item-2")
      back()
    })
    render(<Harness />)

    await user.click(within(screen.getByTestId("header")).getByRole("button", { name: "Close" }))

    await waitFor(() => expect(window.history.back).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(window.history.state).toEqual({ page: "item" }))
    expect(window.location.pathname).toBe("/item-2")
  })

  it("opens a page the app navigates to while the dialog steps back", async () => {
    setWidth(true)
    const user = userEvent.setup()
    window.history.pushState(null, "", "/list")
    function GoToItem() {
      const navigate = useNavigate()
      return (
        <button type="button" hidden data-testid="go-to-item" onClick={() => navigate("/item-3")} />
      )
    }
    function RouterPath() {
      return <p data-testid="router-path">{useLocation().pathname}</p>
    }
    const back = window.history.back.bind(window.history)
    // The app navigates after the dialog asked for its step back and before
    // the step lands, as after a fast request that follows a close.
    vi.spyOn(window.history, "back").mockImplementationOnce(() => {
      back()
      screen.getByTestId("go-to-item").click()
    })
    render(
      <BrowserRouter>
        <GoToItem />
        <RouterPath />
        <Routes>
          <Route path="/list" element={<Harness />} />
          <Route path="/item-3" element={<p>The item page</p>} />
        </Routes>
      </BrowserRouter>
    )

    await user.click(within(screen.getByTestId("header")).getByRole("button", { name: "Close" }))
    await waitFor(() => expect(window.history.back).toHaveBeenCalledTimes(1))
    // Let the step back land.
    await act(() => new Promise((resolve) => setTimeout(resolve, 50)))

    expect(screen.getByText("The item page")).toBeInTheDocument()
    expect(screen.getByTestId("router-path")).toHaveTextContent("/item-3")
    expect(window.location.pathname).toBe("/item-3")
    expect(dialogEntries()).toEqual([])

    // One step back returns to the list, in the browser and in the router.
    act(() => window.history.back())
    await waitFor(() => expect(screen.getByTestId("router-path")).toHaveTextContent("/list"))
    expect(window.location.pathname).toBe("/list")
  })

  it("does not hold later pages back when its step back never lands", async () => {
    setWidth(true)
    const user = userEvent.setup()
    // The browser drops the dialog's step back.
    vi.spyOn(window.history, "back").mockImplementationOnce(() => {})
    render(<Harness />)

    await user.click(within(screen.getByTestId("header")).getByRole("button", { name: "Close" }))
    await waitFor(() => expect(window.history.back).toHaveBeenCalledTimes(1))

    const length = window.history.length
    const now = performance.now()
    vi.spyOn(performance, "now").mockReturnValue(now + 5000)
    window.history.pushState({ page: "later" }, "", "/later")
    expect(window.history.length).toBe(length + 1)
    expect(window.location.pathname).toBe("/later")
  })

  it("pushes one entry under StrictMode", async () => {
    setWidth(true)
    const length = window.history.length
    render(
      <StrictMode>
        <Harness />
      </StrictMode>
    )
    await new Promise((resolve) => setTimeout(resolve, 10))
    expect(window.history.length).toBe(length + 1)
    expect(dialogEntries()).toHaveLength(1)
  })

  it("leaves the history alone from sm up and for the default dialog", () => {
    setWidth(false)
    const length = window.history.length
    const { unmount } = render(<Harness />)
    expect(window.history.length).toBe(length)
    unmount()

    setWidth(true)
    render(<Harness variant="default" />)
    expect(window.history.length).toBe(length)
    expect(dialogEntries()).toEqual([])
  })
})

describe("full-screen dialog and the on-screen keyboard", () => {
  it("fills the part of the screen above the keyboard", () => {
    setWidth(true)
    const viewport = installVisualViewport()
    render(<Harness />)
    const content = screen.getByTestId("content")
    // Nothing covers the screen: the layout's own 100dvh applies.
    expect(content.style.height).toBe("")
    expect(content.style.top).toBe("")

    moveViewport(viewport, { height: 500 })
    expect(content.style.height).toBe("500px")
    expect(content.style.top).toBe("0px")

    // iOS pans the visual viewport to keep the focused field in sight.
    moveViewport(viewport, { offsetTop: 120, height: 450 })
    expect(content.style.top).toBe("120px")
    expect(content.style.height).toBe("450px")

    moveViewport(viewport, { offsetTop: 0, height: 844 })
    expect(content.style.height).toBe("")
  })

  it("does not follow a pinch zoom", () => {
    setWidth(true)
    const viewport = installVisualViewport()
    render(<Harness />)
    moveViewport(viewport, { scale: 2, height: 422, offsetTop: 200 })
    expect(screen.getByTestId("content").style.height).toBe("")
  })

  it("keeps the card from sm up", () => {
    setWidth(false)
    const viewport = installVisualViewport()
    render(<Harness />)
    moveViewport(viewport, { height: 500 })
    expect(screen.getByTestId("content").style.height).toBe("")
  })
})

describe("full-screen dialog and toasts", () => {
  function toastInset() {
    return document.documentElement.style.getPropertyValue(TOAST_BOTTOM_INSET_PROPERTY)
  }

  it("keeps toasts above its footer and the keyboard while it is open", async () => {
    setWidth(true)
    const viewport = installVisualViewport()
    vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockImplementation(function (
      this: HTMLElement
    ) {
      return this.dataset.slot === "dialog-footer" ? 61 : 0
    })
    // The suite's ResizeObserver stub reports nothing; this one reports the
    // first size, as a browser does.
    vi.stubGlobal(
      "ResizeObserver",
      class {
        callback: ResizeObserverCallback
        constructor(callback: ResizeObserverCallback) {
          this.callback = callback
        }
        observe() {
          this.callback([], this as unknown as ResizeObserver)
        }
        unobserve() {}
        disconnect() {}
      }
    )
    const user = userEvent.setup()
    render(<Harness />)
    await waitFor(() => expect(toastInset()).toBe("61px"))

    moveViewport(viewport, { height: 500 })
    expect(toastInset()).toBe(`${61 + 344}px`)

    await user.click(within(screen.getByTestId("header")).getByRole("button", { name: "Close" }))
    expect(toastInset()).toBe("")
  })

  it("sets nothing from sm up", () => {
    setWidth(false)
    render(<Harness />)
    expect(toastInset()).toBe("")
  })
})
