import { ref, type Ref } from 'vue'

/**
 * Callback that resolves to a signed URL for the given file id.
 * Plays the role of the injection seam so views can wire the
 * composable to whichever service owns signed-url generation
 * (currently `fileService.generateSignedUrl`) without the
 * composable depending on that service directly.
 */
export type SignedUrlFetcher = (_fileId: string) => Promise<string>

/**
 * Options accepted by {@link useSignedUrl}.
 */
export interface UseSignedUrlOptions {
  /**
   * Fetcher that returns a signed URL for a file id.
   */
  fetcher: SignedUrlFetcher
}

/**
 * Return shape of {@link useSignedUrl}.
 */
export interface UseSignedUrlReturn {
  /** Current signed URL, or null when no fetch has completed yet. */
  url: Ref<string | null>

  /** True while a fetch is in flight. */
  loading: Ref<boolean>

  /** Last error raised by the fetcher, or null. */
  error: Ref<Error | null>

  /**
   * Resolve the signed URL for a file id. Repeat calls for the same
   * id short-circuit to the cached value; pass `{ force: true }` to
   * bypass the cache (e.g. after the backing URL has expired).
   */
  resolve: (_fileId: string, _opts?: { force?: boolean }) => Promise<string>

  /**
   * Drop all cached URLs. Useful when the current tenant/group
   * changes and previously-issued URLs are no longer valid.
   */
  invalidate: () => void
}

/**
 * Reactive wrapper around an async signed-URL fetcher with per-id
 * memoisation, `loading`/`error` refs, and an `invalidate` escape
 * hatch. Deliberately accepts the fetcher as a dependency so the
 * composable stays decoupled from `fileService` and is trivially
 * testable without mocking axios.
 */
export function useSignedUrl(options: UseSignedUrlOptions): UseSignedUrlReturn {
  const { fetcher } = options

  const url = ref<string | null>(null)
  const loading = ref(false)
  const error = ref<Error | null>(null)

  const cache = new Map<string, string>()

  const resolve = async (fileId: string, opts: { force?: boolean } = {}): Promise<string> => {
    if (!fileId) {
      throw new Error('useSignedUrl: fileId is required')
    }

    if (!opts.force) {
      const cached = cache.get(fileId)
      if (cached) {
        url.value = cached
        return cached
      }
    }

    loading.value = true
    error.value = null
    try {
      const next = await fetcher(fileId)
      cache.set(fileId, next)
      url.value = next
      return next
    } catch (err) {
      error.value = err instanceof Error ? err : new Error(String(err))
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const invalidate = () => {
    cache.clear()
    url.value = null
    error.value = null
  }

  return { url, loading, error, resolve, invalidate }
}
