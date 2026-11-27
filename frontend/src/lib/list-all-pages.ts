// The largest page the list endpoints honor. parsePagination on the server
// accepts 1-100 and answers any larger per_page with its default of 50, so
// asking for more than this returns less.
export const MAX_PAGE_SIZE = 100

// Stops a loop against an endpoint that never returns a short page:
// 100 pages of 100 rows.
const MAX_PAGES = 100

export interface ListPage<T> {
  rows: T[]
  // The number of rows the whole list holds, when the endpoint reports it.
  total?: number
}

// Reads every page of a paged list endpoint, MAX_PAGE_SIZE rows at a time,
// and returns the rows in the order the server sent them. It stops at the
// first short page or once it has as many rows as the reported total.
export async function listAllPages<T>(
  fetchPage: (page: number, perPage: number) => Promise<ListPage<T>>
): Promise<T[]> {
  const out: T[] = []
  for (let page = 1; page <= MAX_PAGES; page++) {
    const { rows, total } = await fetchPage(page, MAX_PAGE_SIZE)
    out.push(...rows)
    if (rows.length < MAX_PAGE_SIZE) break
    if (total !== undefined && out.length >= total) break
  }
  return out
}
