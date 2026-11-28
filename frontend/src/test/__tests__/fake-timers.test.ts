import { expect, it, vi } from "vitest"

// Guards the focus-fixup hook in test/setup.ts. Without it, one test on fake
// timers that moves focus stops every animation frame in the environment, and
// the shared-env project shares that environment with every later file in the
// worker.

function nextFrame(): Promise<"frame" | "no frame"> {
  return Promise.race([
    new Promise<"frame">((resolve) => window.requestAnimationFrame(() => resolve("frame"))),
    new Promise<"no frame">((resolve) => setTimeout(() => resolve("no frame"), 1000)),
  ])
}

it("runs animation frames after a test on fake timers removed the focused element", async () => {
  // jsdom stops its rendering interval once the frame queue is empty, so the
  // fake clock below is the one that would start it again.
  expect(await nextFrame()).toBe("frame")

  vi.useFakeTimers({ shouldAdvanceTime: true })
  const input = document.createElement("input")
  document.body.append(input)
  input.focus()
  input.remove()
  vi.useRealTimers()

  expect(await nextFrame()).toBe("frame")
})
