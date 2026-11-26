import { describe, expect, it } from "vitest"

import type { PendingFile } from "@/features/commodities/draft"
import { mergePendingFiles } from "@/features/commodities/draft"
import {
  applyFileSuggestions,
  fileSuggestionsFor,
  hasSuggestedTitle,
  suggestedTagOf,
  withTags,
  withTitle,
} from "@/features/commodities/fileSuggestions"
import type { ScanFileGuess } from "@/features/commodities/scanApi"

function file(name: string, bytes = 3, lastModified = 1): File {
  return new File([new Uint8Array(bytes)], name, { type: "image/jpeg", lastModified })
}

function entry(f: File, over: Partial<PendingFile> = {}): PendingFile {
  return { id: `id-${f.name}`, file: f, tags: [], ...over }
}

const sure = (value: string) => ({ value, confidence: 0.9 })

describe("fileSuggestionsFor", () => {
  it("pairs each guess with the file at its position in the request", () => {
    const empty = file("empty.jpg", 0)
    const receipt = file("receipt.jpg")
    const drill = file("drill.jpg")
    // The server skipped the empty part but still counts it, so index 1 is
    // the receipt and index 2 the drill.
    const guesses: ScanFileGuess[] = [
      { index: 1, documentType: { value: "receipt", confidence: 0.9 }, title: sure("Receipt") },
      { index: 2, documentType: { value: "photo", confidence: 0.8 }, title: sure("Drill") },
    ]
    const out = fileSuggestionsFor(guesses, [empty, receipt, drill])
    expect(out).toEqual([
      { file: receipt, tag: "invoice", title: "Receipt" },
      { file: drill, tag: "photo", title: "Drill" },
    ])
  })

  it("leaves out a guess below the confidence floor, a type with no tag and an index past the request", () => {
    const a = file("a.jpg")
    const b = file("b.pdf")
    const out = fileSuggestionsFor(
      [
        { index: 0, documentType: { value: "invoice", confidence: 0.2 }, title: sure("A") },
        { index: 1, documentType: { value: "other", confidence: 0.9 } },
        { index: 5, documentType: { value: "manual", confidence: 0.9 } },
      ],
      [a, b]
    )
    expect(out).toEqual([{ file: a, title: "A" }])
  })
})

describe("applyFileSuggestions", () => {
  it("adds the tag and replaces the default title", () => {
    const f = file("IMG_1.jpg")
    const [next] = applyFileSuggestions(
      [entry(f)],
      [{ file: f, tag: "invoice", title: "Bosch invoice" }]
    )
    expect(next.tags).toEqual(["invoice"])
    expect(next.title).toBe("Bosch invoice")
    expect(suggestedTagOf(next)).toBe("invoice")
    expect(hasSuggestedTitle(next)).toBe(true)
  })

  it("treats a title equal to the file name as the default", () => {
    const f = file("IMG_1.jpg")
    const [next] = applyFileSuggestions(
      [entry(f, { title: "IMG_1" })],
      [{ file: f, title: "Bosch drill" }]
    )
    expect(next.title).toBe("Bosch drill")
  })

  it("keeps a title and a document tag the user set", () => {
    const f = file("IMG_1.jpg")
    const before = entry(f, { title: "My receipt", tags: ["manual"] })
    const out = applyFileSuggestions([before], [{ file: f, tag: "invoice", title: "Invoice" }])
    expect(out[0]).toBe(before)
  })

  it("adds the tag beside a tag that does not classify the file", () => {
    const f = file("IMG_1.jpg")
    const [next] = applyFileSuggestions(
      [entry(f, { tags: ["kitchen"], title: "Mine" })],
      [{ file: f, tag: "photo", title: "Drill" }]
    )
    expect(next.tags).toEqual(["kitchen", "photo"])
    expect(next.title).toBe("Mine")
  })

  it("does not add back a suggested tag the user removed", () => {
    const f = file("IMG_1.jpg")
    const [first] = applyFileSuggestions([entry(f)], [{ file: f, tag: "invoice" }])
    const removed = { ...first, tags: [] }
    const out = applyFileSuggestions([removed], [{ file: f, tag: "invoice" }])
    expect(out[0]).toBe(removed)
  })

  it("puts back none of the suggested tags the user removed, over several scans", () => {
    const f = file("IMG_1.jpg")
    const [photo] = applyFileSuggestions([entry(f)], [{ file: f, tag: "photo" }])
    const noPhoto = withTags(photo, [])
    const [invoice] = applyFileSuggestions([noPhoto], [{ file: f, tag: "invoice" }])
    expect(invoice.tags).toEqual(["invoice"])
    const neither = withTags(invoice, [])
    expect(neither.suggestion?.rejectedTags).toEqual(["photo", "invoice"])
    const out = applyFileSuggestions([neither], [{ file: f, tag: "photo" }])
    expect(out[0]).toBe(neither)
    expect(out[0].tags).toEqual([])
  })

  it("does not mark a removed suggested tag the user adds back as suggested", () => {
    const f = file("IMG_1.jpg")
    const [first] = applyFileSuggestions([entry(f)], [{ file: f, tag: "invoice" }])
    expect(suggestedTagOf(first)).toBe("invoice")
    const back = withTags(withTags(first, []), ["invoice"])
    expect(back.tags).toEqual(["invoice"])
    expect(suggestedTagOf(back)).toBeUndefined()
  })

  it("sets no title once the user changed the suggested one, even back to the file name", () => {
    const f = file("IMG_1.jpg")
    const [first] = applyFileSuggestions([entry(f)], [{ file: f, title: "Bosch invoice" }])
    const reverted = withTitle(first, "IMG_1")
    expect(hasSuggestedTitle(reverted)).toBe(false)
    expect(reverted.suggestion?.rejectedTitle).toBe("Bosch invoice")
    const out = applyFileSuggestions([reverted], [{ file: f, title: "Bosch receipt" }])
    expect(out[0]).toBe(reverted)
    expect(out[0].title).toBe("IMG_1")
  })

  it("keeps the suggested title marked while the user leaves it as it is", () => {
    const f = file("IMG_1.jpg")
    const [first] = applyFileSuggestions([entry(f)], [{ file: f, title: "Bosch invoice" }])
    const same = withTitle(first, "Bosch invoice")
    expect(hasSuggestedTitle(same)).toBe(true)
    expect(same.suggestion?.rejectedTitle).toBeUndefined()
  })

  it("changes nothing on a second scan of a file the first one classified and named", () => {
    const f = file("IMG_1.jpg")
    const once = applyFileSuggestions(
      [entry(f)],
      [{ file: f, tag: "invoice", title: "Bosch invoice" }]
    )
    const twice = applyFileSuggestions(once, [{ file: f, tag: "manual", title: "Bosch manual" }])
    expect(twice).toBe(once)
    expect(twice[0].tags).toEqual(["invoice"])
    expect(twice[0].title).toBe("Bosch invoice")
  })

  it("finds a file the list already held by the file, not by its position", () => {
    const staged = file("manual.pdf")
    const photo = file("photo.jpg")
    // The user staged the manual on the Files step first; the scan sent the
    // photo and then the manual, so the list keeps the manual at 0.
    const list = mergePendingFiles([entry(staged)], [photo, staged])
    expect(list.map((e) => e.file.name)).toEqual(["manual.pdf", "photo.jpg"])
    const suggestions = fileSuggestionsFor(
      [
        { index: 0, documentType: { value: "photo", confidence: 0.9 } },
        { index: 1, documentType: { value: "manual", confidence: 0.9 } },
      ],
      [photo, staged]
    )
    const out = applyFileSuggestions(list, suggestions)
    expect(out.map((e) => [e.file.name, e.tags])).toEqual([
      ["manual.pdf", ["manual"]],
      ["photo.jpg", ["photo"]],
    ])
  })
})
