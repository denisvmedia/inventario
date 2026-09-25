import { describe, expect, it } from "vitest"
import { renderHook } from "@testing-library/react"

import { useAppToast } from "@/hooks/useAppToast"

import { toast } from "sonner"

describe("useAppToast", () => {
  it("delegates each variant to the sonner equivalent", () => {
    const { result } = renderHook(() => useAppToast())
    result.current.success("ok")
    result.current.error("nope")
    result.current.info("note")
    result.current.warning("careful")
    result.current.dismiss(1)
    expect(toast.success).toHaveBeenCalledWith("ok", undefined)
    expect(toast.error).toHaveBeenCalledWith("nope", undefined)
    expect(toast.info).toHaveBeenCalledWith("note", undefined)
    expect(toast.warning).toHaveBeenCalledWith("careful", undefined)
    expect(toast.dismiss).toHaveBeenCalledWith(1)
  })
})
