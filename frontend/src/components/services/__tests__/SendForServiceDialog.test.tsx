import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeAll, describe, expect, it, vi } from "vitest"

import { SendForServiceDialog } from "@/components/services/SendForServiceDialog"
import { initI18n } from "@/i18n"
import { renderWithProviders } from "@/test/render"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<SendForServiceDialog />", () => {
  it("marks an empty provider name invalid and describes it by its message", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderWithProviders({
      children: <SendForServiceDialog open onSubmit={onSubmit} onOpenChange={vi.fn()} />,
    })

    await user.click(screen.getByTestId("service-submit"))

    expect(await screen.findByTestId("service-provider-name-error")).toBeInTheDocument()
    const name = screen.getByTestId("service-provider-name")
    expect(name).toHaveAttribute("aria-invalid", "true")
    expect(name).toHaveAccessibleDescription("Provider name is required.")
    // A valid field stays unmarked.
    expect(screen.getByTestId("service-reason")).toHaveAttribute("aria-invalid", "false")
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
