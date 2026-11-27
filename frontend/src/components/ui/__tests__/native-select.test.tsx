import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { NativeSelect } from "@/components/ui/native-select"

describe("<NativeSelect />", () => {
  it("keeps the native element, with the shared focus ring, invalid style and a chevron", async () => {
    render(
      <NativeSelect aria-label="Sort" defaultValue="a" aria-invalid>
        <option value="a">A</option>
        <option value="b">B</option>
      </NativeSelect>
    )
    const select = screen.getByRole("combobox", { name: "Sort" })
    expect(select.tagName).toBe("SELECT")
    expect(select).toHaveClass("h-9", "focus-visible:ring-[3px]", "aria-invalid:border-destructive")
    const chevron = select.parentElement?.querySelector('[data-slot="native-select-icon"]')
    expect(chevron).toHaveAttribute("aria-hidden", "true")

    await userEvent.setup().selectOptions(select, "b")
    expect(select).toHaveValue("b")
  })

  it("styles the select with className and the wrapper with wrapperClassName", () => {
    render(
      <NativeSelect aria-label="Move" size="sm" className="font-medium" wrapperClassName="w-full">
        <option value="a">A</option>
      </NativeSelect>
    )
    const select = screen.getByRole("combobox", { name: "Move" })
    expect(select).toHaveAttribute("data-size", "sm")
    expect(select).toHaveClass("font-medium")
    expect(select.parentElement).toHaveAttribute("data-slot", "native-select-wrapper")
    expect(select.parentElement).toHaveClass("w-full")
  })
})
