/**
 * Fetches all pages of a paginated resource by iterating through pages until exhausted.
 * Use this for lookup/reference data that must be complete (e.g., area/location name maps).
 *
 * @param fetchFn - Function accepting { page, per_page } params and returning a paginated response
 * @param perPage - Items per page to request (capped by backend, typically 100 max)
 * @returns Flat array of all items across all pages
 */
export async function fetchAll<T>(
  fetchFn: (params: { page: number; per_page: number }) => Promise<{ data: { data: T[]; meta: { total_pages: number } } }>,
  perPage = 100,
): Promise<T[]> {
  let page = 1
  const allItems: T[] = []

  while (true) {
    const response = await fetchFn({ page, per_page: perPage })
    const items = response.data.data
    allItems.push(...items)

    const totalPages = response.data.meta.total_pages
    if (totalPages === 0 || page >= totalPages) {
      break
    }
    page++
  }

  return allItems
}

