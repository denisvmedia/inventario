import { describe, expect, it, vi } from "vitest"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { Pagination, pageRange, pageWithin } from "@/components/common/Pagination"
import { renderWithProviders } from "@/test/render"

describe("pageRange", () => {
  it("lists every page while they fit", () => {
    expect(pageRange(1, 7)).toEqual([1, 2, 3, 4, 5, 6, 7])
  })

  // Past seven the control keeps a fixed width by collapsing the middle,
  // always keeping the first and last page reachable in one click.
  it("collapses the middle and keeps both ends", () => {
    expect(pageRange(1, 20)).toEqual([1, 2, "ellipsis", 20])
    expect(pageRange(10, 20)).toEqual([1, "ellipsis", 9, 10, 11, "ellipsis", 20])
    expect(pageRange(20, 20)).toEqual([1, "ellipsis", 19, 20])
  })

  it("does not emit an ellipsis standing in for a single page", () => {
    // 3 would be the only hidden page between 2 and 4, so it is shown.
    expect(pageRange(3, 8)).toEqual([1, 2, 3, 4, "ellipsis", 8])
  })
})

describe("<Pagination />", () => {
  function render(page: number, totalPages: number) {
    const onChange = vi.fn()
    renderWithProviders({
      children: <Pagination page={page} totalPages={totalPages} onChange={onChange} />,
    })
    return { onChange }
  }

  it("moves by one in each direction", async () => {
    const user = userEvent.setup()
    const { onChange } = render(3, 10)

    await user.click(screen.getByLabelText("Previous page"))
    expect(onChange).toHaveBeenCalledWith(2)
    await user.click(screen.getByLabelText("Next page"))
    expect(onChange).toHaveBeenCalledWith(4)
  })

  it("jumps to a numbered page", async () => {
    const user = userEvent.setup()
    const { onChange } = render(1, 10)

    await user.click(screen.getByTestId("pagination-page-10"))
    expect(onChange).toHaveBeenCalledWith(10)
  })

  // The ends are where an off-by-one shows up: a previous from page one would
  // ask for page zero.
  it("disables the arrow that would step past an end", () => {
    render(1, 5)
    expect(screen.getByLabelText("Previous page")).toBeDisabled()
    expect(screen.getByLabelText("Next page")).not.toBeDisabled()
  })

  it("disables the other one on the last page", () => {
    render(5, 5)
    expect(screen.getByLabelText("Next page")).toBeDisabled()
  })

  it("marks the current page for assistive technology", () => {
    render(3, 10)
    expect(screen.getByTestId("pagination-page-3")).toHaveAttribute("aria-current", "page")
    expect(screen.getByTestId("pagination-page-2")).not.toHaveAttribute("aria-current")
  })

  // A view with two lists needs two sets of selectors.
  it("scopes its test ids", () => {
    renderWithProviders({
      children: <Pagination page={1} totalPages={3} onChange={vi.fn()} testId="loans-pagination" />,
    })
    expect(screen.getByTestId("loans-pagination")).toBeInTheDocument()
    expect(screen.getByTestId("loans-pagination-page-2")).toBeInTheDocument()
  })
})

describe("pageWithin", () => {
  it("counts pages, with a full last page not adding an empty one", () => {
    const noop = vi.fn()
    expect(pageWithin(1, noop, 0, 24).totalPages).toBe(1)
    expect(pageWithin(1, noop, 1, 24).totalPages).toBe(1)
    expect(pageWithin(1, noop, 24, 24).totalPages).toBe(1)
    expect(pageWithin(1, noop, 25, 24).totalPages).toBe(2)
    expect(pageWithin(1, noop, 48, 24).totalPages).toBe(2)
    expect(noop).not.toHaveBeenCalled()
  })

  it("leaves a page inside the range alone", () => {
    const setPage = vi.fn()
    expect(pageWithin(2, setPage, 48, 24)).toEqual({ page: 2, totalPages: 2 })
    expect(setPage).not.toHaveBeenCalled()
  })

  it("folds a page past the end back to the last one", () => {
    const setPage = vi.fn()
    expect(pageWithin(5, setPage, 48, 24)).toEqual({ page: 2, totalPages: 2 })
    expect(setPage).toHaveBeenCalledWith(2)
  })

  it("folds back to page one when everything is gone", () => {
    const setPage = vi.fn()
    expect(pageWithin(3, setPage, 0, 24)).toEqual({ page: 1, totalPages: 1 })
    expect(setPage).toHaveBeenCalledWith(1)
  })

  // Before the first response there is no range to fold against. Nothing is
  // written back, so the requested page survives into the render where the
  // total is known.
  it("does not fold while the total is unknown", () => {
    const setPage = vi.fn()
    expect(pageWithin(3, setPage, undefined, 24).page).toBe(1)
    expect(setPage).not.toHaveBeenCalled()
  })
})
