import { describe, expect, it, vi } from "vitest"
import { createEvent, fireEvent, render, screen } from "@testing-library/react"

import { DropOverlay } from "@/components/files/DropOverlay"
import { useFileDropZone } from "@/components/files/useFileDropZone"

interface HarnessProps {
  onFiles: (files: File[]) => void
  disabled?: boolean
}

// Tiny harness component that exercises the public surface of the
// hook (bindProps + isDragging) so the test reads as a real consumer
// would use it. Keeps us out of the renderHook-with-context weeds for
// what is a pure DOM-event hook.
function Harness({ onFiles, disabled }: HarnessProps) {
  const dropZone = useFileDropZone({ onFiles, disabled })
  return (
    <div
      data-testid="zone"
      style={{ position: "relative", width: 200, height: 200 }}
      {...dropZone.bindProps}
    >
      {dropZone.isDragging ? <DropOverlay label="Drop here" /> : null}
      <span data-testid="dragging-flag">{dropZone.isDragging ? "yes" : "no"}</span>
      <div data-testid="child" />
    </div>
  )
}

// jsdom's DataTransfer is incomplete (no constructor, no files setter).
// We hand-roll an event-init payload that mirrors the parts the hook
// actually reads: `types` (must include "Files" to count as a file
// drag), `files`, and `dropEffect` (write target).
function fileDragInit(files: File[] = [], includeFilesType = true): EventInit {
  const types = includeFilesType ? ["Files"] : ["text/plain"]
  return {
    bubbles: true,
    cancelable: true,
    // @ts-expect-error — partial DataTransfer is what the hook needs.
    dataTransfer: {
      types,
      files,
      dropEffect: "none",
    },
  }
}

describe("useFileDropZone", () => {
  it("flips isDragging on dragenter and renders the overlay", () => {
    render(<Harness onFiles={() => {}} />)
    const zone = screen.getByTestId("zone")

    expect(screen.getByTestId("dragging-flag").textContent).toBe("no")
    expect(screen.queryByTestId("entity-drop-overlay")).not.toBeInTheDocument()

    fireEvent.dragEnter(zone, fileDragInit([new File(["x"], "a.png", { type: "image/png" })]))

    expect(screen.getByTestId("dragging-flag").textContent).toBe("yes")
    expect(screen.getByTestId("entity-drop-overlay")).toBeInTheDocument()
  })

  it("ignores non-file drags (text/url payloads) and never highlights", () => {
    const onFiles = vi.fn()
    render(<Harness onFiles={onFiles} />)
    const zone = screen.getByTestId("zone")

    fireEvent.dragEnter(zone, fileDragInit([], false))
    fireEvent.dragOver(zone, fileDragInit([], false))
    fireEvent.drop(zone, fileDragInit([], false))

    expect(screen.getByTestId("dragging-flag").textContent).toBe("no")
    expect(onFiles).not.toHaveBeenCalled()
  })

  it("calls onFiles with the dropped File[] and clears the dragging state", () => {
    const onFiles = vi.fn()
    render(<Harness onFiles={onFiles} />)
    const zone = screen.getByTestId("zone")

    const f1 = new File(["a"], "a.png", { type: "image/png" })
    const f2 = new File(["b"], "b.pdf", { type: "application/pdf" })

    fireEvent.dragEnter(zone, fileDragInit([f1, f2]))
    expect(screen.getByTestId("dragging-flag").textContent).toBe("yes")

    fireEvent.drop(zone, fileDragInit([f1, f2]))
    expect(onFiles).toHaveBeenCalledTimes(1)
    expect(onFiles.mock.calls[0][0]).toHaveLength(2)
    expect(onFiles.mock.calls[0][0][0].name).toBe("a.png")
    expect(onFiles.mock.calls[0][0][1].name).toBe("b.pdf")
    expect(screen.getByTestId("dragging-flag").textContent).toBe("no")
  })

  it("uses a counter so dragging stays true while the cursor crosses children", () => {
    render(<Harness onFiles={() => {}} />)
    const zone = screen.getByTestId("zone")
    const child = screen.getByTestId("child")
    const init = () => fileDragInit([new File(["x"], "a.png", { type: "image/png" })])

    fireEvent.dragEnter(zone, init())
    fireEvent.dragEnter(child, init())
    fireEvent.dragLeave(zone, init())
    // One enter still outstanding — overlay should still be visible.
    expect(screen.getByTestId("dragging-flag").textContent).toBe("yes")
    fireEvent.dragLeave(child, init())
    expect(screen.getByTestId("dragging-flag").textContent).toBe("no")
  })

  it("no-ops when disabled — drops are not consumed and onFiles is not called", () => {
    const onFiles = vi.fn()
    render(<Harness onFiles={onFiles} disabled />)
    const zone = screen.getByTestId("zone")

    fireEvent.dragEnter(zone, fileDragInit([new File(["x"], "a.png", { type: "image/png" })]))
    fireEvent.drop(zone, fileDragInit([new File(["x"], "a.png", { type: "image/png" })]))

    expect(screen.getByTestId("dragging-flag").textContent).toBe("no")
    expect(onFiles).not.toHaveBeenCalled()
  })

  it("still preventDefaults file drops while disabled (avoid browser navigate-to-file)", () => {
    const onFiles = vi.fn()
    render(<Harness onFiles={onFiles} disabled />)
    const zone = screen.getByTestId("zone")
    const file = new File(["x"], "a.png", { type: "image/png" })

    // createEvent + fireEvent so we can read defaultPrevented after
    // dispatch — otherwise fireEvent.drop returns whether the
    // listener cancelled, which is also informative but brittle.
    const enter = createEvent.dragEnter(zone, fileDragInit([file]))
    fireEvent(zone, enter)
    expect(enter.defaultPrevented).toBe(true)

    const over = createEvent.dragOver(zone, fileDragInit([file]))
    fireEvent(zone, over)
    expect(over.defaultPrevented).toBe(true)

    const drop = createEvent.drop(zone, fileDragInit([file]))
    fireEvent(zone, drop)
    expect(drop.defaultPrevented).toBe(true)

    // Still no state flip + no callback.
    expect(screen.getByTestId("dragging-flag").textContent).toBe("no")
    expect(onFiles).not.toHaveBeenCalled()
  })
})
