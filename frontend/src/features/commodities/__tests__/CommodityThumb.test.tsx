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

  it("falls back to the type emoji when no cover is provided", () => {
    render(<CommodityThumb type="electronics" name="Macbook" size={36} testId="t" />)
    expect(screen.queryByRole("img")).toBeNull()
    expect(screen.getByTestId("t").getAttribute("data-state")).toBe("fallback")
    expect(screen.getByTestId("t").textContent).toContain("💻")
  })

  it("renders the generic 📦 emoji when type is unknown", () => {
    render(<CommodityThumb size={36} testId="t" />)
    expect(screen.getByTestId("t").textContent).toContain("📦")
  })

  it("falls back to the emoji when the image fails to load", () => {
    render(<CommodityThumb cover={cover} type="furniture" name="Sofa" size={36} testId="t" />)
    const img = screen.getByRole("img")
    fireEvent.error(img)
    // After error: image gone, emoji shown.
    expect(screen.queryByRole("img")).toBeNull()
    expect(screen.getByTestId("t").textContent).toContain("🪑")
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
