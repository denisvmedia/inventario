import { useCallback, useEffect, useMemo, useState } from "react"

import type { CommoditySelection } from "./api"

interface SelectionState {
  ids: ReadonlySet<string>
  // Every item the list query matches, on every page.
  allMatching: boolean
}

const EMPTY: SelectionState = { ids: new Set(), allMatching: false }

export interface CommoditySelectionControls {
  /** True after "Select all N matching". */
  allMatching: boolean
  /** The ids picked one by one or a page at a time. Empty when allMatching. */
  ids: ReadonlySet<string>
  /** The rows of a page to render as checked. */
  checkedOn: (pageIds: string[]) => ReadonlySet<string>
  /** How many items the selection holds, given the query's total. */
  count: (total: number) => number
  toggle: (id: string, pageIds: string[]) => void
  togglePage: (pageIds: string[]) => void
  selectAllMatching: () => void
  replace: (ids: string[]) => void
  clear: () => void
  /** The selection as a bulk request names it. */
  toRequest: (query: string, total: number) => CommoditySelection
}

// useCommoditySelection holds the Items list selection: ids picked one by
// one or a page at a time, or every item the query matches. A new query
// clears it, so a selection never carries over to items the user did not
// pick. A new page (pageKey: the page number and page size) clears ids
// picked on the page; it keeps "all matching", which covers every page.
//
// "All matching" is not a list of ids: a bulk request sends the query and
// the total the user saw (toRequest), and the server resolves it.
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
    setState((s) => (s.allMatching ? s : EMPTY))
  }, [pageKey])

  const toggle = useCallback((id: string, pageIds: string[]) => {
    setState((s) => {
      // Unchecking a row of "all matching" leaves the rest of the page.
      const ids = new Set(s.allMatching ? pageIds : s.ids)
      if (ids.has(id)) ids.delete(id)
      else ids.add(id)
      return { ids, allMatching: false }
    })
  }, [])

  const togglePage = useCallback((pageIds: string[]) => {
    setState((s) => {
      if (s.allMatching) return EMPTY
      const all = pageIds.length > 0 && pageIds.every((id) => s.ids.has(id))
      const ids = new Set(s.ids)
      for (const id of pageIds) {
        if (all) ids.delete(id)
        else ids.add(id)
      }
      return { ids, allMatching: false }
    })
  }, [])

  const selectAllMatching = useCallback(() => setState({ ids: new Set(), allMatching: true }), [])
  const replace = useCallback(
    (ids: string[]) => setState({ ids: new Set(ids), allMatching: false }),
    []
  )
  const clear = useCallback(() => setState(EMPTY), [])

  return useMemo(
    () => ({
      allMatching: state.allMatching,
      ids: state.ids,
      checkedOn: (pageIds) => (state.allMatching ? new Set(pageIds) : state.ids),
      count: (total) => (state.allMatching ? total : state.ids.size),
      toggle,
      togglePage,
      selectAllMatching,
      replace,
      clear,
      toRequest: (query, total) =>
        state.allMatching ? { query, expectedCount: total } : { ids: [...state.ids] },
    }),
    [state, toggle, togglePage, selectAllMatching, replace, clear]
  )
}
