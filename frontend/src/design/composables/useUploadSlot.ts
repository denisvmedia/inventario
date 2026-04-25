import { ref, type Ref } from 'vue'

/**
 * Snapshot of remote upload-slot state for a given operation, as
 * returned by the backend's `/upload-slots/check` endpoint (see
 * `uploadSlotService.checkUploadCapacity`).
 */
export interface UploadSlotStatus {
  operationName: string
  activeUploads: number
  maxUploads: number
  availableUploads: number
  canStartUpload: boolean
  retryAfterSeconds?: number
}

/**
 * Callback that resolves to a fresh {@link UploadSlotStatus} for the
 * given operation name. Injected so this composable doesn't depend
 * on `uploadSlotService` directly; production code wires it to
 * `uploadSlotService.checkUploadCapacity` and maps the JSON:API
 * payload into the flat shape above.
 */
export type UploadSlotFetcher = (_operation: string) => Promise<UploadSlotStatus>

/**
 * Options accepted by {@link useUploadSlot}.
 */
export interface UseUploadSlotOptions {
  /** Operation name passed through to the fetcher. */
  operation: string

  /** Status fetcher; see {@link UploadSlotFetcher}. */
  fetcher: UploadSlotFetcher
}

/**
 * Return shape of {@link useUploadSlot}.
 */
export interface UseUploadSlotReturn {
  status: Ref<UploadSlotStatus | null>
  loading: Ref<boolean>
  error: Ref<Error | null>

  /** Fetch the current slot status. Returns the fresh snapshot. */
  refresh: () => Promise<UploadSlotStatus>

  /**
   * Resolve once `canStartUpload` is true, polling at the given
   * interval. Throws if `maxWaitMs` elapses without capacity. The
   * default bounds (30 s window, 2 s poll) match
   * `uploadSlotService.waitForAvailability` so migrated call-sites
   * behave identically.
   */
  waitForCapacity: (_opts?: { maxWaitMs?: number; pollIntervalMs?: number }) => Promise<UploadSlotStatus>
}

/**
 * Reactive facade over the backend's upload-slot throttle. Keeps the
 * latest {@link UploadSlotStatus} in a ref, exposes `loading`/`error`
 * flags, and provides a `waitForCapacity` helper for flows that need
 * to block a user action until a slot frees up.
 */
export function useUploadSlot(options: UseUploadSlotOptions): UseUploadSlotReturn {
  const { operation, fetcher } = options

  const status = ref<UploadSlotStatus | null>(null)
  const loading = ref(false)
  const error = ref<Error | null>(null)

  const refresh = async (): Promise<UploadSlotStatus> => {
    loading.value = true
    error.value = null
    try {
      const next = await fetcher(operation)
      status.value = next
      return next
    } catch (err) {
      error.value = err instanceof Error ? err : new Error(String(err))
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const waitForCapacity = async (
    opts: { maxWaitMs?: number; pollIntervalMs?: number } = {},
  ): Promise<UploadSlotStatus> => {
    const maxWaitMs = opts.maxWaitMs ?? 30_000
    const pollIntervalMs = Math.max(50, opts.pollIntervalMs ?? 2_000)

    const start = Date.now()
    // Always run at least one check before timing out, so a
    // `maxWaitMs` of 0 still yields one attempt.
    let attempts = 0
    while (attempts === 0 || Date.now() - start < maxWaitMs) {
      attempts += 1
      const snapshot = await refresh()
      if (snapshot.canStartUpload) return snapshot
      if (Date.now() - start >= maxWaitMs) break
      await delay(pollIntervalMs)
    }

    throw new Error(
      `Timed out waiting for upload capacity on "${operation}" after ${maxWaitMs}ms`,
    )
  }

  return { status, loading, error, refresh, waitForCapacity }
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
