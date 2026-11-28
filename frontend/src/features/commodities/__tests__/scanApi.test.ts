import { describe, expect, it } from "vitest"

import { normalizeScanResponse } from "@/features/commodities/scanApi"

describe("normalizeScanResponse files", () => {
  it("reads the suggestion for each file and drops what it cannot use", () => {
    const result = normalizeScanResponse({
      data: {
        attributes: {
          fields: {},
          files: [
            {
              index: 0,
              document_type: { value: "invoice", confidence: 0.9 },
              title: { value: " Bosch invoice ", confidence: 0.8 },
            },
            // An unknown type is left out; the title stays.
            {
              index: 2,
              document_type: { value: "letter", confidence: 0.9 },
              title: { value: "Note" },
            },
            // A repeated index keeps the first entry.
            { index: 0, document_type: { value: "manual", confidence: 0.9 } },
            // No usable index, or nothing usable left.
            { index: -1, document_type: { value: "photo", confidence: 0.9 } },
            { index: 1.5, document_type: { value: "photo", confidence: 0.9 } },
            { index: 3, title: { value: "  " } },
            null,
          ],
        },
      },
    })
    expect(result.files).toEqual([
      {
        index: 0,
        documentType: { value: "invoice", confidence: 0.9 },
        title: { value: "Bosch invoice", confidence: 0.8 },
      },
      { index: 2, title: { value: "Note", confidence: 0 } },
    ])
  })

  it("has no files when the response has none", () => {
    expect(normalizeScanResponse({ data: { attributes: { fields: {} } } }).files).toEqual([])
  })
})
