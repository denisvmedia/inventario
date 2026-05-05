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
    category: "photos",
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

  it("renders up to three tag badges with a +N overflow", () => {
    render(
      <FileCard
        file={file({ tags: ["a", "b", "c", "d", "e"] })}
        selected={false}
        onToggleSelect={vi.fn()}
        onOpen={vi.fn()}
      />
    )
    // First three tags rendered, then +2 overflow.
    expect(screen.getByText("a")).toBeInTheDocument()
    expect(screen.getByText("b")).toBeInTheDocument()
    expect(screen.getByText("c")).toBeInTheDocument()
    expect(screen.queryByText("d")).not.toBeInTheDocument()
    expect(screen.getByText("+2")).toBeInTheDocument()
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
