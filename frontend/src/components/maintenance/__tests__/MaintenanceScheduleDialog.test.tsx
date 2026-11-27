import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeAll, describe, expect, it, vi } from "vitest"

import { MaintenanceScheduleDialog } from "@/components/maintenance/MaintenanceScheduleDialog"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<MaintenanceScheduleDialog />", () => {
  it("marks an empty title invalid and describes it by its message", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <MaintenanceScheduleDialog
        open
        onOpenChange={vi.fn()}
        commodityId="c1"
        initial={null}
        onSubmit={onSubmit}
        submitting={false}
      />
    )

    await user.click(screen.getByTestId("maintenance-submit"))

    expect(await screen.findByTestId("maintenance-title-error")).toBeInTheDocument()
    const title = screen.getByTestId("maintenance-title-input")
    expect(title).toHaveAttribute("aria-invalid", "true")
    expect(title).toHaveAccessibleDescription("Title is required.")
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
