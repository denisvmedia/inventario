import "@testing-library/jest-dom/vitest"
import { afterAll, afterEach, beforeAll, expect, vi } from "vitest"
import { cleanup } from "@testing-library/react"
import { toHaveNoViolations } from "jest-axe"

import { server } from "./server"
import { initI18n } from "@/i18n"

expect.extend(toHaveNoViolations)

// Quiet sonner globally for the test suite. The real `<Toaster />` portals
// into document.body and the toast queue persists across renders — both
// would leak DOM and force per-test cleanup gymnastics. The mock keeps
// the same export shape so component imports compile, but `<Toaster />`
// renders nothing and `toast.*` functions are no-ops returning a stub id.
// Tests that want to assert toast behavior (see useAppToast.test.tsx) call
// vi.mock("sonner", ...) again locally with their own spies — vi.mock
// hoists per-file so the local mock wins.
vi.mock("sonner", () => {
  const id = "stub-toast-id"
  const noop = vi.fn(() => id)
  return {
    Toaster: () => null,
    toast: Object.assign(noop, {
      success: noop,
      error: noop,
      info: noop,
      warning: noop,
      message: noop,
      promise: noop,
      dismiss: vi.fn(),
      loading: noop,
      custom: noop,
    }),
  }
})

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

// pdfjs-dist references DOMMatrix at module load (canvas API). JSDOM
// doesn't ship with it, so any test that pulls the file detail sheet
// transitively imports the PDF viewer and crashes on the missing
// global. Stubbed on globalThis with a no-op identity so the import
// resolves; tests that actually render PDFs would mock pdfjs-dist
// directly.
if (typeof (globalThis as { DOMMatrix?: unknown }).DOMMatrix === "undefined") {
  ;(globalThis as { DOMMatrix?: unknown }).DOMMatrix = class DOMMatrixStub {
    constructor() {}
  }
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
  // ThemeProvider writes `light`/`dark` classes on <html>, DensityProvider
  // writes `data-density`. RTL's cleanup() unmounts the React tree but
  // doesn't revert these mutations, so without an explicit reset the next
  // test would observe whatever the previous test ended on. Order-of-tests
  // dependence is exactly what cross-suite stability is supposed to rule
  // out — reset both here. Test storage keys also get cleared so any
  // localStorage-driven rehydration (theme, density, sidebar cookie) starts
  // from a known baseline.
  document.documentElement.classList.remove("light", "dark")
  document.documentElement.removeAttribute("data-density")
  window.localStorage.clear()
})
afterAll(() => server.close())
