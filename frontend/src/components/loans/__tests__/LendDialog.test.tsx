import { axe } from "jest-axe"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeAll, describe, expect, it, vi } from "vitest"

import { LendDialog } from "@/components/loans/LendDialog"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<LendDialog />", () => {
  it("submits with the typed values + defaults lent_at to today", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<LendDialog open onSubmit={onSubmit} onOpenChange={vi.fn()} />)

    await user.type(screen.getByTestId("lend-borrower-name"), "Alice")
    await user.click(screen.getByTestId("lend-submit"))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    const call = onSubmit.mock.calls[0]?.[0]
    expect(call.borrower_name).toBe("Alice")
    expect(call.borrower_contact).toBe("")
    expect(call.borrower_note).toBe("")
    expect(call.due_back_at).toBe("")
    // lent_at defaults to "today" in YYYY-MM-DD; assert shape, not value.
    expect(call.lent_at).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })

  it("blocks submit when borrower_name is empty", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<LendDialog open onSubmit={onSubmit} onOpenChange={vi.fn()} />)
    await user.click(screen.getByTestId("lend-submit"))
    expect(await screen.findByTestId("lend-borrower-name-error")).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("rejects an invalid lent_at format", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<LendDialog open onSubmit={onSubmit} onOpenChange={vi.fn()} />)

    await user.type(screen.getByTestId("lend-borrower-name"), "Alice")
    const lentAt = screen.getByTestId("lend-lent-at") as HTMLInputElement
    // Replace the default value with garbage. Native date inputs in
    // jsdom don't enforce the YYYY-MM-DD regex on their own — we rely
    // on the zod schema to do it.
    await user.clear(lentAt)
    await user.type(lentAt, "not-a-date")
    await user.click(screen.getByTestId("lend-submit"))

    expect(await screen.findByTestId("lend-lent-at-error")).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("is axe-clean while open", async () => {
    const { baseElement } = render(<LendDialog open onSubmit={vi.fn()} onOpenChange={vi.fn()} />)
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
