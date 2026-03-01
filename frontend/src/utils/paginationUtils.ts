/** Hard cap to prevent runaway loops in case of unexpected API responses. */
const MAX_PAGES = 1000

/**
 * Fetches all pages of a paginated resource by iterating through pages until exhausted.
 * Use this for lookup/reference data that must be complete (e.g., area/location name maps).
 *
 * Includes two safety guards against infinite loops:
 * - A hard page cap of 1000 pages.
 * - A fallback stop when total_pages is missing/invalid: stops when fewer items than
 *   requested are returned (indicating the last page).
 *
 * @param fetchFn - Function accepting { page, per_page } params and returning a paginated response
 * @param perPage - Items per page to request (capped by backend, typically 100 max)
 * @returns Flat array of all items across all pages
 */
export async function fetchAll<T>(
  fetchFn: (_params: { page: number; per_page: number }) => Promise<{ data: { data: T[]; meta: { total_pages: number } } }>,
  perPage = 100,
): Promise<T[]> {
  let page = 1
  const allItems: T[] = []

  while (page <= MAX_PAGES) {
    const response = await fetchFn({ page, per_page: perPage })
    const items = Array.isArray(response.data?.data) ? response.data.data : []
    allItems.push(...items)

    const rawTotalPages = response.data?.meta?.total_pages
    const totalPages = Number(rawTotalPages)

    if (Number.isFinite(totalPages) && totalPages > 0) {
      // Normal path: stop when we've fetched all pages.
      if (page >= totalPages) break
    } else {
      // Fallback: stop when the response contains fewer items than requested.
      if (items.length < perPage) break
    }

    page++
  }

  return allItems
}

