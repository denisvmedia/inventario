import { useState } from "react"
import { describe, expect, it, vi } from "vitest"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  type DialogContentVariant,
} from "@/components/ui/dialog"

// jsdom applies no Tailwind CSS, so these tests pin the classes that carry
// each layout and the behavior Radix gives both variants. The rendered
// layout at a phone and a desktop width is covered by
// e2e/tests/mobile-dialogs.spec.ts.

function FormDialog({
  variant,
  onOpenChange = () => {},
  footerClassName,
  showCloseButton,
}: {
  variant?: DialogContentVariant
  onOpenChange?: (open: boolean) => void
  footerClassName?: string
  showCloseButton?: boolean
}) {
  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent
        variant={variant}
        showCloseButton={showCloseButton}
        className="sm:max-w-lg"
        data-testid="content"
      >
        <DialogHeader data-testid="header">
          <DialogTitle>New location</DialogTitle>
          <DialogDescription>Where your things live.</DialogDescription>
        </DialogHeader>
        <form>
          <label htmlFor="name">Name</label>
          <input id="name" />
          <DialogFooter data-testid="footer" className={footerClassName}>
            <Button type="button">Cancel</Button>
            <Button type="submit">Create</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function classesOf(testId: string) {
  return screen.getByTestId(testId).className.split(/\s+/)
}

describe("<DialogContent variant>", () => {
  it("renders the centered card by default", () => {
    render(<FormDialog />)
    const content = screen.getByTestId("content")
    expect(content).toHaveAttribute("data-variant", "default")
    expect(classesOf("content")).toEqual(
      expect.arrayContaining(["top-[50%]", "left-[50%]", "rounded-lg", "border", "sm:max-w-lg"])
    )
    expect(content.className).not.toContain("max-sm:")
    expect(classesOf("header")).toContain("text-center")
    expect(screen.getByTestId("header").className).not.toContain("sticky")
    expect(classesOf("footer")).toContain("flex-col-reverse")
    expect(screen.getByTestId("footer").className).not.toContain("sticky")
    // One close button, after the content.
    expect(screen.getAllByRole("button", { name: "Close" })).toHaveLength(1)
  })

  it("fills the screen below sm and keeps the card from sm up", () => {
    render(<FormDialog variant="fullscreen-mobile" />)
    expect(screen.getByTestId("content")).toHaveAttribute("data-variant", "fullscreen-mobile")
    expect(classesOf("content")).toEqual(
      expect.arrayContaining([
        // Edge to edge, sized to the dynamic viewport, scrolling inside.
        "max-sm:top-0",
        "max-sm:left-0",
        "max-sm:translate-none",
        "max-sm:h-dvh",
        "max-sm:w-full",
        "max-sm:max-w-none",
        "max-sm:max-h-none",
        "max-sm:rounded-none",
        "max-sm:border-0",
        "max-sm:shadow-none",
        "max-sm:overflow-y-auto",
        // A focused field stays clear of the sticky header and footer.
        "max-sm:scroll-pt-36",
        "max-sm:scroll-pb-28",
        // Slides up and down, and only fades under reduced motion.
        "max-sm:motion-safe:data-[state=open]:slide-in-from-bottom",
        "max-sm:motion-safe:data-[state=closed]:slide-out-to-bottom",
        "max-sm:motion-safe:data-[state=open]:fade-in-100",
        // From sm up: the same centered card as the default variant.
        "top-[50%]",
        "left-[50%]",
        "translate-x-[-50%]",
        "translate-y-[-50%]",
        "rounded-lg",
        "border",
        "sm:max-w-lg",
        "data-[state=open]:zoom-in-95",
        // A card on a phone held sideways is never taller than the screen.
        "sm:max-h-[calc(100dvh-2rem)]",
        "sm:overflow-y-auto",
      ])
    )
    expect(classesOf("header")).toEqual(
      expect.arrayContaining([
        "max-sm:sticky",
        "max-sm:top-0",
        "max-sm:border-b",
        "max-sm:bg-background",
        "max-sm:pt-[calc(0.75rem+env(safe-area-inset-top))]",
        "max-sm:pr-14",
        "text-left",
      ])
    )
    expect(classesOf("header")).not.toContain("text-center")
    expect(classesOf("footer")).toEqual(
      expect.arrayContaining([
        "max-sm:sticky",
        "max-sm:bottom-0",
        "max-sm:mt-auto",
        "max-sm:border-t",
        "max-sm:pb-[calc(0.75rem+env(safe-area-inset-bottom))]",
        "flex-row",
        "sm:justify-end",
      ])
    )
    expect(classesOf("footer")).not.toContain("flex-col-reverse")
  })

  it("puts the close button in the header below sm and keeps the corner one from sm up", () => {
    render(<FormDialog variant="fullscreen-mobile" />)
    const [first, last] = screen.getAllByRole("button", { name: "Close" })
    expect(within(screen.getByTestId("header")).getByRole("button", { name: "Close" })).toBe(first)
    expect(first.className.split(/\s+/)).toEqual(expect.arrayContaining(["sm:hidden", "size-11"]))
    expect(last.className).toContain("max-sm:hidden")
    expect(screen.getByTestId("header")).not.toContainElement(last)
  })

  it("renders no close button at all with showCloseButton={false}", () => {
    render(<FormDialog variant="fullscreen-mobile" showCloseButton={false} />)
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument()
    // The header keeps the full-screen layout.
    expect(classesOf("header")).toEqual(expect.arrayContaining(["max-sm:sticky", "max-sm:top-0"]))
  })

  it("lays out a dialog nested inside a full-screen one as a card", () => {
    render(
      <Dialog open>
        <DialogContent variant="fullscreen-mobile" data-testid="outer">
          <DialogHeader data-testid="outer-header">
            <DialogTitle>Add item</DialogTitle>
            <DialogDescription>Step 1 of 5</DialogDescription>
          </DialogHeader>
          <Dialog open>
            <DialogContent data-testid="inner">
              <DialogHeader data-testid="inner-header">
                <DialogTitle>Pick a location</DialogTitle>
                <DialogDescription>Where it lives.</DialogDescription>
              </DialogHeader>
              <DialogFooter data-testid="inner-footer" showCloseButton />
            </DialogContent>
          </Dialog>
        </DialogContent>
      </Dialog>
    )
    expect(classesOf("outer-header")).toContain("max-sm:sticky")
    const inner = screen.getByTestId("inner")
    expect(inner).toHaveAttribute("data-variant", "default")
    expect(inner.className).not.toContain("max-sm:")
    expect(screen.getByTestId("inner-header").className).not.toContain("max-sm:")
    expect(screen.getByTestId("inner-footer").className).not.toContain("max-sm:")
    // The inner header has no close button of its own; the inner card has its
    // corner one and the footer's.
    expect(within(screen.getByTestId("inner-header")).queryByRole("button")).toBeNull()
    expect(within(inner).getAllByRole("button", { name: "Close" })).toHaveLength(2)
  })

  it("lets the call site override the footer alignment", () => {
    render(
      <FormDialog
        variant="fullscreen-mobile"
        footerClassName="justify-between sm:justify-between"
      />
    )
    expect(classesOf("footer")).toContain("justify-between")
    expect(classesOf("footer")).not.toContain("justify-end")
  })

  it("keeps the title and description bound and focuses the header close button first", async () => {
    render(<FormDialog variant="fullscreen-mobile" />)
    const dialog = screen.getByRole("dialog")
    const title = screen.getByText("New location")
    const description = screen.getByText("Where your things live.")
    expect(dialog).toHaveAttribute("aria-labelledby", title.id)
    expect(dialog).toHaveAttribute("aria-describedby", description.id)
    // jsdom hides neither close button, so this pins the DOM order that a
    // phone sees: the header close button is the first control.
    const headerClose = within(screen.getByTestId("header")).getByRole("button", { name: "Close" })
    await waitFor(() => expect(headerClose).toHaveFocus())
  })

  it("traps the focus and closes on Escape and on the header close button", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    render(<FormDialog variant="fullscreen-mobile" onOpenChange={onOpenChange} />)
    const dialog = screen.getByRole("dialog")
    await waitFor(() => expect(dialog).toContainElement(document.activeElement as HTMLElement))
    for (let i = 0; i < 6; i++) {
      await user.tab()
      expect(dialog).toContainElement(document.activeElement as HTMLElement)
    }
    await user.keyboard("{Escape}")
    expect(onOpenChange).toHaveBeenLastCalledWith(false)

    onOpenChange.mockClear()
    await user.click(within(screen.getByTestId("header")).getByRole("button", { name: "Close" }))
    expect(onOpenChange).toHaveBeenLastCalledWith(false)
  })

  it("shows a confirmation card above the full-screen dialog", async () => {
    const user = userEvent.setup()
    function WithConfirm() {
      const [confirmOpen, setConfirmOpen] = useState(false)
      return (
        <Dialog open>
          <DialogContent variant="fullscreen-mobile" data-testid="form">
            <DialogHeader>
              <DialogTitle>Add item</DialogTitle>
              <DialogDescription>Step 1 of 5</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" onClick={() => setConfirmOpen(true)}>
                Cancel
              </Button>
            </DialogFooter>
          </DialogContent>
          <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
            <DialogContent className="sm:max-w-sm" data-testid="confirm">
              <DialogHeader data-testid="confirm-header">
                <DialogTitle>Save your progress?</DialogTitle>
                <DialogDescription>Keep a draft.</DialogDescription>
              </DialogHeader>
              <DialogFooter data-testid="confirm-footer">
                <Button type="button" onClick={() => setConfirmOpen(false)}>
                  Keep editing
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </Dialog>
      )
    }
    render(<WithConfirm />)
    await user.click(screen.getByRole("button", { name: "Cancel" }))

    const form = screen.getByTestId("form")
    const confirm = await screen.findByTestId("confirm")
    // The confirmation is a centered card at every width.
    expect(confirm).toHaveAttribute("data-variant", "default")
    expect(confirm.className).not.toContain("max-sm:")
    expect(screen.getByTestId("confirm-header").className).not.toContain("sticky")
    expect(screen.getByTestId("confirm-footer").className).not.toContain("sticky")
    // Both sit on z-50; the confirmation's portal comes later in the
    // document, so it paints above the full-screen dialog.
    expect(confirm.className).toContain("z-50")
    expect(form.className).toContain("z-50")
    expect(form.compareDocumentPosition(confirm) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    // Assistive tech sees only the confirmation.
    expect(screen.getByRole("dialog", { name: "Save your progress?" })).toBe(confirm)
    expect(screen.queryByRole("dialog", { name: "Add item" })).not.toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "Keep editing" }))
    await waitFor(() => expect(screen.queryByTestId("confirm")).not.toBeInTheDocument())
    expect(screen.getByRole("dialog", { name: "Add item" })).toBe(form)
  })

  it("has no axe violations in the full-screen variant", async () => {
    const { baseElement } = render(<FormDialog variant="fullscreen-mobile" />)
    expect(await axe(baseElement)).toHaveNoViolations()
  })
})
