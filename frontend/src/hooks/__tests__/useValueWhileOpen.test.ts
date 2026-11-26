import { describe, expect, it } from "vitest"
import { renderHook } from "@testing-library/react"

import { useValueWhileOpen } from "@/hooks/useValueWhileOpen"

describe("useValueWhileOpen", () => {
  it("follows the value while open and holds the last one after closing", () => {
    const first = { id: "a" }
    const second = { id: "b" }
    const { result, rerender } = renderHook(
      ({ open, value }: { open: boolean; value: { id: string } | null }) =>
        useValueWhileOpen(open, value),
      { initialProps: { open: true, value: first as { id: string } | null } }
    )
    expect(result.current).toBe(first)

    rerender({ open: true, value: second })
    expect(result.current).toBe(second)

    // The parent clears the value in the same update that closes the dialog.
    rerender({ open: false, value: null })
    expect(result.current).toBe(second)

    rerender({ open: true, value: null })
    expect(result.current).toBeNull()
  })
})
