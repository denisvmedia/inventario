import { describe, expect, it } from "vitest"

import {
  buildDefaults,
  fileSignature,
  mergePendingFiles,
  type PendingFile,
} from "@/features/commodities/draft"
import type { Commodity } from "@/features/commodities/api"

// Pin lastModified so two Files built from the same (name, bytes) hash to the
// same signature — otherwise Date.now() drift between constructions would make
// the de-dupe assertions flaky.
function file(name: string, opts?: { type?: string; bytes?: number }): File {
  const size = opts?.bytes ?? 3
  return new File([new Uint8Array(size)], name, {
    type: opts?.type ?? "image/jpeg",
    lastModified: 1_700_000_000_000,
  })
}

function pending(name: string): PendingFile {
  return { id: `id-${name}`, file: file(name), tags: [] }
}

// buildDefaults' `defaultDraft` seam backs the anonymous "add your first
// item" flow (#1988): the landing dialog passes true so a first-time
// visitor only has to fill name/short_name/type (price/date relax to
// optional in the draft schema).
describe("buildDefaults", () => {
  it("defaults a new item to non-draft", () => {
    expect(buildDefaults(undefined, "USD").draft).toBe(false)
  })

  it("honours defaultDraft=true for a brand-new item", () => {
    expect(buildDefaults(undefined, "USD", true).draft).toBe(true)
  })

  it("lets an existing record's own draft value win over defaultDraft", () => {
    const existing = { draft: false } as unknown as Commodity
    expect(buildDefaults(existing, "USD", true).draft).toBe(false)
  })
})

// mergePendingFiles folds the AI-scan source files into the Files-step queue
// (#1983 Part A) without double-attaching anything the user already staged.
describe("mergePendingFiles", () => {
  it("appends new files as PendingFile entries with empty tags", () => {
    const out = mergePendingFiles([], [file("a.jpg"), file("b.pdf", { type: "application/pdf" })])
    expect(out).toHaveLength(2)
    expect(out.map((e) => e.file.name)).toEqual(["a.jpg", "b.pdf"])
    expect(out.every((e) => e.tags.length === 0 && typeof e.id === "string" && e.id !== "")).toBe(
      true
    )
  })

  it("keeps existing entries and only adds the genuinely new file", () => {
    const existing = [pending("keep.jpg")]
    const out = mergePendingFiles(existing, [
      file("keep.jpg"),
      file("new.png", { type: "image/png" }),
    ])
    expect(out.map((e) => e.file.name)).toEqual(["keep.jpg", "new.png"])
    // The pre-existing entry is preserved verbatim (same id), not recreated.
    expect(out[0]).toBe(existing[0])
  })

  it("de-dupes by name+size+lastModified so the same file isn't attached twice", () => {
    const f = file("dup.jpg")
    const out = mergePendingFiles([{ id: "x", file: f, tags: ["invoice"] }], [f])
    expect(out).toHaveLength(1)
    // The original entry (with its tags) wins; the duplicate is dropped.
    expect(out[0].tags).toEqual(["invoice"])
  })

  it("returns the same array reference when there is nothing new to add", () => {
    const existing = [pending("a.jpg")]
    expect(mergePendingFiles(existing, [])).toBe(existing)
    expect(mergePendingFiles(existing, [file("a.jpg")])).toBe(existing)
  })

  it("treats different sizes as distinct files", () => {
    const out = mergePendingFiles(
      [{ id: "x", file: file("a.jpg", { bytes: 3 }), tags: [] }],
      [file("a.jpg", { bytes: 9 })]
    )
    expect(out).toHaveLength(2)
  })
})

describe("fileSignature", () => {
  it("is identical for the same File and distinct across name/size", () => {
    const f = file("a.jpg", { bytes: 5 })
    expect(fileSignature(f)).toBe(fileSignature(f))
    expect(fileSignature(file("a.jpg", { bytes: 5 }))).not.toBe(
      fileSignature(file("b.jpg", { bytes: 5 }))
    )
    expect(fileSignature(file("a.jpg", { bytes: 5 }))).not.toBe(
      fileSignature(file("a.jpg", { bytes: 6 }))
    )
  })
})
