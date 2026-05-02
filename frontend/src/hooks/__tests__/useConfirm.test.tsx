import { describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useState } from "react"

import { ConfirmProvider, useConfirm } from "@/hooks/useConfirm"

function Harness() {
  const confirm = useConfirm()
  const [result, setResult] = useState<string>("none")
  return (
    <div>
      <button
        type="button"
        onClick={async () => {
          const ok = await confirm({
            title: "Delete item?",
            description: "This cannot be undone.",
            destructive: true,
          })
          setResult(ok ? "confirmed" : "cancelled")
        }}
      >
        ask
      </button>
      <div data-testid="result">{result}</div>
    </div>
  )
}

describe("useConfirm", () => {
  it("resolves true when the user clicks Confirm", async () => {
    const user = userEvent.setup()
    render(
      <ConfirmProvider>
        <Harness />
      </ConfirmProvider>
    )
    await user.click(screen.getByRole("button", { name: /ask/i }))
    // The dialog mounts via Radix's portal — we look it up by role.
    expect(await screen.findByRole("dialog")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: /confirm/i }))
    expect(await screen.findByText("confirmed")).toBeInTheDocument()
  })

  it("resolves false when the user clicks Cancel", async () => {
    const user = userEvent.setup()
    render(
      <ConfirmProvider>
        <Harness />
      </ConfirmProvider>
    )
    await user.click(screen.getByRole("button", { name: /ask/i }))
    await screen.findByRole("dialog")
    await user.click(screen.getByRole("button", { name: /cancel/i }))
    expect(await screen.findByText("cancelled")).toBeInTheDocument()
  })

  it("throws when used outside the provider", () => {
    function Broken() {
      useConfirm()
      return null
    }
    const spy = vi.spyOn(console, "error").mockImplementation(() => {})
    expect(() => render(<Broken />)).toThrow(/must be used within a ConfirmProvider/)
    spy.mockRestore()
  })
})
