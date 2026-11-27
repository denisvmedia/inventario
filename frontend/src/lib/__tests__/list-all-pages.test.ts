import { describe, expect, it, vi } from "vitest"

import { listAllPages, MAX_PAGE_SIZE, type ListPage } from "@/lib/list-all-pages"

type Row = { id: string }

function rows(from: number, count: number): Row[] {
  return Array.from({ length: count }, (_, i) => ({ id: `r${from + i}` }))
}

// A server that answers pages of pageSize rows out of list, with the total.
function pagedServer(list: Row[], pageSize: number, opts: { total?: boolean } = {}) {
  return vi.fn(async (page: number): Promise<ListPage<Row>> => {
    const start = (page - 1) * pageSize
    return {
      rows: list.slice(start, start + pageSize),
      total: opts.total === false ? undefined : list.length,
    }
  })
}

describe("listAllPages", () => {
  it("reads every page of a server that caps the page below the size it asks for", async () => {
    const list = rows(1, 120)
    const fetchPage = pagedServer(list, 50)

    const got = await listAllPages(fetchPage)

    expect(got).toEqual(list)
    expect(fetchPage).toHaveBeenCalledTimes(3)
    expect(fetchPage).toHaveBeenCalledWith(1, MAX_PAGE_SIZE)
  })

  it("stops at the first short page when the server reports no total", async () => {
    const fetchPage = pagedServer(rows(1, 150), MAX_PAGE_SIZE, { total: false })

    const got = await listAllPages(fetchPage)

    expect(got).toHaveLength(150)
    expect(fetchPage).toHaveBeenCalledTimes(2)
  })

  // A row created at the top between two reads moves the last row of page 1
  // onto page 2.
  it("holds each id once when a row comes back on the next page", async () => {
    const fetchPage = vi.fn(async (page: number): Promise<ListPage<Row>> => {
      if (page === 1) return { rows: rows(1, 100), total: 150 }
      return { rows: rows(100, 51), total: 151 }
    })

    const got = await listAllPages(fetchPage)

    expect(got.map((row) => row.id)).toEqual(rows(1, 150).map((row) => row.id))
    expect(new Set(got.map((row) => row.id)).size).toBe(got.length)
  })

  it("stops when a page brings no new row", async () => {
    const fetchPage = vi.fn(async (): Promise<ListPage<Row>> => ({
      rows: rows(1, 100),
      total: 500,
    }))

    const got = await listAllPages(fetchPage)

    expect(got).toHaveLength(100)
    expect(fetchPage).toHaveBeenCalledTimes(2)
  })
})
