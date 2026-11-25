import { useEffect, useMemo, useState } from "react"
import { hashKey, useQueries, type QueryClient, type QueryKey } from "@tanstack/react-query"

// Previous / next within the list a record was opened from.
//
// A record surface (file review, item page, inbox row) gets the originating
// list from its `?from=` context (lib/list-context.ts), turns it into a
// ListPageSource that can fetch any page of that list, and asks this hook
// where the current record sits and which records come before and after it.
//
// Rules:
//
//   - Order and membership are the list's own: the same query, page size and
//     sort the list page uses, fetched through the same query keys, so the
//     list and the record surface share the cache.
//   - Neighbors across a page edge come from the adjacent page: the record
//     before the first one on page 3 is the last one on page 2. The returned
//     neighbor carries the page it is on, which becomes the context's page
//     when the user moves to it, so returning to the list lands there.
//   - The record is looked up on the context's page first, then on the pages
//     on either side. A save that adds or removes one record shifts the others
//     across a page edge by one place, so a record moved to by Next can sit on
//     the page before the one the context names.
//   - When the record is no longer in the list (an edit took it out of the
//     filter), the records after it have moved up one place. Its last known
//     offset is kept, so Next goes to the record that now sits at that offset
//     (the one that followed it) and Previous to the one before. Nothing is
//     skipped and nothing comes up twice.
//   - A record that was never seen in the list (a stale or hand-made link) has
//     no position and no neighbors.
//
// "Save and next" reads `next` before saving and moves there after the save
// succeeds; see the file review page for the pattern.

/** One page of a list, reduced to what navigation needs. */
export interface ListPage {
  /** Record ids on this page, in list order. */
  ids: string[]
  /** Number of records in the whole list. */
  total: number
}

/**
 * Query options for one page of a list. Build it with `listPageQuery` so the
 * fetcher's own data type is checked against `select`.
 */
export interface ListPageQuery {
  queryKey: QueryKey
  queryFn: (context: { signal: AbortSignal }) => Promise<unknown>
  select: (data: unknown) => ListPage
}

/** Types a page query against its fetcher, then erases the data type. */
export function listPageQuery<TData>(query: {
  queryKey: QueryKey
  queryFn: (context: { signal: AbortSignal }) => Promise<TData>
  select: (data: TData) => ListPage
}): ListPageQuery {
  return query as unknown as ListPageQuery
}

/** A list that can be walked: its page size, the page in context and a fetcher per page. */
export interface ListPageSource {
  pageSize: number
  /** The page the list context points at (1-based). */
  page: number
  pageQuery: (page: number) => ListPageQuery
}

/** A record next to the current one, and the list page it is on. */
export interface ListNeighbor {
  id: string
  page: number
}

export interface ListPosition {
  /** Pages the answer still waits for. Empty once it is final. */
  need: number[]
  /** 0-based offset of the record in the list; null when it is not in it. */
  offset: number | null
  total: number | null
  /** The list page holding the record, or where it was; the page to return to. */
  page: number
  previous: ListNeighbor | null
  next: ListNeighbor | null
}

export interface ResolveListPositionInput {
  currentId: string
  /** The page the list context points at. */
  page: number
  pageSize: number
  /** Pages loaded so far. */
  pages: ReadonlyMap<number, ListPage>
  /** The record's last known 0-based offset in this list, if it was ever seen there. */
  anchor: number | null
}

/** Pure core of useListNavigation; see the rules at the top of this file. */
export function resolveListPosition({
  currentId,
  page,
  pageSize,
  pages,
  anchor,
}: ResolveListPositionInput): ListPosition {
  const pending: ListPosition = {
    need: [page],
    offset: null,
    total: null,
    page,
    previous: null,
    next: null,
  }
  const base = pages.get(page)
  if (!base) return pending

  const total = base.total
  const lastPage = Math.max(1, Math.ceil(total / pageSize))
  const candidates = [page, page - 1, page + 1].filter(
    (p) => p >= 1 && (p === page || p <= lastPage)
  )

  let holder: { page: number; index: number } | null = null
  for (const p of candidates) {
    const index = pages.get(p)?.ids.indexOf(currentId) ?? -1
    if (index !== -1) {
      holder = { page: p, index }
      break
    }
  }

  if (!holder) {
    const unloaded = candidates.filter((p) => !pages.has(p))
    if (unloaded.length > 0) return { ...pending, need: unloaded, total }
    return outsideList({ page, pageSize, pages, anchor, total })
  }

  const need: number[] = []
  const ids = pages.get(holder.page)!.ids
  const offset = (holder.page - 1) * pageSize + holder.index

  let previous: ListNeighbor | null = null
  if (holder.index > 0) {
    previous = { id: ids[holder.index - 1], page: holder.page }
  } else if (holder.page > 1) {
    const before = pages.get(holder.page - 1)
    if (!before) need.push(holder.page - 1)
    else if (before.ids.length > 0) {
      previous = { id: before.ids[before.ids.length - 1], page: holder.page - 1 }
    }
  }

  let next: ListNeighbor | null = null
  if (holder.index < ids.length - 1) {
    next = { id: ids[holder.index + 1], page: holder.page }
  } else if (offset + 1 < total) {
    const after = pages.get(holder.page + 1)
    if (!after) need.push(holder.page + 1)
    else if (after.ids.length > 0) next = { id: after.ids[0], page: holder.page + 1 }
  }

  return { need, offset, total, page: holder.page, previous, next }
}

function outsideList({
  page,
  pageSize,
  pages,
  anchor,
  total,
}: {
  page: number
  pageSize: number
  pages: ReadonlyMap<number, ListPage>
  anchor: number | null
  total: number
}): ListPosition {
  if (anchor === null) {
    return { need: [], offset: null, total, page, previous: null, next: null }
  }
  const need: number[] = []
  const at = (offset: number): ListNeighbor | null => {
    if (offset < 0 || offset >= total) return null
    const p = Math.floor(offset / pageSize) + 1
    const data = pages.get(p)
    if (!data) {
      need.push(p)
      return null
    }
    const id = data.ids[offset % pageSize]
    return id ? { id, page: p } : null
  }
  const next = at(anchor)
  const previous = at(anchor - 1)
  const returnOffset = Math.max(0, Math.min(anchor, total - 1))
  return {
    need,
    offset: null,
    total,
    page: Math.floor(returnOffset / pageSize) + 1,
    previous,
    next,
  }
}

/**
 * The record to go to once the record at a 0-based offset has left the list,
 * read from the server; null when there is none.
 *
 * `expected` is the record that followed it before it left. While the list
 * still holds that record on the page of the offset or the page before it
 * (records move up when earlier ones leave the list, here or elsewhere), it
 * is the one: no record after it is skipped and none before it comes back.
 * Otherwise it is the record now at the offset, the one the record after it
 * moved up to; a record that left the list in the meantime is not opened.
 * `skipId`, when it still sits at the offset, is passed over.
 */
export async function fetchRecordAfter(
  queryClient: QueryClient,
  source: ListPageSource,
  offset: number,
  { skipId, expected }: { skipId?: string; expected?: string | null } = {}
): Promise<ListNeighbor | null> {
  const pages = new Map<number, Promise<ListPage>>()
  const readPage = (page: number): Promise<ListPage> => {
    let read = pages.get(page)
    if (!read) {
      const query = source.pageQuery(page)
      read = queryClient
        .fetchQuery({ queryKey: query.queryKey, queryFn: query.queryFn })
        .then(query.select)
      pages.set(page, read)
    }
    return read
  }
  const readAt = async (at: number): Promise<ListNeighbor | null> => {
    const page = Math.floor(at / source.pageSize) + 1
    const data = await readPage(page)
    if (at >= data.total) return null
    const id = data.ids[at % source.pageSize]
    return id ? { id, page } : null
  }
  if (expected) {
    const page = Math.floor(offset / source.pageSize) + 1
    for (const p of page > 1 ? [page, page - 1] : [page]) {
      if ((await readPage(p)).ids.includes(expected)) return { id: expected, page: p }
    }
  }
  const found = await readAt(offset)
  return found && found.id === skipId ? readAt(offset + 1) : found
}

export interface ListNavigation {
  /** A list is being walked (a source was given). */
  active: boolean
  /** The answer still waits for a list page. */
  loading: boolean
  /** A list page could not be loaded. */
  error: boolean
  /** The record is in the list's current results. */
  inList: boolean
  /** 1-based position of the record in the list; null when it is not in it. */
  position: number | null
  total: number | null
  /** The list page holding the record (or where it was); the page to return to. */
  page: number
  previous: ListNeighbor | null
  next: ListNeighbor | null
}

const INACTIVE: ListNavigation = {
  active: false,
  loading: false,
  error: false,
  inList: false,
  position: null,
  total: null,
  page: 1,
  previous: null,
  next: null,
}

interface Tracked {
  key: string
  pages: number[]
}

interface Anchor {
  key: string
  id: string
  offset: number
}

/**
 * Where `currentId` sits in the list `source` describes, and its neighbors.
 * Pass a null source when the record was not opened from a list it can walk.
 */
export function useListNavigation({
  source,
  currentId,
}: {
  source: ListPageSource | null
  currentId: string | undefined
}): ListNavigation {
  // Identifies the list (its filters and sort) independently of the page.
  const listKey = source ? hashKey(source.pageQuery(1).queryKey) : ""
  const trackKey = source ? `${listKey}|${source.page}|${currentId ?? ""}` : ""

  const [tracked, setTracked] = useState<Tracked>({ key: "", pages: [] })
  const [anchor, setAnchor] = useState<Anchor | null>(null)

  const requested = useMemo(
    () =>
      source ? [...new Set([source.page, ...(tracked.key === trackKey ? tracked.pages : [])])] : [],
    [source, tracked, trackKey]
  )

  const results = useQueries({
    queries: requested.map((p) => {
      const q = source!.pageQuery(p)
      return {
        queryKey: q.queryKey,
        queryFn: q.queryFn,
        select: q.select,
        enabled: !!source && !!currentId,
      }
    }),
  })

  const loaded = new Map<number, ListPage>()
  requested.forEach((p, i) => {
    const data = results[i]?.data
    if (data) loaded.set(p, data)
  })
  const error = results.some((r) => r.isError)

  const resolution =
    source && currentId
      ? resolveListPosition({
          currentId,
          page: source.page,
          pageSize: source.pageSize,
          pages: loaded,
          anchor:
            anchor && anchor.key === listKey && anchor.id === currentId ? anchor.offset : null,
        })
      : null

  const missing = resolution ? resolution.need.filter((p) => !requested.includes(p)) : []
  const missingKey = missing.join(",")
  useEffect(() => {
    if (!missingKey) return
    const add = missingKey.split(",").map(Number)
    // eslint-disable-next-line react-hooks/set-state-in-effect -- the pages to fetch follow from the pages already fetched; this widens the query set until the answer no longer needs another page.
    setTracked((prev) => ({
      key: trackKey,
      pages: [...new Set([...(prev.key === trackKey ? prev.pages : []), ...add])],
    }))
  }, [missingKey, trackKey])

  const offset = resolution?.offset ?? null
  useEffect(() => {
    if (offset === null || !currentId) return
    // eslint-disable-next-line react-hooks/set-state-in-effect -- remembers where the record was last seen, for when an edit takes it out of the list.
    setAnchor((prev) =>
      prev && prev.key === listKey && prev.id === currentId && prev.offset === offset
        ? prev
        : { key: listKey, id: currentId, offset }
    )
  }, [offset, currentId, listKey])

  if (!source || !resolution) return INACTIVE
  return {
    active: true,
    loading: resolution.need.length > 0 && !error,
    error,
    inList: resolution.offset !== null,
    position: resolution.offset === null ? null : resolution.offset + 1,
    total: resolution.total,
    page: resolution.page,
    previous: resolution.previous,
    next: resolution.next,
  }
}
