import { vi } from "vitest"

// jsdom never fetches an image, so neither `load` nor `error` ever fires and
// Radix Avatar stays on its fallback for good. stubImageLoading replaces
// window.Image with one that settles each src on the next microtask: loaded,
// unless `fails` says the src is broken.
//
// The stub is global and the suite shares one module registry, so a test that
// calls this must undo it with vi.unstubAllGlobals() in its afterEach.
export function stubImageLoading(fails: (src: string) => boolean = () => false): void {
  class SettlingImage extends EventTarget {
    complete = false
    naturalWidth = 0
    referrerPolicy = ""
    crossOrigin: string | null = null
    onload: ((event: Event) => void) | null = null
    onerror: ((event: Event) => void) | null = null
    #src = ""

    get src(): string {
      return this.#src
    }

    set src(value: string) {
      this.#src = value
      queueMicrotask(() => {
        const ok = !fails(value)
        this.complete = true
        this.naturalWidth = ok ? 1 : 0
        const event = new Event(ok ? "load" : "error")
        this.dispatchEvent(event)
        ;(ok ? this.onload : this.onerror)?.call(this, event)
      })
    }
  }
  vi.stubGlobal("Image", SettlingImage)
}
