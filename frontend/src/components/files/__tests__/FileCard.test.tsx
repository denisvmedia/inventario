import { beforeAll, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { FileCard } from "@/components/files/FileCard"
import type { FileEntity } from "@/features/files/api"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

function file(overrides: Partial<FileEntity> = {}): FileEntity & { id: string } {
  return {
    id: "f1",
    title: "My file",
    category: "images",
    type: "image",
    tags: [],
    path: "my-file",
    original_path: "my-file.jpg",
    ext: ".jpg",
    mime_type: "image/jpeg",
    created_at: "2026-04-30T10:00:00Z",
    ...overrides,
  } as FileEntity & { id: string }
}

describe("<FileCard />", () => {
  it("renders the title and an image thumbnail when MIME is image/* and a signed URL is supplied", () => {
    render(
      <FileCard
        file={file()}
        signedUrl={{ url: "https://cdn.example/file.jpg" }}
        selected={false}
        onToggleSelect={vi.fn()}
        onOpen={vi.fn()}
      />
    )
    expect(screen.getByText("My file")).toBeInTheDocument()
    const img = screen.getByRole("img") as HTMLImageElement
    expect(img.src).toBe("https://cdn.example/file.jpg")
  })

  it("falls back to a category icon when there is no signed URL", () => {
    render(
      <FileCard
        file={file({ category: "documents", mime_type: "application/pdf" })}
        selected={false}
        onToggleSelect={vi.fn()}
        onOpen={vi.fn()}
      />
    )
    expect(screen.queryByRole("img")).not.toBeInTheDocument()
  })

  it("renders the title fallback when title is empty (uses path)", () => {
    render(
      <FileCard
        file={file({ title: "", path: "named-by-path" })}
        selected={false}
        onToggleSelect={vi.fn()}
        onOpen={vi.fn()}
      />
    )
    expect(screen.getByText("named-by-path")).toBeInTheDocument()
  })

  it("renders up to three tag chips with a +N overflow", () => {
    render(
      <FileCard
        file={file({ tags: ["a", "b", "c", "d", "e"] })}
        selected={false}
        onToggleSelect={vi.fn()}
        onOpen={vi.fn()}
      />
    )
    // Per-#1659 the card emits mock-style colored "#tag" chips instead
    // of `<Badge>`s — match the prefix so the assertion follows the
    // visual change.
    expect(screen.getByText("#a")).toBeInTheDocument()
    expect(screen.getByText("#b")).toBeInTheDocument()
    expect(screen.getByText("#c")).toBeInTheDocument()
    expect(screen.queryByText("#d")).not.toBeInTheDocument()
    expect(screen.getByText("+2")).toBeInTheDocument()
  })

  // #1659 item 1: the per-MIME / per-category fallback icon palette
  // mirrors design-mocks/src/views/FileBrowserView.tsx mimeIconAndColor
  // — image/*  → status-active (green), application/pdf → status-expired
  // (red), zip/archive → chart-4, anything else falls back to the
  // category token (documents → chart-3, other → muted-foreground).
  // Post-#1622 the `invoice` palette is tag-driven, not category-driven
  // (the `invoices` FileCategory was dropped); the tag-based test below
  // covers that path.
  it.each([
    ["image/png", "images", "image"],
    ["application/pdf", "documents", "pdf"],
    ["application/zip", "other", "archive"],
    ["application/x-zip-compressed", "other", "archive"],
    ["text/plain", "documents", "document"],
    ["application/octet-stream", "other", "other"],
  ])("tags the fallback palette as %s+%s → %s", (mime, category, group) => {
    render(
      <FileCard
        file={file({
          mime_type: mime,
          category: category as "images" | "documents" | "other",
        })}
        onOpen={vi.fn()}
      />
    )
    // The image branch renders an <img>, not the fallback tile, when
    // a signed URL is supplied — but here we never supply one, so the
    // fallback tile is the rendered branch regardless of MIME.
    const card = screen.getByTestId("file-card-f1")
    expect(card.getAttribute("data-mime-group")).toBe(group)
  })

  // Post-#1622: invoice palette is driven by the `invoice` tag (not a
  // dedicated FileCategory), and tag detection short-circuits before
  // the PDF / archive MIME branches so an invoice-tagged PDF still
  // reads as an invoice in the row.
  it("tags an invoice-tagged file with the invoice palette regardless of MIME (#1622)", () => {
    render(
      <FileCard
        file={file({
          mime_type: "application/pdf",
          category: "documents",
          tags: ["invoice"],
        })}
        onOpen={vi.fn()}
      />
    )
    const card = screen.getByTestId("file-card-f1")
    expect(card.getAttribute("data-mime-group")).toBe("invoice")
  })

  it("calls onOpen with the file id when the body is clicked", async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    render(<FileCard file={file()} selected={false} onToggleSelect={vi.fn()} onOpen={onOpen} />)
    await user.click(screen.getByTestId("file-card-open-f1"))
    expect(onOpen).toHaveBeenCalledWith("f1")
  })

  it("calls onToggleSelect when the checkbox is toggled", async () => {
    const user = userEvent.setup()
    const onToggleSelect = vi.fn()
    render(
      <FileCard file={file()} selected={false} onToggleSelect={onToggleSelect} onOpen={vi.fn()} />
    )
    await user.click(screen.getByTestId("file-card-checkbox-f1"))
    expect(onToggleSelect).toHaveBeenCalledWith("f1")
  })

  it("does not render the cover-toggle star when no onSetCover handler is supplied", () => {
    render(<FileCard file={file()} onOpen={vi.fn()} />)
    expect(screen.queryByTestId("file-card-cover-f1")).not.toBeInTheDocument()
  })

  it("does not render the cover-toggle star for non-photo files even if onSetCover is supplied", () => {
    render(
      <FileCard
        file={file({ category: "documents", mime_type: "application/pdf" })}
        onSetCover={vi.fn()}
        onOpen={vi.fn()}
      />
    )
    expect(screen.queryByTestId("file-card-cover-f1")).not.toBeInTheDocument()
  })

  it("renders the cover-toggle star with explicit state when this file is the current cover", () => {
    render(
      <FileCard
        file={file()}
        coverState={{ current: "f1" }}
        onSetCover={vi.fn()}
        onOpen={vi.fn()}
      />
    )
    const star = screen.getByTestId("file-card-cover-f1")
    expect(star.getAttribute("data-cover-state")).toBe("explicit")
    expect(star.getAttribute("aria-pressed")).toBe("true")
  })

  it("renders the cover-toggle star with auto state when this file is the first-photo auto-pick", () => {
    render(
      <FileCard file={file()} coverState={{ auto: "f1" }} onSetCover={vi.fn()} onOpen={vi.fn()} />
    )
    expect(screen.getByTestId("file-card-cover-f1").getAttribute("data-cover-state")).toBe("auto")
  })

  it("calls onSetCover with the file id when the outline star is clicked", async () => {
    const user = userEvent.setup()
    const onSetCover = vi.fn()
    render(
      <FileCard
        file={file()}
        coverState={{ auto: "f1" }}
        onSetCover={onSetCover}
        onOpen={vi.fn()}
      />
    )
    await user.click(screen.getByTestId("file-card-cover-f1"))
    expect(onSetCover).toHaveBeenCalledWith("f1")
  })

  it("calls onSetCover with null when the explicit star is clicked (clears the override)", async () => {
    const user = userEvent.setup()
    const onSetCover = vi.fn()
    render(
      <FileCard
        file={file()}
        coverState={{ current: "f1" }}
        onSetCover={onSetCover}
        onOpen={vi.fn()}
      />
    )
    await user.click(screen.getByTestId("file-card-cover-f1"))
    expect(onSetCover).toHaveBeenCalledWith(null)
  })

  it("disables the cover-toggle star while a mutation is in flight", () => {
    render(
      <FileCard
        file={file()}
        coverState={{ auto: "f1" }}
        onSetCover={vi.fn()}
        coverBusy
        onOpen={vi.fn()}
      />
    )
    expect(screen.getByTestId("file-card-cover-f1")).toBeDisabled()
  })
})
