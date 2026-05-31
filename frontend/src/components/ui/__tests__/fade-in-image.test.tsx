import { describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen } from "@testing-library/react"

import { FadeInImage } from "@/components/ui/fade-in-image"

const SRC = "https://example.test/photo.jpg"

describe("<FadeInImage />", () => {
  it("starts hidden behind a shimmer placeholder, then fades in on load", () => {
    render(<FadeInImage src={SRC} alt="Photo" data-testid="img" />)
    const img = screen.getByTestId("img")
    // Placeholder visible while loading; image is transparent + async-decoded.
    expect(screen.getByTestId("img").className).toContain("opacity-0")
    expect(document.querySelector('[data-slot="fade-in-image-placeholder"]')).not.toBeNull()
    expect(img).toHaveAttribute("decoding", "async")

    fireEvent.load(img)
    // After load: image opaque, placeholder gone.
    expect(img.className).toContain("opacity-100")
    expect(document.querySelector('[data-slot="fade-in-image-placeholder"]')).toBeNull()
  })

  it("reveals the (broken) image and drops the placeholder on error", () => {
    const onError = vi.fn()
    render(<FadeInImage src={SRC} alt="Photo" data-testid="img" onError={onError} />)
    const img = screen.getByTestId("img")
    fireEvent.error(img)
    // A failed image must not stay stuck invisible — the surface decides
    // its own fallback (e.g. CommodityThumb swaps to an icon).
    expect(img.className).toContain("opacity-100")
    expect(document.querySelector('[data-slot="fade-in-image-placeholder"]')).toBeNull()
    expect(onError).toHaveBeenCalledTimes(1)
  })

  it("re-runs the fade when src changes (recycled <img>)", () => {
    const { rerender } = render(<FadeInImage src={SRC} alt="Photo" data-testid="img" />)
    const img = screen.getByTestId("img")
    fireEvent.load(img)
    expect(img.className).toContain("opacity-100")

    rerender(<FadeInImage src="https://example.test/other.jpg" alt="Photo" data-testid="img" />)
    // New src resets to loading: transparent again + shimmer back.
    expect(screen.getByTestId("img").className).toContain("opacity-0")
    expect(document.querySelector('[data-slot="fade-in-image-placeholder"]')).not.toBeNull()
  })

  it("forwards image attributes and merges the caller className", () => {
    render(
      <FadeInImage
        src={SRC}
        alt="Photo"
        loading="lazy"
        width={120}
        height={120}
        className="size-full object-cover"
        data-testid="img"
      />
    )
    const img = screen.getByTestId("img") as HTMLImageElement
    expect(img).toHaveAttribute("loading", "lazy")
    expect(img).toHaveAttribute("width", "120")
    expect(img.className).toContain("object-cover")
    expect(img.className).toContain("transition-opacity")
  })

  it("renders a caller-supplied placeholder only while loading", () => {
    render(
      <FadeInImage
        src={SRC}
        alt="Photo"
        data-testid="img"
        placeholder={<span data-testid="custom-ph" />}
      />
    )
    expect(screen.getByTestId("custom-ph")).toBeInTheDocument()
    // The default muted shimmer is not rendered when overridden.
    expect(document.querySelector('[data-slot="fade-in-image-placeholder"]')).toBeNull()

    fireEvent.load(screen.getByTestId("img"))
    expect(screen.queryByTestId("custom-ph")).not.toBeInTheDocument()
  })

  it("treats an already-complete (cached) image as loaded without waiting for onLoad", () => {
    // A cache hit can fire `load` before React wires onLoad; the hook
    // reconciles `<img>.complete` after mount so the image never stays
    // stranded at opacity-0. jsdom never decodes, so fake the getters.
    const proto = HTMLImageElement.prototype
    const completeDesc = Object.getOwnPropertyDescriptor(proto, "complete")
    const widthDesc = Object.getOwnPropertyDescriptor(proto, "naturalWidth")
    Object.defineProperty(proto, "complete", { configurable: true, get: () => true })
    Object.defineProperty(proto, "naturalWidth", { configurable: true, get: () => 42 })
    try {
      render(<FadeInImage src={SRC} alt="Photo" data-testid="img" />)
      expect(screen.getByTestId("img").className).toContain("opacity-100")
      expect(document.querySelector('[data-slot="fade-in-image-placeholder"]')).toBeNull()
    } finally {
      if (completeDesc) Object.defineProperty(proto, "complete", completeDesc)
      if (widthDesc) Object.defineProperty(proto, "naturalWidth", widthDesc)
    }
  })

  it("renders no placeholder when placeholder is null", () => {
    render(<FadeInImage src={SRC} alt="Photo" data-testid="img" placeholder={null} />)
    expect(document.querySelector('[data-slot="fade-in-image-placeholder"]')).toBeNull()
    expect(screen.getByTestId("img").className).toContain("opacity-0")
  })
})
