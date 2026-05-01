import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CategoryTiles } from "@/components/files/CategoryTiles"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<CategoryTiles />", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("renders five tiles with the supplied counts", () => {
    render(
      <CategoryTiles
        active="all"
        counts={{ all: 11, photos: 3, invoices: 5, documents: 1, other: 2 }}
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByTestId("files-tile-count-all")).toHaveTextContent("11")
    expect(screen.getByTestId("files-tile-count-photos")).toHaveTextContent("3")
    expect(screen.getByTestId("files-tile-count-invoices")).toHaveTextContent("5")
    expect(screen.getByTestId("files-tile-count-documents")).toHaveTextContent("1")
    expect(screen.getByTestId("files-tile-count-other")).toHaveTextContent("2")
  })

  it("renders an em-dash placeholder while counts are loading", () => {
    render(<CategoryTiles active="all" loading onSelect={vi.fn()} />)
    expect(screen.getByTestId("files-tile-count-photos")).toHaveTextContent("—")
  })

  it("invokes onSelect with the tile key on click", async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(<CategoryTiles active="all" counts={{ all: 0 }} onSelect={onSelect} />)
    await user.click(screen.getByTestId("files-tile-photos"))
    expect(onSelect).toHaveBeenCalledWith("photos")
  })

  it("invokes onSelect when a tile receives Enter via keyboard", async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(<CategoryTiles active="all" counts={{ all: 0 }} onSelect={onSelect} />)
    const tile = screen.getByTestId("files-tile-invoices")
    tile.focus()
    await user.keyboard("{Enter}")
    expect(onSelect).toHaveBeenCalledWith("invoices")
  })

  it("invokes onSelect when a tile receives Space via keyboard", async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(<CategoryTiles active="all" counts={{ all: 0 }} onSelect={onSelect} />)
    const tile = screen.getByTestId("files-tile-documents")
    tile.focus()
    await user.keyboard(" ")
    expect(onSelect).toHaveBeenCalledWith("documents")
  })

  it("flips aria-selected to true on the active tile only", () => {
    render(<CategoryTiles active="invoices" counts={{ all: 0 }} onSelect={vi.fn()} />)
    expect(screen.getByTestId("files-tile-invoices")).toHaveAttribute("aria-selected", "true")
    expect(screen.getByTestId("files-tile-all")).toHaveAttribute("aria-selected", "false")
  })

  it("is axe-clean across the rendered tab list", async () => {
    const { container } = render(
      <CategoryTiles active="photos" counts={{ all: 4, photos: 2 }} onSelect={vi.fn()} />
    )
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
