import { useCallback, useEffect, useMemo, useState } from "react"

import type { CommoditySelection } from "./api"

// The items the list query matched when the user chose "all matching": how
// many, and the list's selection digest of their ids.
interface Matching {
  total: number
  digest: string
}

interface SelectionState {
  ids: ReadonlySet<string>
  // Every item the list query matches, on every page, as it was then.
  matching: Matching | null
}

const EMPTY: SelectionState = { ids: new Set(), matching: null }

export interface CommoditySelectionControls {
  /** True after "Select all N matching". */
  allMatching: boolean
  /** The ids picked one by one or a page at a time. Empty when allMatching. */
  ids: ReadonlySet<string>
  /** The rows of a page to render as checked. */
  checkedOn: (pageIds: string[]) => ReadonlySet<string>
  /** How many items the selection holds. */
  count: number
  toggle: (id: string, pageIds: string[]) => void
  togglePage: (pageIds: string[]) => void
  /** Selects the total items the query matches now, which digest names. */
  selectAllMatching: (total: number, digest: string) => void
  replace: (ids: string[]) => void
  clear: () => void
  /** The selection as a bulk request names it. */
  toRequest: (query: string) => CommoditySelection
}

// useCommoditySelection holds the Items list selection: ids picked one by
// one or a page at a time, or every item the query matches. A new query
// clears it, so a selection never carries over to items the user did not
// pick. A new page (pageKey: the page number and page size) clears ids
// picked on the page; it keeps "all matching", which covers every page.
//
// "All matching" is not a list of ids: it keeps the total and the selection
// digest the list gave when the user chose it, and a bulk request sends the
// query with both (toRequest). The server resolves the query and refuses
// the request when it selects other items by then. Keeping them from that
// moment, rather than from the latest refetch, holds the action to the items
// the user chose, not ones a background refresh swapped in.
export function useCommoditySelection(
  queryKey: string,
  pageKey: string
): CommoditySelectionControls {
  const [state, setState] = useState<SelectionState>(EMPTY)

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- a new query starts a new selection
    setState(EMPTY)
  }, [queryKey])
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- ids picked on a page do not outlive it
    setState((s) => (s.matching ? s : EMPTY))
  }, [pageKey])

  const toggle = useCallback((id: string, pageIds: string[]) => {
    setState((s) => {
      // Unchecking a row of "all matching" leaves the rest of the page.
      const ids = new Set(s.matching ? pageIds : s.ids)
      if (ids.has(id)) ids.delete(id)
      else ids.add(id)
      return { ids, matching: null }
    })
  }, [])

  const togglePage = useCallback((pageIds: string[]) => {
    setState((s) => {
      if (s.matching) return EMPTY
      const all = pageIds.length > 0 && pageIds.every((id) => s.ids.has(id))
      const ids = new Set(s.ids)
      for (const id of pageIds) {
        if (all) ids.delete(id)
        else ids.add(id)
      }
      return { ids, matching: null }
    })
  }, [])

  const selectAllMatching = useCallback(
    (total: number, digest: string) => setState({ ids: new Set(), matching: { total, digest } }),
    []
  )
  const replace = useCallback(
    (ids: string[]) => setState({ ids: new Set(ids), matching: null }),
    []
  )
  const clear = useCallback(() => setState(EMPTY), [])

  return useMemo(
    () => ({
      allMatching: state.matching !== null,
      ids: state.ids,
      checkedOn: (pageIds) => (state.matching ? new Set(pageIds) : state.ids),
      count: state.matching ? state.matching.total : state.ids.size,
      toggle,
      togglePage,
      selectAllMatching,
      replace,
      clear,
      toRequest: (query) =>
        state.matching
          ? { query, expectedCount: state.matching.total, expectedDigest: state.matching.digest }
          : { ids: [...state.ids] },
    }),
    [state, toggle, togglePage, selectAllMatching, replace, clear]
  )
}
