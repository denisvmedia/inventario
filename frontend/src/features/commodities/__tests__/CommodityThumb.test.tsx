import { describe, expect, it } from "vitest"
import { fireEvent, render, screen } from "@testing-library/react"
import { axe } from "jest-axe"

import { CommodityThumb } from "@/features/commodities/CommodityThumb"
import type { CommodityCover } from "@/features/commodities/api"

const cover: CommodityCover = {
  fileId: "f1",
  thumbnails: {
    small: "https://example.test/thumb/small.jpg",
    medium: "https://example.test/thumb/medium.jpg",
  },
  source: "first_photo",
}

describe("CommodityThumb", () => {
  it("renders the cover image when a cover is provided", () => {
    render(<CommodityThumb cover={cover} type="electronics" name="Macbook" size={36} testId="t" />)
    const img = screen.getByRole("img") as HTMLImageElement
    expect(img.alt).toBe("Macbook")
    // size <= 150px → small variant.
    expect(img.src).toBe(cover.thumbnails.small)
    expect(screen.getByTestId("t").getAttribute("data-state")).toBe("image")
  })

  it("falls back to the type icon when no cover is provided", () => {
    render(<CommodityThumb type="electronics" name="Macbook" size={36} testId="t" />)
    expect(screen.queryByRole("img")).toBeNull()
    const slot = screen.getByTestId("t")
    expect(slot.getAttribute("data-state")).toBe("fallback")
    expect(slot.getAttribute("data-commodity-type")).toBe("electronics")
    // Lucide renders an inline SVG inside the slot.
    expect(slot.querySelector("svg")).not.toBeNull()
  })

  it("renders the generic fallback icon when type is unknown", () => {
    render(<CommodityThumb size={36} testId="t" />)
    const slot = screen.getByTestId("t")
    expect(slot.getAttribute("data-commodity-type")).toBe("unknown")
    expect(slot.querySelector("svg")).not.toBeNull()
  })

  it("falls back to the type icon when the image fails to load", () => {
    render(<CommodityThumb cover={cover} type="furniture" name="Sofa" size={36} testId="t" />)
    const img = screen.getByRole("img")
    fireEvent.error(img)
    // After error: image gone, icon shown.
    expect(screen.queryByRole("img")).toBeNull()
    const slot = screen.getByTestId("t")
    expect(slot.getAttribute("data-state")).toBe("fallback")
    expect(slot.getAttribute("data-commodity-type")).toBe("furniture")
    expect(slot.querySelector("svg")).not.toBeNull()
  })

  it("picks the medium variant when the slot is larger than 150px", () => {
    render(<CommodityThumb cover={cover} type="furniture" name="Sofa" size={300} testId="t" />)
    const img = screen.getByRole("img") as HTMLImageElement
    expect(img.src).toBe(cover.thumbnails.medium)
  })

  it("falls back to the available variant when the requested one is missing", () => {
    const sparseCover: CommodityCover = {
      fileId: "f1",
      thumbnails: { small: "https://example.test/only-small.jpg" },
      source: "first_photo",
    }
    render(<CommodityThumb cover={sparseCover} type="other" name="Box" size={300} testId="t" />)
    const img = screen.getByRole("img") as HTMLImageElement
    expect(img.src).toBe(sparseCover.thumbnails.small)
  })

  it("offers every thumbnail size to the browser, by width", () => {
    const fullCover: CommodityCover = {
      ...cover,
      thumbnails: { ...cover.thumbnails, large: "https://example.test/thumb/large.jpg" },
    }
    render(<CommodityThumb cover={fullCover} type="other" name="Box" size={96} testId="t" />)
    const img = screen.getByRole("img") as HTMLImageElement
    expect(img.getAttribute("srcset")).toBe(
      "https://example.test/thumb/small.jpg 150w, https://example.test/thumb/medium.jpg 300w, https://example.test/thumb/large.jpg 600w"
    )
    expect(img.getAttribute("sizes")).toBe("96px")
    expect(img.getAttribute("loading")).toBe("lazy")
  })

  it("fills its parent at the width the caller gives (#2797)", () => {
    const fullCover: CommodityCover = {
      ...cover,
      thumbnails: { ...cover.thumbnails, large: "https://example.test/thumb/large.jpg" },
    }
    render(
      <CommodityThumb
        cover={fullCover}
        type="other"
        name="Box"
        fill
        sizes="(min-width: 1280px) 240px, 50vw"
        testId="t"
      />
    )
    const slot = screen.getByTestId("t")
    expect(slot.style.width).toBe("")
    expect(slot.className).toContain("size-full")
    const img = screen.getByRole("img") as HTMLImageElement
    expect(img.getAttribute("sizes")).toBe("(min-width: 1280px) 240px, 50vw")
    expect(img.getAttribute("width")).toBeNull()
    // Without srcset support the browser falls back to the medium size.
    expect(img.src).toBe(fullCover.thumbnails.medium)
  })

  it("shows the type icon in a photo-less fill slot", () => {
    render(<CommodityThumb type="furniture" name="Sofa" fill testId="t" />)
    const slot = screen.getByTestId("t")
    expect(slot.getAttribute("data-state")).toBe("fallback")
    expect(screen.getByTestId("t-icon")).toHaveClass("size-10")
  })

  it("loads only the forced size when a variant is set", () => {
    render(
      <CommodityThumb cover={cover} type="other" name="Box" size={36} variant="medium" testId="t" />
    )
    const img = screen.getByRole("img") as HTMLImageElement
    expect(img.src).toBe(cover.thumbnails.medium)
    expect(img.getAttribute("srcset")).toBeNull()
  })

  it("uses a generic alt when name is omitted (a11y safety net)", () => {
    render(<CommodityThumb cover={cover} type="other" size={36} testId="t" />)
    const img = screen.getByRole("img") as HTMLImageElement
    expect(img.alt).toBe("Commodity photo")
  })

  it("is axe-clean in both image and fallback states", async () => {
    const withCover = render(
      <CommodityThumb cover={cover} type="electronics" name="Macbook" size={36} />
    )
    expect(await axe(withCover.container)).toHaveNoViolations()
    withCover.unmount()

    const withoutCover = render(<CommodityThumb type="electronics" name="Macbook" size={36} />)
    expect(await axe(withoutCover.container)).toHaveNoViolations()
  })
})
