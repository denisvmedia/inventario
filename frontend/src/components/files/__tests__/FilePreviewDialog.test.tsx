import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"

import { FilePreviewDialog } from "@/components/files/FilePreviewDialog"
import type { ListedFile } from "@/features/files/api"

// `original_path` is the storage blob KEY, not a filename (#2241): today
// `t/<tenant>/files/<uuid>.pdf`, and before that a path-shaped
// `t/<tenant>/files/receipt-1783824560.pdf`. It was being handed to the browser
// as the download name, so saving a file wrote a UUID under a path-shaped name
// instead of the name the user gave it.
//
// The name the user sees in the UI is the name they must get on disk.
function makeFile(overrides: Partial<ListedFile["file"]> = {}): ListedFile {
  return {
    file: {
      id: "f1",
      title: "Kitchen receipt",
      path: "kitchen-receipt",
      ext: ".pdf",
      original_path: "t/tenant-1/files/f47ac10b-58cc-4372-a567-0e02b2c3d479.pdf",
      mime_type: "application/octet-stream",
      tags: [],
      ...overrides,
    },
    signedUrl: { url: "https://example.test/signed" },
  } as unknown as ListedFile
}

describe("FilePreviewDialog download name", () => {
  it("saves under the human filename, never the blob key", () => {
    render(<FilePreviewDialog file={makeFile()} onClose={() => {}} />)

    const link = screen.getByRole("link")
    expect(link).toHaveAttribute("download", "kitchen-receipt.pdf")
    expect(link.getAttribute("download")).not.toContain("t/tenant-1")
    expect(link.getAttribute("download")).not.toContain("f47ac10b")
  })

  it("falls back to the title when the row carries no path", () => {
    render(<FilePreviewDialog file={makeFile({ path: "" })} onClose={() => {}} />)

    expect(screen.getByRole("link")).toHaveAttribute("download", "Kitchen receipt.pdf")
  })

  // `path` is nominally the name WITHOUT its extension, but the API accepts one
  // that already carries it (see go/jsonapi/files_update_validation_test.go,
  // which round-trips "receipt.pdf"), so appending unconditionally would save
  // the file as `receipt.pdf.pdf`.
  it("does not double the extension when path already carries it", () => {
    render(<FilePreviewDialog file={makeFile({ path: "receipt.pdf" })} onClose={() => {}} />)

    expect(screen.getByRole("link")).toHaveAttribute("download", "receipt.pdf")
  })

  it("matches the extension case-insensitively", () => {
    render(<FilePreviewDialog file={makeFile({ path: "Receipt.PDF" })} onClose={() => {}} />)

    expect(screen.getByRole("link")).toHaveAttribute("download", "Receipt.PDF")
  })
})
