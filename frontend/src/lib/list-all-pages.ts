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
// and returns the rows in the order the server sent them. With a reported
// total it reads until it holds that many rows, so a server that caps the
// page below MAX_PAGE_SIZE is still read to the end; without one it stops at
// the first short page. A page that adds no new row ends the walk too.
//
// Offset paging is not a snapshot: a row created, deleted or renamed between
// two page reads moves the rows after it, so a row can come back on the next
// page or be missed. A row whose id was already read is dropped, so the list
// holds each id once.
export async function listAllPages<T extends { id?: string }>(
  fetchPage: (page: number, perPage: number) => Promise<ListPage<T>>
): Promise<T[]> {
  const out: T[] = []
  const seen = new Set<string>()
  for (let page = 1; page <= MAX_PAGES; page++) {
    const { rows, total } = await fetchPage(page, MAX_PAGE_SIZE)
    let added = 0
    for (const row of rows) {
      if (row.id !== undefined) {
        if (seen.has(row.id)) continue
        seen.add(row.id)
      }
      out.push(row)
      added++
    }
    if (added === 0) break
    if (total === undefined ? rows.length < MAX_PAGE_SIZE : out.length >= total) break
  }
  return out
}
