import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../api', () => ({
  default: { get: vi.fn() },
}))

import api from '../api'
import searchService from '../searchService'

const mockedGet = api.get as unknown as ReturnType<typeof vi.fn>

describe('searchService.search', () => {
  beforeEach(() => {
    mockedGet.mockReset()
  })
  afterEach(() => {
    mockedGet.mockReset()
  })

  it('returns an empty list and skips the request for blank queries', async () => {
    const result = await searchService.search('   ')

    expect(result).toEqual({ data: [] })
    expect(mockedGet).not.toHaveBeenCalled()
  })

  it('forwards trimmed query, default type, limit and offset', async () => {
    mockedGet.mockResolvedValueOnce({ data: { data: [], meta: { total: 0 } } })

    await searchService.search('  hello  ')

    expect(mockedGet).toHaveBeenCalledTimes(1)
    expect(mockedGet).toHaveBeenCalledWith('/api/v1/search', {
      params: { q: 'hello', type: 'commodities' },
    })
  })

  it('passes the explicit type / limit / offset options', async () => {
    mockedGet.mockResolvedValueOnce({ data: { data: [], meta: { total: 0 } } })

    await searchService.search('cup', { type: 'files', limit: 5, offset: 10 })

    expect(mockedGet).toHaveBeenCalledWith('/api/v1/search', {
      params: { q: 'cup', type: 'files', limit: 5, offset: 10 },
    })
  })

  it('coerces a non-array response data to []', async () => {
    mockedGet.mockResolvedValueOnce({ data: { data: null, meta: { total: 0 } } })

    const result = await searchService.search('x')

    expect(result.data).toEqual([])
  })

  it('returns the data array and meta from the backend', async () => {
    const items = [
      { id: '1', type: 'commodities', attributes: { name: 'Coffee' } },
      { id: '2', type: 'commodities', attributes: { name: 'Cookbook' } },
    ]
    mockedGet.mockResolvedValueOnce({ data: { data: items, meta: { total: 2 } } })

    const result = await searchService.search('co')

    expect(result.data).toEqual(items)
    expect(result.meta).toEqual({ total: 2 })
  })

  it('propagates errors from the underlying api', async () => {
    mockedGet.mockRejectedValueOnce(new Error('server-down'))

    await expect(searchService.search('x')).rejects.toThrow('server-down')
  })
})
