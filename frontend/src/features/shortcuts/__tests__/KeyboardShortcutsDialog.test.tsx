import { describe, expect, it } from "vitest"
import { fireEvent, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { KeyboardShortcutsProvider, useKeyboardShortcutsDialog } from "@/features/shortcuts"
import { renderWithProviders } from "@/test/render"

function OpenButton() {
  const { setOpen } = useKeyboardShortcutsDialog()
  return (
    <button type="button" onClick={() => setOpen(true)}>
      open
    </button>
  )
}

function harness() {
  return renderWithProviders({
    children: (
      <KeyboardShortcutsProvider>
        <OpenButton />
      </KeyboardShortcutsProvider>
    ),
  })
}

describe("<KeyboardShortcutsDialog />", () => {
  it("opens via the global `?` keybinding and closes on Esc", async () => {
    harness()
    expect(screen.queryByTestId("keyboard-shortcuts-dialog")).not.toBeInTheDocument()

    fireEvent.keyDown(window, { key: "?" })
    const dialog = await screen.findByTestId("keyboard-shortcuts-dialog")
    expect(dialog).toBeVisible()

    // Esc closes the dialog (Radix handles this internally).
    await userEvent.keyboard("{Escape}")
    await waitFor(() =>
      expect(screen.queryByTestId("keyboard-shortcuts-dialog")).not.toBeInTheDocument()
    )
  })

  it("opens imperatively via setOpen() — the Settings → Help entry point", async () => {
    harness()
    await userEvent.click(screen.getByRole("button", { name: /open/i }))
    expect(await screen.findByTestId("keyboard-shortcuts-dialog")).toBeVisible()
  })

  it("skips the `?` binding while typing in an input", () => {
    const { container } = renderWithProviders({
      children: (
        <KeyboardShortcutsProvider>
          <input data-testid="probe" />
        </KeyboardShortcutsProvider>
      ),
    })
    const input = within(container).getByTestId("probe")
    input.focus()
    fireEvent.keyDown(input, { key: "?", target: input })
    expect(screen.queryByTestId("keyboard-shortcuts-dialog")).not.toBeInTheDocument()
  })

  it("ignores `?` when combined with a modifier", () => {
    harness()
    fireEvent.keyDown(window, { key: "?", metaKey: true })
    expect(screen.queryByTestId("keyboard-shortcuts-dialog")).not.toBeInTheDocument()
    fireEvent.keyDown(window, { key: "?", ctrlKey: true })
    expect(screen.queryByTestId("keyboard-shortcuts-dialog")).not.toBeInTheDocument()
  })

  it("renders the Navigation/Search/Help section the cheat sheet ships with", async () => {
    harness()
    fireEvent.keyDown(window, { key: "?" })
    const dialog = await screen.findByTestId("keyboard-shortcuts-dialog")
    // Categories that contain at least one MVP shortcut appear.
    expect(within(dialog).getByTestId("keyboard-shortcuts-section-search")).toBeInTheDocument()
    expect(within(dialog).getByTestId("keyboard-shortcuts-section-layout")).toBeInTheDocument()
    expect(within(dialog).getByTestId("keyboard-shortcuts-section-help")).toBeInTheDocument()
    // Every MVP entry has a row.
    expect(within(dialog).getByTestId("keyboard-shortcut-command-palette.open")).toBeInTheDocument()
    expect(within(dialog).getByTestId("keyboard-shortcut-sidebar.toggle")).toBeInTheDocument()
    expect(within(dialog).getByTestId("keyboard-shortcut-shortcuts.show")).toBeInTheDocument()
  })

  it("filters the list by label substring", async () => {
    harness()
    fireEvent.keyDown(window, { key: "?" })
    const dialog = await screen.findByTestId("keyboard-shortcuts-dialog")
    const filter = within(dialog).getByTestId("keyboard-shortcuts-filter")
    await userEvent.type(filter, "sidebar")
    // Only the sidebar row should remain.
    expect(within(dialog).getByTestId("keyboard-shortcut-sidebar.toggle")).toBeInTheDocument()
    expect(
      within(dialog).queryByTestId("keyboard-shortcut-command-palette.open")
    ).not.toBeInTheDocument()
  })

  it("renders an empty state when no shortcut matches the filter", async () => {
    harness()
    fireEvent.keyDown(window, { key: "?" })
    const dialog = await screen.findByTestId("keyboard-shortcuts-dialog")
    const filter = within(dialog).getByTestId("keyboard-shortcuts-filter")
    await userEvent.type(filter, "zzzzzznothing")
    expect(within(dialog).getByTestId("keyboard-shortcuts-empty")).toBeInTheDocument()
  })
})
