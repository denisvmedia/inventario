import { describe, expect, it } from 'vitest'
import { usePagination } from '../usePagination'

describe('usePagination', () => {
  it('uses the documented defaults when called with no options', () => {
    const p = usePagination()
    expect(p.page.value).toBe(1)
    expect(p.pageSize.value).toBe(20)
    expect(p.total.value).toBe(0)
    expect(p.totalPages.value).toBe(0)
    expect(p.hasPrev.value).toBe(false)
    expect(p.hasNext.value).toBe(false)
    expect(p.offset.value).toBe(0)
  })

  it('derives totalPages, hasNext, hasPrev and offset from state', () => {
    const p = usePagination({ pageSize: 10, total: 25 })
    expect(p.totalPages.value).toBe(3)
    expect(p.hasPrev.value).toBe(false)
    expect(p.hasNext.value).toBe(true)

    p.setPage(2)
    expect(p.offset.value).toBe(10)
    expect(p.hasPrev.value).toBe(true)
    expect(p.hasNext.value).toBe(true)

    p.setPage(3)
    expect(p.offset.value).toBe(20)
    expect(p.hasNext.value).toBe(false)
  })

  it('clamps setPage to the valid [1, totalPages] range', () => {
    const p = usePagination({ pageSize: 10, total: 25 })

    p.setPage(99)
    expect(p.page.value).toBe(3)

    p.setPage(-5)
    expect(p.page.value).toBe(1)

    p.setPage(Number.NaN)
    expect(p.page.value).toBe(1)
  })

  it('re-clamps the current page when pageSize shrinks the bounds', () => {
    const p = usePagination({ pageSize: 10, total: 25 })
    p.setPage(3)
    expect(p.page.value).toBe(3)

    p.setPageSize(25)
    expect(p.totalPages.value).toBe(1)
    expect(p.page.value).toBe(1)
  })

  it('re-clamps the current page when total shrinks', () => {
    const p = usePagination({ pageSize: 10, total: 50 })
    p.setPage(5)
    expect(p.page.value).toBe(5)

    p.setTotal(12)
    expect(p.totalPages.value).toBe(2)
    expect(p.page.value).toBe(2)
  })

  it('nextPage and prevPage respect the hasNext/hasPrev guards', () => {
    const p = usePagination({ pageSize: 10, total: 25 })

    p.prevPage()
    expect(p.page.value).toBe(1)

    p.nextPage()
    p.nextPage()
    p.nextPage()
    p.nextPage()
    expect(p.page.value).toBe(3)
  })

  it('reset() returns page to 1 without touching total or pageSize', () => {
    const p = usePagination({ pageSize: 10, total: 50 })
    p.setPage(4)
    p.reset()
    expect(p.page.value).toBe(1)
    expect(p.pageSize.value).toBe(10)
    expect(p.total.value).toBe(50)
  })
})
