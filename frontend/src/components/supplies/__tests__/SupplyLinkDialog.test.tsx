import { axe } from "jest-axe"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeAll, describe, expect, it, vi } from "vitest"

import { SupplyLinkDialog } from "@/components/supplies/SupplyLinkDialog"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<SupplyLinkDialog />", () => {
  it("submits with the typed values when label + url are valid", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<SupplyLinkDialog open title="Add link" onSubmit={onSubmit} onOpenChange={vi.fn()} />)

    await user.type(screen.getByTestId("supply-link-label-input"), "Water filter")
    await user.type(screen.getByTestId("supply-link-url-input"), "https://example.com/water-filter")
    await user.type(screen.getByTestId("supply-link-notes-input"), "Pack of 2")
    await user.click(screen.getByTestId("supply-link-submit"))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit.mock.calls[0]?.[0]).toEqual({
      label: "Water filter",
      url: "https://example.com/water-filter",
      notes: "Pack of 2",
    })
  })

  it("blocks submit when label is empty", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<SupplyLinkDialog open title="Add link" onSubmit={onSubmit} onOpenChange={vi.fn()} />)

    await user.type(screen.getByTestId("supply-link-url-input"), "https://example.com/x")
    await user.click(screen.getByTestId("supply-link-submit"))

    expect(await screen.findByTestId("supply-link-label-error")).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("rejects a URL without scheme", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<SupplyLinkDialog open title="Add link" onSubmit={onSubmit} onOpenChange={vi.fn()} />)

    await user.type(screen.getByTestId("supply-link-label-input"), "Water filter")
    await user.type(screen.getByTestId("supply-link-url-input"), "example.com/no-scheme")
    await user.click(screen.getByTestId("supply-link-submit"))

    expect(await screen.findByTestId("supply-link-url-error")).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("seeds the form when initial is provided (edit flow)", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(
      <SupplyLinkDialog
        open
        title="Edit supply link"
        initial={{
          id: "supply-1",
          label: "Existing",
          url: "https://example.com/existing",
          notes: "old notes",
          commodity_id: "commodity-1",
          sort_order: 0,
        }}
        onSubmit={onSubmit}
        onOpenChange={vi.fn()}
      />
    )

    const labelInput = screen.getByTestId("supply-link-label-input") as HTMLInputElement
    expect(labelInput.value).toBe("Existing")

    await user.clear(labelInput)
    await user.type(labelInput, "Renamed")
    await user.click(screen.getByTestId("supply-link-submit"))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit.mock.calls[0]?.[0]?.label).toBe("Renamed")
  })

  it("is axe-clean while open", async () => {
    const { baseElement } = render(
      <SupplyLinkDialog open title="Add link" onSubmit={vi.fn()} onOpenChange={vi.fn()} />
    )
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
