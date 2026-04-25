import { ref, watch, type Ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'

/**
 * Options accepted by {@link useDebouncedSearch}.
 */
export interface UseDebouncedSearchOptions {
  /**
   * Initial value of the search ref. Defaults to an empty string.
   */
  initial?: string

  /**
   * Debounce window in milliseconds. Defaults to 300.
   */
  delay?: number

  /**
   * Minimum input length required before the callback fires. Shorter
   * queries are silently skipped so views don't have to re-implement
   * the "empty query is special" rule everywhere. Defaults to 0.
   */
  minLength?: number

  /**
   * Invoked with the trimmed, debounced query whenever it settles and
   * passes the {@link minLength} check. Async errors bubble up to the
   * caller untouched.
   */
  onSearch?: (_query: string) => void | Promise<void>
}

/**
 * Return shape of {@link useDebouncedSearch}.
 */
export interface UseDebouncedSearchReturn {
  /**
   * Two-way bindable ref that views wire to an Input `v-model`.
   */
  query: Ref<string>

  /**
   * Most recent value that was forwarded to `onSearch` (post-trim,
   * post-minLength). Useful for decorating the UI without re-running
   * the handler.
   */
  debouncedQuery: Ref<string>

  /**
   * Trigger a search immediately, bypassing the debounce window.
   * Handy for explicit "Search" buttons and form submissions.
   */
  flush: () => void
}

/**
 * Reactive, debounced search primitive that standardises the
 * "type-to-search" interaction across the app. The composable owns
 * the debounce timer, trims the query, and drops values shorter than
 * `minLength` before invoking the caller-supplied handler.
 *
 * Intentionally thin: it does not fetch data or manage loading state,
 * so the same primitive can back commodity lists, file pickers, and
 * settings screens without forcing any particular API shape.
 */
export function useDebouncedSearch(
  options: UseDebouncedSearchOptions = {},
): UseDebouncedSearchReturn {
  const { initial = '', delay = 300, minLength = 0, onSearch } = options

  const query = ref(initial)
  const debouncedQuery = ref(initial.trim())

  const run = (raw: string) => {
    const next = raw.trim()
    if (next.length < minLength) return
    debouncedQuery.value = next
    if (onSearch) {
      void onSearch(next)
    }
  }

  const debounced = useDebounceFn(run, delay)

  watch(query, (value) => {
    void debounced(value)
  })

  const flush = () => {
    run(query.value)
  }

  return { query, debouncedQuery, flush }
}
