import { axe } from "jest-axe"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeAll, describe, expect, it, vi } from "vitest"

import { EditLoanDialog } from "@/components/loans/EditLoanDialog"
import type { LoanEntity } from "@/features/loans/api"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

const baseLoan = (overrides: Partial<LoanEntity> = {}): LoanEntity & { id: string } =>
  ({
    id: "loan-1",
    borrower_name: "Alice",
    borrower_contact: "alice@example.com",
    borrower_note: "",
    lent_at: "2026-05-01",
    due_back_at: "2026-12-31",
    returned_at: undefined,
    ...overrides,
  }) as LoanEntity & { id: string }

describe("<EditLoanDialog />", () => {
  it("submits an empty patch when no fields changed", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<EditLoanDialog open loan={baseLoan()} onOpenChange={vi.fn()} onSubmit={onSubmit} />)

    await user.click(screen.getByTestId("edit-loan-submit"))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit).toHaveBeenCalledWith({})
  })

  it("clearing the due date emits null in the patch", async () => {
    // The Clear affordance is the entire reason this dialog exists
    // (issue #1513). null on the wire is what flips the BE's
    // ClearDueBackAt flag — anything else (undefined, empty string)
    // would be silently dropped or rejected.
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<EditLoanDialog open loan={baseLoan()} onOpenChange={vi.fn()} onSubmit={onSubmit} />)

    await user.click(screen.getByTestId("edit-loan-clear-due-back"))
    await user.click(screen.getByTestId("edit-loan-submit"))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit).toHaveBeenCalledWith({ due_back_at: null })
  })

  it("setting a new due date emits the date in the patch", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(
      <EditLoanDialog
        open
        loan={baseLoan({ due_back_at: undefined })}
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
      />
    )

    const input = screen.getByTestId("edit-loan-due-back-at") as HTMLInputElement
    await user.type(input, "2027-01-15")
    await user.click(screen.getByTestId("edit-loan-submit"))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit).toHaveBeenCalledWith({ due_back_at: "2027-01-15" })
  })

  it("editing borrower fields produces a sparse patch", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<EditLoanDialog open loan={baseLoan()} onOpenChange={vi.fn()} onSubmit={onSubmit} />)

    const note = screen.getByTestId("edit-loan-borrower-note")
    await user.type(note, "she lives next door")
    await user.click(screen.getByTestId("edit-loan-submit"))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit).toHaveBeenCalledWith({ borrower_note: "she lives next door" })
  })

  it("clear button is hidden when the loan has no due date", () => {
    render(
      <EditLoanDialog
        open
        loan={baseLoan({ due_back_at: undefined })}
        onOpenChange={vi.fn()}
        onSubmit={vi.fn()}
      />
    )
    expect(screen.queryByTestId("edit-loan-clear-due-back")).not.toBeInTheDocument()
  })

  it("blocks submit when borrower_name is emptied", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<EditLoanDialog open loan={baseLoan()} onOpenChange={vi.fn()} onSubmit={onSubmit} />)

    const name = screen.getByTestId("edit-loan-borrower-name")
    await user.clear(name)
    await user.click(screen.getByTestId("edit-loan-submit"))

    expect(await screen.findByTestId("edit-loan-borrower-name-error")).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("is axe-clean while open", async () => {
    const { baseElement } = render(
      <EditLoanDialog open loan={baseLoan()} onOpenChange={vi.fn()} onSubmit={vi.fn()} />
    )
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })

  // Issue #1511: closed loans get a read-only render for due_back_at /
  // returned_at (date-of-record after the loan ends, immutable per the
  // BE allowlist). Borrower fields stay editable so the user can fix
  // typos and add retrospective notes.
  describe("closed loan (#1511)", () => {
    const closedLoan = baseLoan({
      returned_at: "2026-05-10",
      borrower_note: "",
    })

    it("renders due_back_at and returned_at as read-only and hides the Clear button", () => {
      render(<EditLoanDialog open loan={closedLoan} onOpenChange={vi.fn()} onSubmit={vi.fn()} />)

      expect(screen.getByTestId("edit-loan-due-back-at-readonly")).toBeInTheDocument()
      expect(screen.getByTestId("edit-loan-returned-at-readonly")).toBeInTheDocument()
      expect(screen.queryByTestId("edit-loan-due-back-at")).not.toBeInTheDocument()
      expect(screen.queryByTestId("edit-loan-clear-due-back")).not.toBeInTheDocument()
      // Inline hint mirrors the "Lent / due / returned dates can't be
      // changed" tooltip on the disabled fields so keyboard / screen-
      // reader users see it without a hover.
      expect(screen.getByTestId("edit-loan-closed-date-hint")).toBeInTheDocument()
    })

    it("emits a sparse patch with only borrower fields for closed loans", async () => {
      const user = userEvent.setup()
      const onSubmit = vi.fn().mockResolvedValue(undefined)
      render(<EditLoanDialog open loan={closedLoan} onOpenChange={vi.fn()} onSubmit={onSubmit} />)

      await user.type(screen.getByTestId("edit-loan-borrower-note"), "returned with screen scratch")
      await user.click(screen.getByTestId("edit-loan-submit"))

      expect(onSubmit).toHaveBeenCalledTimes(1)
      // Critically: NO due_back_at key — the BE rejects date edits on
      // closed loans (ErrClosedLoanFieldImmutable → 422).
      expect(onSubmit).toHaveBeenCalledWith({ borrower_note: "returned with screen scratch" })
    })

    it("allows borrower name typo fixes on closed loans", async () => {
      const user = userEvent.setup()
      const onSubmit = vi.fn().mockResolvedValue(undefined)
      render(<EditLoanDialog open loan={closedLoan} onOpenChange={vi.fn()} onSubmit={onSubmit} />)

      const name = screen.getByTestId("edit-loan-borrower-name")
      await user.clear(name)
      await user.type(name, "Alicia")
      await user.click(screen.getByTestId("edit-loan-submit"))

      expect(onSubmit).toHaveBeenCalledTimes(1)
      expect(onSubmit).toHaveBeenCalledWith({ borrower_name: "Alicia" })
    })

    it("uses the closed-loan title and description copy", () => {
      render(<EditLoanDialog open loan={closedLoan} onOpenChange={vi.fn()} onSubmit={vi.fn()} />)
      expect(screen.getByText("Edit closed loan")).toBeInTheDocument()
      expect(
        screen.getByText(
          "Edit borrower details. Lent, due, and returned dates are fixed on a closed loan."
        )
      ).toBeInTheDocument()
    })

    it("is axe-clean in closed-loan mode", async () => {
      const { baseElement } = render(
        <EditLoanDialog open loan={closedLoan} onOpenChange={vi.fn()} onSubmit={vi.fn()} />
      )
      const results = await axe(baseElement)
      expect(results).toHaveNoViolations()
    })
  })
})
