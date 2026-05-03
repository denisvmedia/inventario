import { axe } from "jest-axe"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeAll, describe, expect, it, vi } from "vitest"

import { TagFormDialog } from "@/components/tags/TagFormDialog"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<TagFormDialog />", () => {
  it("auto-derives the slug from the label in create mode", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<TagFormDialog open mode="create" onSubmit={onSubmit} onOpenChange={vi.fn()} />)
    const labelInput = screen.getByTestId("tag-form-label")
    const slugInput = screen.getByTestId("tag-form-slug") as HTMLInputElement
    await user.type(labelInput, "Kitchen Stuff")
    expect(slugInput.value).toBe("kitchen-stuff")
  })

  it("submits with the typed values when valid", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<TagFormDialog open mode="create" onSubmit={onSubmit} onOpenChange={vi.fn()} />)
    await user.type(screen.getByTestId("tag-form-label"), "Kitchen")
    // Color picker — pick green
    await user.click(screen.getByTestId("tag-form-color-green"))
    await user.click(screen.getByTestId("tag-form-submit"))
    expect(onSubmit).toHaveBeenCalledWith({
      label: "Kitchen",
      slug: "kitchen",
      color: "green",
    })
  })

  it("shows the labelRequired error when label is empty", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<TagFormDialog open mode="create" onSubmit={onSubmit} onOpenChange={vi.fn()} />)
    // Submit empty form — should fail validation, not call onSubmit
    await user.click(screen.getByTestId("tag-form-submit"))
    expect(await screen.findByTestId("tag-form-label-error")).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("prefills label/slug/color from initialValues in edit mode", () => {
    render(
      <TagFormDialog
        open
        mode="edit"
        initialValues={{ id: "t1", slug: "garden", label: "Garden", color: "blue" }}
        onSubmit={vi.fn()}
        onOpenChange={vi.fn()}
      />
    )
    expect((screen.getByTestId("tag-form-label") as HTMLInputElement).value).toBe("Garden")
    expect((screen.getByTestId("tag-form-slug") as HTMLInputElement).value).toBe("garden")
    // Selected swatch carries aria-checked=true.
    expect(screen.getByTestId("tag-form-color-blue").getAttribute("aria-checked")).toBe("true")
  })

  it("is axe-clean while open", async () => {
    const { baseElement } = render(
      <TagFormDialog open mode="create" onSubmit={vi.fn()} onOpenChange={vi.fn()} />
    )
    // Use baseElement (instead of container) so the dialog portal,
    // which Radix renders outside the test root, is included in the
    // axe scan.
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
