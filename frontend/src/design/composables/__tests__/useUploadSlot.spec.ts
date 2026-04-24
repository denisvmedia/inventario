import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useUploadSlot, type UploadSlotStatus } from '../useUploadSlot'

function makeStatus(overrides: Partial<UploadSlotStatus> = {}): UploadSlotStatus {
  return {
    operationName: 'image-upload',
    activeUploads: 0,
    maxUploads: 3,
    availableUploads: 3,
    canStartUpload: true,
    ...overrides,
  }
}

describe('useUploadSlot', () => {
  it('refresh() pushes the fetcher result into the status ref', async () => {
    const snapshot = makeStatus({ availableUploads: 2 })
    const fetcher = vi.fn(async () => snapshot)
    const slot = useUploadSlot({ operation: 'image-upload', fetcher })

    const result = await slot.refresh()

    expect(result).toEqual(snapshot)
    expect(slot.status.value).toEqual(snapshot)
    expect(slot.loading.value).toBe(false)
    expect(fetcher).toHaveBeenCalledWith('image-upload')
  })

  it('captures fetcher failures in the error ref and re-throws', async () => {
    const failure = new Error('capacity service offline')
    const fetcher = vi.fn(async () => {
      throw failure
    })
    const slot = useUploadSlot({ operation: 'image-upload', fetcher })

    await expect(slot.refresh()).rejects.toBe(failure)
    expect(slot.error.value).toBe(failure)
    expect(slot.loading.value).toBe(false)
  })

  it('waitForCapacity resolves on the first success without polling', async () => {
    const fetcher = vi.fn(async () => makeStatus({ canStartUpload: true }))
    const slot = useUploadSlot({ operation: 'image-upload', fetcher })

    const result = await slot.waitForCapacity({ maxWaitMs: 1_000, pollIntervalMs: 50 })

    expect(result.canStartUpload).toBe(true)
    expect(fetcher).toHaveBeenCalledTimes(1)
  })
})

describe('useUploadSlot waitForCapacity polling', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('polls until canStartUpload flips to true', async () => {
    const responses: UploadSlotStatus[] = [
      makeStatus({ canStartUpload: false, availableUploads: 0 }),
      makeStatus({ canStartUpload: false, availableUploads: 0 }),
      makeStatus({ canStartUpload: true, availableUploads: 1 }),
    ]
    const fetcher = vi.fn(async () => responses.shift() as UploadSlotStatus)
    const slot = useUploadSlot({ operation: 'image-upload', fetcher })

    const promise = slot.waitForCapacity({ maxWaitMs: 10_000, pollIntervalMs: 100 })

    await vi.advanceTimersByTimeAsync(300)
    const result = await promise

    expect(result.canStartUpload).toBe(true)
    expect(fetcher).toHaveBeenCalledTimes(3)
  })

  it('throws when maxWaitMs elapses without capacity', async () => {
    const fetcher = vi.fn(async () => makeStatus({ canStartUpload: false }))
    const slot = useUploadSlot({ operation: 'image-upload', fetcher })

    const promise = slot.waitForCapacity({ maxWaitMs: 250, pollIntervalMs: 100 })
    const assertion = expect(promise).rejects.toThrow(/Timed out waiting for upload capacity/)

    await vi.advanceTimersByTimeAsync(500)
    await assertion
  })
})
