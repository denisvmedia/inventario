import { describe, expect, it, vi } from 'vitest'
import { useSignedUrl } from '../useSignedUrl'

describe('useSignedUrl', () => {
  it('caches by file id and short-circuits repeat resolves', async () => {
    const fetcher = vi.fn(async (id: string) => `https://cdn.example/${id}?sig=1`)
    const { url, resolve, loading } = useSignedUrl({ fetcher })

    const first = await resolve('abc')
    expect(first).toBe('https://cdn.example/abc?sig=1')
    expect(url.value).toBe('https://cdn.example/abc?sig=1')
    expect(loading.value).toBe(false)

    const second = await resolve('abc')
    expect(second).toBe(first)
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('re-fetches when called with { force: true }', async () => {
    let counter = 0
    const fetcher = vi.fn(async (id: string) => `https://cdn.example/${id}?sig=${++counter}`)
    const { resolve } = useSignedUrl({ fetcher })

    const first = await resolve('abc')
    const second = await resolve('abc', { force: true })

    expect(first).toBe('https://cdn.example/abc?sig=1')
    expect(second).toBe('https://cdn.example/abc?sig=2')
    expect(fetcher).toHaveBeenCalledTimes(2)
  })

  it('invalidate() drops the cache and clears url/error state', async () => {
    const fetcher = vi.fn(async (id: string) => `https://cdn.example/${id}`)
    const { url, resolve, invalidate } = useSignedUrl({ fetcher })

    await resolve('abc')
    expect(url.value).toBe('https://cdn.example/abc')

    invalidate()
    expect(url.value).toBeNull()

    await resolve('abc')
    expect(fetcher).toHaveBeenCalledTimes(2)
  })

  it('captures fetcher errors in `error` and re-throws to the caller', async () => {
    const failure = new Error('network down')
    const fetcher = vi.fn(async () => {
      throw failure
    })
    const { error, loading, resolve } = useSignedUrl({ fetcher })

    await expect(resolve('abc')).rejects.toBe(failure)
    expect(error.value).toBe(failure)
    expect(loading.value).toBe(false)
  })

  it('rejects when called without a file id', async () => {
    const fetcher = vi.fn(async () => 'unused')
    const { resolve } = useSignedUrl({ fetcher })

    await expect(resolve('')).rejects.toThrow(/fileId is required/)
    expect(fetcher).not.toHaveBeenCalled()
  })
})
