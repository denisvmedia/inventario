import { describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { TagsInput } from "@/components/files/TagsInput"
import { renderWithProviders } from "@/test/render"

describe("<TagsInput />", () => {
  it("commits a typed tag on Enter", async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<TagsInput values={[]} onChange={onChange} testId="t" />)
    const input = screen.getByTestId("t-input")
    await user.type(input, "alpha")
    await user.keyboard("{Enter}")
    expect(onChange).toHaveBeenCalledWith(["alpha"])
  })

  it("commits a typed tag on comma", async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<TagsInput values={[]} onChange={onChange} testId="t" />)
    const input = screen.getByTestId("t-input")
    await user.type(input, "beta,")
    expect(onChange).toHaveBeenCalledWith(["beta"])
  })

  it("ignores duplicate values silently", async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<TagsInput values={["alpha"]} onChange={onChange} testId="t" />)
    await user.type(screen.getByTestId("t-input"), "alpha{Enter}")
    expect(onChange).not.toHaveBeenCalled()
  })

  it("removes a chip via its remove button", async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<TagsInput values={["alpha", "beta"]} onChange={onChange} testId="t" />)
    await user.click(screen.getByLabelText("remove alpha"))
    expect(onChange).toHaveBeenCalledWith(["beta"])
  })

  it("pops the last tag on Backspace when the draft is empty", async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<TagsInput values={["alpha", "beta"]} onChange={onChange} testId="t" />)
    const input = screen.getByTestId("t-input")
    input.focus()
    await user.keyboard("{Backspace}")
    expect(onChange).toHaveBeenCalledWith(["alpha"])
  })

  it("renders the supplied label when present", () => {
    render(<TagsInput label="My tags" values={[]} onChange={vi.fn()} testId="t" />)
    expect(screen.getByText("My tags")).toBeInTheDocument()
  })

  it("emits a datalist with the provided suggestions, minus already-selected values", () => {
    const { container } = render(
      <TagsInput
        values={["alpha"]}
        onChange={vi.fn()}
        suggestions={["alpha", "beta", "gamma"]}
        testId="t"
      />
    )
    const datalist = container.querySelector('datalist[data-testid="t-datalist"]')
    expect(datalist).not.toBeNull()
    const options = Array.from(datalist!.querySelectorAll("option"))
    expect(options.map((o) => o.value)).toEqual(["beta", "gamma"])
  })

  it("omits the datalist entirely when no suggestions are provided", () => {
    const { container } = render(<TagsInput values={[]} onChange={vi.fn()} testId="t" />)
    expect(container.querySelector("datalist")).toBeNull()
  })
})

// #1630: the Popover autocomplete rendered a listbox but was mouse-only —
// no arrow keys, no active-option tracking, and every option statically
// `aria-selected="false"`. A keyboard or screen-reader user could not reach
// the suggestions at all, which is a regression against the plain <datalist>
// the browser used to handle.
describe("<TagsInput /> autocomplete keyboard", () => {
  const suggestions = ["alpha", "beta", "gamma"]

  function renderCombobox(values: string[] = []) {
    const onChange = vi.fn()
    renderWithProviders({
      children: (
        <TagsInput
          values={values}
          onChange={onChange}
          testId="t"
          autocomplete
          kind="file"
          suggestions={suggestions}
        />
      ),
    })
    return { onChange, input: screen.getByTestId("t-input") }
  }

  it("declares the combobox relationship the pattern requires", async () => {
    const user = userEvent.setup()
    const { input } = renderCombobox()

    expect(input).toHaveAttribute("role", "combobox")
    expect(input).toHaveAttribute("aria-autocomplete", "list")
    expect(input).toHaveAttribute("aria-expanded", "false")

    await user.click(input)
    expect(input).toHaveAttribute("aria-expanded", "true")
    expect(input.getAttribute("aria-controls")).toBe(
      screen.getByTestId("t-dropdown").getAttribute("id")
    )
  })

  it("walks the options with the arrow keys and commits the highlighted one", async () => {
    const user = userEvent.setup()
    const { onChange, input } = renderCombobox()

    await user.click(input)
    await user.keyboard("{ArrowDown}{ArrowDown}")
    // aria-activedescendant has to name the option the user is on, or a
    // screen reader announces nothing as the highlight moves.
    const options = screen.getAllByRole("option")
    expect(input.getAttribute("aria-activedescendant")).toBe(options[1].getAttribute("id"))
    expect(options[1]).toHaveAttribute("aria-selected", "true")
    expect(options[0]).toHaveAttribute("aria-selected", "false")

    await user.keyboard("{Enter}")
    expect(onChange).toHaveBeenCalledWith(["beta"])
  })

  it("wraps at both ends, and Home / End jump to them", async () => {
    const user = userEvent.setup()
    const { onChange, input } = renderCombobox()

    await user.click(input)
    // Up from nothing highlighted lands on the last option.
    await user.keyboard("{ArrowUp}")
    expect(screen.getAllByRole("option")[2]).toHaveAttribute("aria-selected", "true")
    // And down from the last wraps to the first.
    await user.keyboard("{ArrowDown}")
    expect(screen.getAllByRole("option")[0]).toHaveAttribute("aria-selected", "true")

    await user.keyboard("{End}")
    expect(screen.getAllByRole("option")[2]).toHaveAttribute("aria-selected", "true")
    await user.keyboard("{Home}{Enter}")
    expect(onChange).toHaveBeenCalledWith(["alpha"])
  })

  it("Enter with nothing highlighted still commits the typed draft", async () => {
    const user = userEvent.setup()
    const { onChange, input } = renderCombobox()

    await user.click(input)
    await user.type(input, "delta")
    await user.keyboard("{Enter}")
    expect(onChange).toHaveBeenCalledWith(["delta"])
  })

  it("typing drops the highlight, because the list under it has changed", async () => {
    const user = userEvent.setup()
    const { onChange, input } = renderCombobox()

    await user.click(input)
    await user.keyboard("{ArrowDown}")
    expect(screen.getAllByRole("option")[0]).toHaveAttribute("aria-selected", "true")

    // "a" narrows the list; the old index would now point at a different tag.
    await user.type(input, "gam")
    expect(input).not.toHaveAttribute("aria-activedescendant")
    await user.keyboard("{Enter}")
    expect(onChange).toHaveBeenCalledWith(["gam"])
  })

  it("Escape closes the list and clears the highlight", async () => {
    const user = userEvent.setup()
    const { input } = renderCombobox()

    await user.click(input)
    await user.keyboard("{ArrowDown}")
    await user.keyboard("{Escape}")
    expect(input).toHaveAttribute("aria-expanded", "false")
    expect(input).not.toHaveAttribute("aria-activedescendant")
  })
})

// Raised in review of #1630. Each of these is a way the highlight and the
// committed value can disagree.
describe("<TagsInput /> autocomplete highlight lifetime", () => {
  const suggestions = ["alpha", "beta", "gamma"]

  function renderCombobox() {
    const onChange = vi.fn()
    renderWithProviders({
      children: (
        <TagsInput
          values={[]}
          onChange={onChange}
          testId="t"
          autocomplete
          kind="file"
          suggestions={suggestions}
        />
      ),
    })
    return { onChange, input: screen.getByTestId("t-input") }
  }

  // Comma has always meant "end this tag". Folding it into the same branch as
  // Enter made it take the highlighted suggestion and throw the draft away.
  it("comma commits the typed draft even with a suggestion highlighted", async () => {
    const user = userEvent.setup()
    const { onChange, input } = renderCombobox()

    await user.click(input)
    await user.type(input, "b")
    await user.keyboard("{ArrowDown}")
    expect(screen.getAllByRole("option")[0]).toHaveAttribute("aria-selected", "true")

    await user.type(input, ",")
    expect(onChange).toHaveBeenCalledWith(["b"])
  })

  // Enter still does take it — the two keys differ on purpose.
  it("Enter takes the highlighted suggestion where comma would not", async () => {
    const user = userEvent.setup()
    const { onChange, input } = renderCombobox()

    await user.click(input)
    await user.type(input, "b")
    await user.keyboard("{ArrowDown}{Enter}")
    expect(onChange).toHaveBeenCalledWith(["beta"])
  })

  // Picking with the mouse drops the picked tag out of the filtered list, so
  // every index after it shifts; a leftover highlight names a different tag.
  it("a pointer pick clears the highlight", async () => {
    const user = userEvent.setup()
    const { input } = renderCombobox()

    await user.click(input)
    await user.keyboard("{ArrowDown}")
    await user.click(screen.getAllByRole("option")[2])

    await user.click(input)
    expect(input).not.toHaveAttribute("aria-activedescendant")
  })

  // Closing by clicking away does not go through the Escape branch.
  it("closing the list by other means clears the highlight too", async () => {
    const user = userEvent.setup()
    const { input } = renderCombobox()

    await user.click(input)
    await user.keyboard("{ArrowDown}")
    expect(input).toHaveAttribute("aria-activedescendant")

    await user.click(document.body)
    await user.click(input)
    expect(input).not.toHaveAttribute("aria-activedescendant")
  })

  // Enter also confirms an IME candidate. Committing on that keystroke turns a
  // half-composed word into a tag.
  it("ignores Enter that is ending an IME composition", async () => {
    const user = userEvent.setup()
    const { onChange, input } = renderCombobox()

    await user.click(input)
    await user.type(input, "beta")

    fireEvent.keyDown(input, { key: "Enter", isComposing: true })
    expect(onChange).not.toHaveBeenCalled()

    // The same key once composition has ended does commit.
    fireEvent.keyDown(input, { key: "Enter" })
    expect(onChange).toHaveBeenCalledWith(["beta"])
  })

  // testId is optional and caller-supplied, so it cannot double as the DOM id.
  // Two instances that share one — or two that pass none at all — would
  // otherwise answer to the same listbox id, and aria-controls would point at
  // whichever the browser finds first. Both here share a testId deliberately:
  // deriving the id from it passes when they differ.
  it("gives each instance its own listbox id even with the same testId", () => {
    renderWithProviders({
      children: (
        <>
          <TagsInput
            values={[]}
            onChange={vi.fn()}
            testId="dup"
            autocomplete
            kind="file"
            suggestions={suggestions}
          />
          <TagsInput
            values={[]}
            onChange={vi.fn()}
            testId="dup"
            autocomplete
            kind="file"
            suggestions={suggestions}
          />
        </>
      ),
    })

    const [first, second] = screen.getAllByTestId("dup-input")
    const firstControls = first.getAttribute("aria-controls")
    const secondControls = second.getAttribute("aria-controls")

    expect(firstControls).toBeTruthy()
    expect(secondControls).toBeTruthy()
    expect(firstControls).not.toBe(secondControls)
  })
})
