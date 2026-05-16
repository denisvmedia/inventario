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

  it("renders four tiles with the supplied counts (#1622)", () => {
    // Post-#1622 the `invoices` tile is gone — counts contain only
    // the three real categories plus the synthetic `all` total.
    render(
      <CategoryTiles
        active="all"
        counts={{ all: 6, images: 3, documents: 1, other: 2 }}
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByTestId("files-tile-count-all")).toHaveTextContent("6")
    expect(screen.getByTestId("files-tile-count-images")).toHaveTextContent("3")
    expect(screen.getByTestId("files-tile-count-documents")).toHaveTextContent("1")
    expect(screen.getByTestId("files-tile-count-other")).toHaveTextContent("2")
    expect(screen.queryByTestId("files-tile-invoices")).toBeNull()
  })

  it("renders an em-dash placeholder while counts are loading", () => {
    render(<CategoryTiles active="all" loading onSelect={vi.fn()} />)
    expect(screen.getByTestId("files-tile-count-images")).toHaveTextContent("—")
  })

  it("invokes onSelect with the tile key on click", async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(<CategoryTiles active="all" counts={{ all: 0 }} onSelect={onSelect} />)
    await user.click(screen.getByTestId("files-tile-images"))
    expect(onSelect).toHaveBeenCalledWith("images")
  })

  it("invokes onSelect when a tile receives Enter via keyboard", async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(<CategoryTiles active="all" counts={{ all: 0 }} onSelect={onSelect} />)
    const tile = screen.getByTestId("files-tile-documents")
    tile.focus()
    await user.keyboard("{Enter}")
    expect(onSelect).toHaveBeenCalledWith("documents")
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
    render(<CategoryTiles active="documents" counts={{ all: 0 }} onSelect={vi.fn()} />)
    expect(screen.getByTestId("files-tile-documents")).toHaveAttribute("aria-selected", "true")
    expect(screen.getByTestId("files-tile-all")).toHaveAttribute("aria-selected", "false")
  })

  it("is axe-clean across the rendered tab list", async () => {
    const { container } = render(
      <CategoryTiles active="images" counts={{ all: 4, images: 2 }} onSelect={vi.fn()} />
    )
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })

  it("collapses to a <select> on mobile breakpoints", async () => {
    const original = window.matchMedia
    // Force the useIsMobile hook to read mobile=true via a synchronous
    // matchMedia stub. Restored in afterEach so it doesn't leak.
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      media: query,
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
      onchange: null,
    })) as unknown as typeof window.matchMedia
    try {
      const onSelect = vi.fn()
      render(
        <CategoryTiles
          active="images"
          counts={{ all: 4, images: 2, documents: 1, other: 0 }}
          onSelect={onSelect}
        />
      )
      const sel = screen.getByTestId("files-category-select") as HTMLSelectElement
      expect(sel.value).toBe("images")
      // Each option carries the count in its label — the mobile fallback
      // doubles as a quick reference.
      expect(sel.textContent).toMatch(/Images \(2\)/)
      expect(sel.textContent).toMatch(/All \(4\)/)
      const user = userEvent.setup()
      await user.selectOptions(sel, "documents")
      expect(onSelect).toHaveBeenCalledWith("documents")
    } finally {
      window.matchMedia = original
    }
  })
})
