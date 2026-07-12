import { describe, expect, it } from "vitest"

import { appendExt } from "@/lib/filename"

// `files.path` is nominally the name WITHOUT its extension, but the API accepts
// one that carries it, so concatenating blindly rendered — and downloaded —
// `receipt.pdf.pdf` (#2250).
describe("appendExt", () => {
  it("appends a missing extension", () => {
    expect(appendExt("receipt", ".pdf")).toBe("receipt.pdf")
  })

  it("does not double an extension already present", () => {
    expect(appendExt("receipt.pdf", ".pdf")).toBe("receipt.pdf")
  })

  it("matches case-insensitively", () => {
    expect(appendExt("Receipt.PDF", ".pdf")).toBe("Receipt.PDF")
  })

  it("appends a genuinely different extension", () => {
    expect(appendExt("archive.tar", ".gz")).toBe("archive.tar.gz")
  })

  it("is a no-op without an extension or a name", () => {
    expect(appendExt("receipt", undefined)).toBe("receipt")
    expect(appendExt("", ".pdf")).toBe("")
  })
})
