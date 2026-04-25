import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { useDebouncedSearch } from '../useDebouncedSearch'

describe('useDebouncedSearch', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('fires onSearch with the trimmed query after the debounce window', async () => {
    const onSearch = vi.fn()
    const { query, debouncedQuery } = useDebouncedSearch({ delay: 200, onSearch })

    query.value = '  camera  '
    await nextTick()

    expect(onSearch).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(200)

    expect(onSearch).toHaveBeenCalledTimes(1)
    expect(onSearch).toHaveBeenCalledWith('camera')
    expect(debouncedQuery.value).toBe('camera')
  })

  it('collapses rapid typing into a single call with the latest value', async () => {
    const onSearch = vi.fn()
    const { query } = useDebouncedSearch({ delay: 150, onSearch })

    query.value = 'c'
    await nextTick()
    await vi.advanceTimersByTimeAsync(50)
    query.value = 'ca'
    await nextTick()
    await vi.advanceTimersByTimeAsync(50)
    query.value = 'cam'
    await nextTick()

    expect(onSearch).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(150)

    expect(onSearch).toHaveBeenCalledTimes(1)
    expect(onSearch).toHaveBeenCalledWith('cam')
  })

  it('skips dispatch when the trimmed query is shorter than minLength', async () => {
    const onSearch = vi.fn()
    const { query, debouncedQuery } = useDebouncedSearch({
      delay: 100,
      minLength: 3,
      onSearch,
    })

    query.value = 'ca'
    await nextTick()
    await vi.advanceTimersByTimeAsync(100)

    expect(onSearch).not.toHaveBeenCalled()
    expect(debouncedQuery.value).toBe('')

    query.value = 'cam'
    await nextTick()
    await vi.advanceTimersByTimeAsync(100)

    expect(onSearch).toHaveBeenCalledWith('cam')
  })

  it('flush() bypasses the debounce window and fires synchronously', async () => {
    const onSearch = vi.fn()
    const { query, flush } = useDebouncedSearch({ delay: 500, onSearch })

    query.value = 'camera'
    await nextTick()

    flush()
    expect(onSearch).toHaveBeenCalledTimes(1)
    expect(onSearch).toHaveBeenCalledWith('camera')
  })

  it('honours the initial option and exposes it as debouncedQuery', () => {
    const { query, debouncedQuery } = useDebouncedSearch({ initial: 'seed' })
    expect(query.value).toBe('seed')
    expect(debouncedQuery.value).toBe('seed')
  })
})
