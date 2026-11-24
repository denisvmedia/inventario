import { useState } from "react"
import { describe, expect, it, vi } from "vitest"
import { Link, Route, useLocation } from "react-router-dom"
import { fireEvent, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { useUnsavedChangesGuard } from "@/hooks/useUnsavedChangesGuard"
import { renderWithProviders } from "@/test/render"

function Where() {
  const location = useLocation()
  return <p data-testid="where">{location.pathname}</p>
}

function Editor({ onSave }: { onSave: () => Promise<boolean> }) {
  const [dirty, setDirty] = useState(false)
  const [left, setLeft] = useState(0)
  const { guard, dialog } = useUnsavedChangesGuard({ dirty, onSave })
  return (
    <div>
      <button type="button" onClick={() => setDirty(true)}>
        edit
      </button>
      <button type="button" onClick={() => guard(() => setLeft((n) => n + 1))}>
        leave
      </button>
      <Link to="/elsewhere">elsewhere</Link>
      <a href="/file.pdf" download>
        download
      </a>
      <p data-testid="left">{left}</p>
      {dialog}
    </div>
  )
}

function renderEditor(onSave: () => Promise<boolean> = async () => true) {
  return renderWithProviders({
    initialPath: "/editor",
    routes: (
      <>
        <Route
          path="/editor"
          element={
            <>
              <Editor onSave={onSave} />
              <Where />
            </>
          }
        />
        <Route path="/elsewhere" element={<Where />} />
      </>
    ),
  })
}

describe("useUnsavedChangesGuard", () => {
  it("leaves at once when nothing is unsaved", async () => {
    const user = userEvent.setup()
    renderEditor()
    await user.click(screen.getByRole("button", { name: "leave" }))
    expect(screen.getByTestId("left")).toHaveTextContent("1")
    expect(screen.queryByTestId("unsaved-changes-dialog")).not.toBeInTheDocument()
  })

  it("keeps editing when asked to", async () => {
    const user = userEvent.setup()
    renderEditor()
    await user.click(screen.getByRole("button", { name: "edit" }))
    await user.click(screen.getByRole("button", { name: "leave" }))
    expect(await screen.findByTestId("unsaved-changes-dialog")).toBeInTheDocument()

    await user.click(screen.getByTestId("unsaved-changes-keep-editing"))
    expect(screen.getByTestId("left")).toHaveTextContent("0")
    expect(screen.queryByTestId("unsaved-changes-dialog")).not.toBeInTheDocument()
  })

  it("leaves without saving on Discard", async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async () => true)
    renderEditor(onSave)
    await user.click(screen.getByRole("button", { name: "edit" }))
    await user.click(screen.getByRole("button", { name: "leave" }))
    await user.click(await screen.findByTestId("unsaved-changes-discard"))
    expect(screen.getByTestId("left")).toHaveTextContent("1")
    expect(onSave).not.toHaveBeenCalled()
  })

  it("saves, then leaves", async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async () => true)
    renderEditor(onSave)
    await user.click(screen.getByRole("button", { name: "edit" }))
    await user.click(screen.getByRole("button", { name: "leave" }))
    await user.click(await screen.findByTestId("unsaved-changes-save"))
    expect(onSave).toHaveBeenCalledTimes(1)
    expect(screen.getByTestId("left")).toHaveTextContent("1")
  })

  it("stays when the save fails", async () => {
    const user = userEvent.setup()
    renderEditor(async () => false)
    await user.click(screen.getByRole("button", { name: "edit" }))
    await user.click(screen.getByRole("button", { name: "leave" }))
    await user.click(await screen.findByTestId("unsaved-changes-save"))
    expect(screen.getByTestId("left")).toHaveTextContent("0")
    expect(screen.queryByTestId("unsaved-changes-dialog")).not.toBeInTheDocument()
  })

  it("holds an in-app link click until the user decides", async () => {
    const user = userEvent.setup()
    renderEditor()
    await user.click(screen.getByRole("button", { name: "edit" }))
    await user.click(screen.getByRole("link", { name: "elsewhere" }))
    expect(await screen.findByTestId("unsaved-changes-dialog")).toBeInTheDocument()
    expect(screen.getByTestId("where")).toHaveTextContent("/editor")

    await user.click(screen.getByTestId("unsaved-changes-discard"))
    expect(await screen.findByTestId("where")).toHaveTextContent("/elsewhere")
  })

  it("lets a link through when nothing is unsaved", async () => {
    const user = userEvent.setup()
    renderEditor()
    await user.click(screen.getByRole("link", { name: "elsewhere" }))
    expect(await screen.findByTestId("where")).toHaveTextContent("/elsewhere")
  })

  it("does not hold a download link", async () => {
    const user = userEvent.setup()
    renderEditor()
    await user.click(screen.getByRole("button", { name: "edit" }))
    const link = screen.getByRole("link", { name: "download" })
    // jsdom does not download; preventing the default keeps it from
    // attempting a navigation it does not implement.
    link.addEventListener("click", (e) => e.preventDefault())
    fireEvent.click(link)
    expect(screen.queryByTestId("unsaved-changes-dialog")).not.toBeInTheDocument()
  })

  it("asks the browser to confirm closing the tab while edits are unsaved", async () => {
    const user = userEvent.setup()
    renderEditor()
    const clean = new Event("beforeunload", { cancelable: true })
    window.dispatchEvent(clean)
    expect(clean.defaultPrevented).toBe(false)

    await user.click(screen.getByRole("button", { name: "edit" }))
    const dirty = new Event("beforeunload", { cancelable: true })
    window.dispatchEvent(dirty)
    expect(dirty.defaultPrevented).toBe(true)
  })
})
