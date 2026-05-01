import { describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { TagsInput } from "@/components/files/TagsInput"

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
