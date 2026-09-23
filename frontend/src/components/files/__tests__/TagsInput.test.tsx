import { describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
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
