import type { ReactNode } from "react"
import { describe, expect, it } from "vitest"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook, waitFor } from "@testing-library/react"

import {
  listPageQuery,
  resolveListPosition,
  useListNavigation,
  type ListPage,
  type ListPageSource,
} from "@/hooks/useListNavigation"

// A list of `ids` cut into pages of `size`, as the server would return them.
function paged(ids: string[], size: number): Map<number, ListPage> {
  const pages = new Map<number, ListPage>()
  const count = Math.max(1, Math.ceil(ids.length / size))
  for (let p = 1; p <= count; p++) {
    pages.set(p, { ids: ids.slice((p - 1) * size, p * size), total: ids.length })
  }
  return pages
}

function only(pages: Map<number, ListPage>, keep: number[]): Map<number, ListPage> {
  return new Map([...pages].filter(([p]) => keep.includes(p)))
}

const SEVEN = ["a", "b", "c", "d", "e", "f", "g"]

describe("resolveListPosition", () => {
  it("asks for the context page first", () => {
    const r = resolveListPosition({
      currentId: "b",
      page: 1,
      pageSize: 3,
      pages: new Map(),
      anchor: null,
    })
    expect(r.need).toEqual([1])
    expect(r.offset).toBeNull()
  })

  it("finds neighbors on the same page", () => {
    const r = resolveListPosition({
      currentId: "b",
      page: 1,
      pageSize: 3,
      pages: paged(SEVEN, 3),
      anchor: null,
    })
    expect(r).toEqual({
      need: [],
      offset: 1,
      total: 7,
      page: 1,
      previous: { id: "a", page: 1 },
      next: { id: "c", page: 1 },
    })
  })

  it("crosses into the next page from the last record of a page", () => {
    const pages = paged(SEVEN, 3)
    const waiting = resolveListPosition({
      currentId: "c",
      page: 1,
      pageSize: 3,
      pages: only(pages, [1]),
      anchor: null,
    })
    expect(waiting.need).toEqual([2])
    expect(waiting.next).toBeNull()
    expect(waiting.offset).toBe(2)

    const r = resolveListPosition({ currentId: "c", page: 1, pageSize: 3, pages, anchor: null })
    expect(r.next).toEqual({ id: "d", page: 2 })
    expect(r.previous).toEqual({ id: "b", page: 1 })
  })

  it("crosses into the previous page from the first record of a page", () => {
    const pages = paged(SEVEN, 3)
    const r = resolveListPosition({
      currentId: "d",
      page: 2,
      pageSize: 3,
      pages: only(pages, [1, 2]),
      anchor: null,
    })
    expect(r.previous).toEqual({ id: "c", page: 1 })
    expect(r.next).toEqual({ id: "e", page: 2 })
    expect(r.offset).toBe(3)
  })

  it("has no previous on the first record and no next on the last", () => {
    const pages = paged(SEVEN, 3)
    const first = resolveListPosition({ currentId: "a", page: 1, pageSize: 3, pages, anchor: null })
    expect(first.previous).toBeNull()
    expect(first.need).toEqual([])

    const last = resolveListPosition({ currentId: "g", page: 3, pageSize: 3, pages, anchor: null })
    expect(last.next).toBeNull()
    // "g" is alone on page 3, so the record before it is on page 2.
    expect(last.previous).toEqual({ id: "f", page: 2 })
    expect(last.need).toEqual([])
  })

  it("finds a record that moved to the page before the one in context", () => {
    // "c" was the last record of page 1 and the user moved on to "d" on
    // page 2; then "c" left the list, so "d" is now last on page 1.
    const pages = paged(["a", "b", "d", "e", "f", "g"], 3)
    const r = resolveListPosition({ currentId: "d", page: 2, pageSize: 3, pages, anchor: null })
    expect(r.page).toBe(1)
    expect(r.offset).toBe(2)
    expect(r.previous).toEqual({ id: "b", page: 1 })
    expect(r.next).toEqual({ id: "e", page: 2 })
  })

  it("waits for the pages on both sides before deciding a record is gone", () => {
    const pages = paged(["a", "b", "d", "e", "f", "g"], 3)
    const r = resolveListPosition({
      currentId: "c",
      page: 2,
      pageSize: 3,
      pages: only(pages, [2]),
      anchor: 2,
    })
    expect(r.need).toEqual([1])
    expect(r.offset).toBeNull()
  })

  it("continues from where a record was after an edit took it out of the list", () => {
    // "d" sat at offset 3 (first on page 2). After the save it is gone and
    // "e" has moved up into its place.
    const pages = paged(["a", "b", "c", "e", "f", "g"], 3)
    const r = resolveListPosition({ currentId: "d", page: 2, pageSize: 3, pages, anchor: 3 })
    expect(r.offset).toBeNull()
    expect(r.next).toEqual({ id: "e", page: 2 })
    expect(r.previous).toEqual({ id: "c", page: 1 })
    expect(r.page).toBe(2)
  })

  it("continues across a page edge when the record that left was last on its page", () => {
    // "c" was last on page 1 (offset 2). Without it, page 1 ends with "d",
    // which used to open page 2: Next must land on "d", not skip to "e".
    const pages = paged(["a", "b", "d", "e", "f", "g"], 3)
    const r = resolveListPosition({ currentId: "c", page: 1, pageSize: 3, pages, anchor: 2 })
    expect(r.next).toEqual({ id: "d", page: 1 })
    expect(r.previous).toEqual({ id: "b", page: 1 })
  })

  it("ends the walk when the record that left was the last one", () => {
    const pages = paged(["a", "b", "c", "d", "e", "f"], 3)
    // The server answers a page past the end with no rows.
    pages.set(3, { ids: [], total: 6 })
    const r = resolveListPosition({ currentId: "g", page: 3, pageSize: 3, pages, anchor: 6 })
    expect(r.next).toBeNull()
    expect(r.previous).toEqual({ id: "f", page: 2 })
    expect(r.page).toBe(2)
  })

  it("gives a record never seen in the list no position and no neighbors", () => {
    const r = resolveListPosition({
      currentId: "zz",
      page: 1,
      pageSize: 3,
      pages: paged(SEVEN, 3),
      anchor: null,
    })
    expect(r).toEqual({ need: [], offset: null, total: 7, page: 1, previous: null, next: null })
  })

  it("handles a context page past the end of a list that shrank", () => {
    const pages = paged(["a", "b", "c", "d"], 3)
    pages.set(3, { ids: [], total: 4 })
    const r = resolveListPosition({ currentId: "d", page: 3, pageSize: 3, pages, anchor: null })
    expect(r.page).toBe(2)
    expect(r.offset).toBe(3)
    expect(r.previous).toEqual({ id: "c", page: 1 })
    expect(r.next).toBeNull()
  })
})

// --- Hook ---------------------------------------------------------------

function makeSource(
  list: { ids: string[] },
  page: number,
  fetched: number[],
  pageSize = 3
): ListPageSource {
  return {
    pageSize,
    page,
    pageQuery: (p) =>
      listPageQuery({
        queryKey: ["test-list", p],
        queryFn: async () => {
          fetched.push(p)
          return {
            rows: list.ids.slice((p - 1) * pageSize, p * pageSize),
            count: list.ids.length,
          }
        },
        select: (data) => ({ ids: data.rows, total: data.count }),
      }),
  }
}

function wrapperFor(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }
}

function newClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } })
}

describe("useListNavigation", () => {
  it("is inactive without a source", () => {
    const { result } = renderHook(() => useListNavigation({ source: null, currentId: "a" }), {
      wrapper: wrapperFor(newClient()),
    })
    expect(result.current.active).toBe(false)
    expect(result.current.next).toBeNull()
  })

  it("fetches the next page only when the record sits on a page edge", async () => {
    const fetched: number[] = []
    const list = { ids: SEVEN }
    const client = newClient()

    const mid = renderHook(
      () => useListNavigation({ source: makeSource(list, 1, fetched), currentId: "b" }),
      { wrapper: wrapperFor(client) }
    )
    await waitFor(() => expect(mid.result.current.loading).toBe(false))
    expect(mid.result.current.position).toBe(2)
    expect(mid.result.current.total).toBe(7)
    expect(fetched).toEqual([1])
    mid.unmount()

    const edge = renderHook(
      () => useListNavigation({ source: makeSource(list, 1, fetched), currentId: "c" }),
      { wrapper: wrapperFor(client) }
    )
    await waitFor(() => expect(edge.result.current.next).toEqual({ id: "d", page: 2 }))
    expect(edge.result.current.loading).toBe(false)
    expect(fetched).toEqual([1, 2])
  })

  it("keeps the walk going after a refetch drops the current record", async () => {
    const fetched: number[] = []
    const list = { ids: [...SEVEN] }
    const client = newClient()
    const source = makeSource(list, 2, fetched)

    const { result } = renderHook(() => useListNavigation({ source, currentId: "d" }), {
      wrapper: wrapperFor(client),
    })
    await waitFor(() => expect(result.current.position).toBe(4))
    expect(result.current.next).toEqual({ id: "e", page: 2 })

    // An edit takes "d" out of the filter; the list refetches.
    list.ids = list.ids.filter((id) => id !== "d")
    await act(() => client.invalidateQueries({ queryKey: ["test-list"] }))

    await waitFor(() => expect(result.current.inList).toBe(false))
    expect(result.current.position).toBeNull()
    expect(result.current.next).toEqual({ id: "e", page: 2 })
    expect(result.current.previous).toEqual({ id: "c", page: 1 })
  })

  it("reports a failed page", async () => {
    const client = newClient()
    const source: ListPageSource = {
      pageSize: 3,
      page: 1,
      pageQuery: (p) =>
        listPageQuery({
          queryKey: ["broken", p],
          queryFn: async (): Promise<ListPage> => {
            throw new Error("boom")
          },
          select: (data) => data,
        }),
    }
    const { result } = renderHook(() => useListNavigation({ source, currentId: "a" }), {
      wrapper: wrapperFor(client),
    })
    await waitFor(() => expect(result.current.error).toBe(true))
    expect(result.current.loading).toBe(false)
    expect(result.current.next).toBeNull()
  })
})
