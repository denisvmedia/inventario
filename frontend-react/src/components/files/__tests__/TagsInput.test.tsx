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
})
