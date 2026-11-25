import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useState } from "react"
import { describe, expect, it, vi } from "vitest"

import { TagFilterMenu, type TagFilterValue } from "@/components/tags/TagFilterMenu"
import type { ResolvedTag } from "@/features/tags/hooks"

const TAGS: ResolvedTag[] = [
  { slug: "garage", label: "Garage", color: "blue" },
  { slug: "fragile", label: "Fragile", color: "red" },
  { slug: "kitchen", label: "Kitchen", color: "green" },
]

const EMPTY: TagFilterValue = { include: [], exclude: [], mode: "all", untagged: false }

function Harness({
  initial = EMPTY,
  onChange,
  ...props
}: {
  initial?: TagFilterValue
  onChange?: (v: TagFilterValue) => void
  allowExclude?: boolean
  allowMode?: boolean
  allowUntagged?: boolean
  counts?: Record<string, number>
  untaggedCount?: number
  onOpenChange?: (open: boolean) => void
}) {
  const [value, setValue] = useState(initial)
  return (
    <TagFilterMenu
      label="Tags"
      testIdPrefix="tf"
      tags={TAGS}
      value={value}
      onChange={(next) => {
        setValue(next)
        onChange?.(next)
      }}
      {...props}
    />
  )
}

describe("<TagFilterMenu />", () => {
  it("cycles a tag through required, excluded and off", async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Harness onChange={onChange} allowExclude />)
    await user.click(screen.getByTestId("tf"))
    const option = await screen.findByTestId("tf-option-garage")

    await user.click(option)
    expect(onChange).toHaveBeenLastCalledWith({ ...EMPTY, include: ["garage"] })
    expect(screen.getByTestId("tf-option-garage")).toHaveAttribute("data-state", "include")

    await user.click(screen.getByTestId("tf-option-garage"))
    expect(onChange).toHaveBeenLastCalledWith({ ...EMPTY, exclude: ["garage"] })
    expect(screen.getByTestId("tf-option-garage")).toHaveAttribute("data-state", "exclude")

    await user.click(screen.getByTestId("tf-option-garage"))
    expect(onChange).toHaveBeenLastCalledWith(EMPTY)
    // No condition left, so the trigger shows no count.
    expect(within(screen.getByTestId("tf")).queryByText("1")).toBeNull()
  })

  it("toggles a tag on and off when exclusion is not offered", async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Harness onChange={onChange} />)
    await user.click(screen.getByTestId("tf"))
    await user.click(await screen.findByTestId("tf-option-kitchen"))
    await user.click(screen.getByTestId("tf-option-kitchen"))
    expect(onChange).toHaveBeenLastCalledWith(EMPTY)
    expect(screen.queryByTestId("tf-untagged")).toBeNull()
    expect(screen.queryByTestId("tf-mode-any")).toBeNull()
  })

  it("shows counts, switches all/any and picks items without tags", async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    const onOpenChange = vi.fn()
    render(
      <Harness
        initial={{ ...EMPTY, include: ["garage"] }}
        onChange={onChange}
        onOpenChange={onOpenChange}
        allowExclude
        allowMode
        allowUntagged
        counts={{ garage: 4, fragile: 2 }}
        untaggedCount={7}
      />
    )
    expect(within(screen.getByTestId("tf")).getByText("1")).toBeInTheDocument()
    await user.click(screen.getByTestId("tf"))
    expect(onOpenChange).toHaveBeenLastCalledWith(true)
    expect(await screen.findByTestId("tag-filter-count-garage")).toHaveTextContent("4")
    // A tag without a count lists none of the query's items.
    expect(screen.getByTestId("tag-filter-count-kitchen")).toHaveTextContent("0")
    expect(screen.getByTestId("tf-untagged")).toHaveTextContent("7")

    await user.click(screen.getByTestId("tf-mode-any"))
    expect(onChange).toHaveBeenLastCalledWith({ ...EMPTY, include: ["garage"], mode: "any" })

    // "No tags" replaces the tag conditions; picking a tag replaces it.
    await user.click(screen.getByTestId("tf-untagged"))
    expect(onChange).toHaveBeenLastCalledWith({ ...EMPTY, mode: "any", untagged: true })
    await user.click(screen.getByTestId("tf-option-fragile"))
    expect(onChange).toHaveBeenLastCalledWith({ ...EMPTY, mode: "any", include: ["fragile"] })

    await user.click(screen.getByTestId("tf-clear"))
    expect(onChange).toHaveBeenLastCalledWith({ ...EMPTY, mode: "any" })
  })

  it("filters the list by the search box and keeps an unknown selected slug", async () => {
    const user = userEvent.setup()
    render(<Harness initial={{ ...EMPTY, include: ["gone"] }} />)
    await user.click(screen.getByTestId("tf"))
    expect(await screen.findByTestId("tf-option-gone")).toHaveAttribute("data-state", "include")
    await user.type(screen.getByPlaceholderText("Search tags…"), "kit")
    expect(screen.getByTestId("tf-option-kitchen")).toBeInTheDocument()
    expect(screen.queryByTestId("tf-option-garage")).toBeNull()
  })
})
