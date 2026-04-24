import { computed, ref, type ComputedRef, type Ref } from 'vue'

/**
 * Options accepted by {@link usePagination}.
 */
export interface UsePaginationOptions {
  /**
   * Starting page (1-based). Defaults to 1.
   */
  initialPage?: number

  /**
   * Initial page size. Defaults to 20, matching the Phase 0 baseline.
   */
  pageSize?: number

  /**
   * Initial total item count. Defaults to 0 so views can initialise
   * the composable before the first request has completed.
   */
  total?: number
}

/**
 * Return shape of {@link usePagination}.
 */
export interface UsePaginationReturn {
  page: Ref<number>
  pageSize: Ref<number>
  total: Ref<number>

  totalPages: ComputedRef<number>
  hasNext: ComputedRef<boolean>
  hasPrev: ComputedRef<boolean>

  /**
   * Offset suitable for `skip`/`offset` style APIs.
   */
  offset: ComputedRef<number>

  setPage: (_page: number) => void
  setPageSize: (_size: number) => void
  setTotal: (_total: number) => void
  nextPage: () => void
  prevPage: () => void
  reset: () => void
}

/**
 * Framework-agnostic pagination state container. Keeps the page,
 * page size, and total item count reactive, derives the common
 * computed values (`totalPages`, `hasNext`, `hasPrev`, `offset`),
 * and clamps navigation so callers don't have to reimplement bounds
 * checks on every list screen.
 *
 * The composable is deliberately transport-agnostic — it does not
 * fetch, it does not read from the route. Wiring into `vue-router`
 * query params is left to the consuming view so the same primitive
 * powers modal pickers, cursor-less infinite lists, and URL-backed
 * list pages alike.
 */
export function usePagination(
  options: UsePaginationOptions = {},
): UsePaginationReturn {
  const { initialPage = 1, pageSize: initialPageSize = 20, total: initialTotal = 0 } = options

  const page = ref(clampPage(initialPage, initialTotal, initialPageSize))
  const pageSize = ref(Math.max(1, initialPageSize))
  const total = ref(Math.max(0, initialTotal))

  const totalPages = computed(() =>
    total.value === 0 ? 0 : Math.max(1, Math.ceil(total.value / pageSize.value)),
  )

  const hasNext = computed(() => page.value < totalPages.value)
  const hasPrev = computed(() => page.value > 1)
  const offset = computed(() => (page.value - 1) * pageSize.value)

  const setPage = (next: number) => {
    page.value = clampPage(next, total.value, pageSize.value)
  }

  const setPageSize = (size: number) => {
    const normalised = Math.max(1, Math.floor(size))
    pageSize.value = normalised
    // Re-clamp current page against the new page size so we never
    // land on a page that no longer exists.
    page.value = clampPage(page.value, total.value, normalised)
  }

  const setTotal = (nextTotal: number) => {
    const normalised = Math.max(0, Math.floor(nextTotal))
    total.value = normalised
    page.value = clampPage(page.value, normalised, pageSize.value)
  }

  const nextPage = () => {
    if (hasNext.value) page.value += 1
  }

  const prevPage = () => {
    if (hasPrev.value) page.value -= 1
  }

  const reset = () => {
    page.value = 1
  }

  return {
    page,
    pageSize,
    total,
    totalPages,
    hasNext,
    hasPrev,
    offset,
    setPage,
    setPageSize,
    setTotal,
    nextPage,
    prevPage,
    reset,
  }
}

function clampPage(candidate: number, total: number, pageSize: number): number {
  const maxPage = total === 0 ? 1 : Math.max(1, Math.ceil(total / Math.max(1, pageSize)))
  if (!Number.isFinite(candidate) || candidate < 1) return 1
  return Math.min(Math.floor(candidate), maxPage)
}
