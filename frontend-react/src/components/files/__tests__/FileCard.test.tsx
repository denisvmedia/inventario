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
})
