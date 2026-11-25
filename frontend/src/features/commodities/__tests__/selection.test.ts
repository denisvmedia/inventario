import { act, renderHook } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { useCommoditySelection } from "@/features/commodities/selection"

function setup() {
  return renderHook(({ queryKey, pageKey }) => useCommoditySelection(queryKey, pageKey), {
    initialProps: { queryKey: "q1", pageKey: "1:25" },
  })
}

describe("useCommoditySelection", () => {
  it("unchecking a row of all matching keeps the rows shown as checked, after a refetch too", () => {
    const { result } = setup()
    act(() => result.current.selectAllMatching(30, "digest-30"))

    // A refetch replaced B with C on the page. In all matching mode every
    // row on the page shows as checked, C included.
    const pageIds = ["a", "c"]
    expect([...result.current.checkedOn(pageIds)]).toEqual(["a", "c"])

    act(() => result.current.toggle("a", pageIds))

    // What stays selected is what the user still sees checked, as ids, and
    // the count in the bar follows.
    expect(result.current.allMatching).toBe(false)
    expect([...result.current.checkedOn(pageIds)]).toEqual(["c"])
    expect(result.current.count).toBe(1)
    expect(result.current.toRequest("tags=x")).toEqual({ ids: ["c"] })
  })

  it("keeps the total and digest from the moment of Select all", () => {
    const { result } = setup()
    act(() => result.current.selectAllMatching(30, "digest-30"))
    expect(result.current.toRequest("tags=x")).toEqual({
      query: "tags=x",
      expectedCount: 30,
      expectedDigest: "digest-30",
    })
  })

  it("settles the selection a request was sent for", () => {
    const { result } = setup()
    act(() => result.current.togglePage(["a", "b", "c"]))
    const version = result.current.version

    act(() => result.current.settle(version, ["b"]))
    expect([...result.current.ids]).toEqual(["b"])

    act(() => result.current.togglePage(["a", "b", "c"]))
    act(() => result.current.settle(result.current.version, []))
    expect(result.current.count).toBe(0)
  })

  it("lets every request sent for one selection settle it, in the order they end", () => {
    const { result } = setup()
    act(() => result.current.togglePage(["a", "b"]))
    const version = result.current.version

    // A move and a delete were both sent for a and b. The move ends first
    // and keeps b, which it failed to move; the delete then removes both.
    act(() => result.current.settle(version, ["b"]))
    expect(result.current.version).toBe(version)
    act(() => result.current.settle(version, []))
    expect(result.current.count).toBe(0)
  })

  it("leaves a selection made after the request was sent", () => {
    const { result } = setup()
    act(() => result.current.togglePage(["a", "b"]))
    const version = result.current.version

    act(() => result.current.toggle("z", ["z"]))
    act(() => result.current.settle(version, ["a"]))
    expect([...result.current.ids].sort()).toEqual(["a", "b", "z"])

    act(() => result.current.settle(version, []))
    expect(result.current.count).toBe(3)
  })

  it("leaves the selection of a new query", () => {
    const { result, rerender } = setup()
    act(() => result.current.togglePage(["a", "b"]))
    const version = result.current.version

    rerender({ queryKey: "q2", pageKey: "1:25" })
    act(() => result.current.toggle("n", ["n"]))
    act(() => result.current.settle(version, ["a"]))
    expect([...result.current.ids]).toEqual(["n"])
  })
})
