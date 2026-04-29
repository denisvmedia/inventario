import "@testing-library/jest-dom/vitest"
import { afterAll, afterEach, beforeAll, expect, vi } from "vitest"
import { cleanup } from "@testing-library/react"
import { toHaveNoViolations } from "jest-axe"

import { server } from "./server"
import { initI18n } from "@/i18n"

expect.extend(toHaveNoViolations)

// JSDOM doesn't ship with `matchMedia` (the prefers-color-scheme listener
// in our ThemeProvider needs it). Stub it as a static "no" answer with no
// listeners so any code that probes it during tests gets a stable result;
// individual tests can still vi.spyOn(window, "matchMedia") to override.
if (!window.matchMedia) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
}

// JSDOM doesn't ship with ResizeObserver either; radix-ui primitives
// (Checkbox via @radix-ui/react-use-size) call into it during layout.
// Stub it as a no-op so tests that mount those primitives don't crash.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
if (typeof window.ResizeObserver === "undefined") {
  ;(window as unknown as { ResizeObserver: typeof ResizeObserverStub }).ResizeObserver =
    ResizeObserverStub
}
if (typeof globalThis.ResizeObserver === "undefined") {
  ;(globalThis as unknown as { ResizeObserver: typeof ResizeObserverStub }).ResizeObserver =
    ResizeObserverStub
}

// Boot i18n once for the whole suite. The en bundle is already in memory
// (statically imported by i18next.config.ts), so this resolves in a single
// microtask and every useTranslation() in test render returns real strings
// instead of the bare keys.
beforeAll(async () => {
  await initI18n({ lng: "en" })
  server.listen({ onUnhandledRequest: "error" })
})
afterEach(() => {
  cleanup()
  server.resetHandlers()
})
afterAll(() => server.close())
