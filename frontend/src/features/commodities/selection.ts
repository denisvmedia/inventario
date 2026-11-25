import { useCallback, useEffect, useMemo, useState } from "react"

import type { CommoditySelection } from "./api"

// The items the list query matched when the user chose "all matching": how
// many, and the list's selection digest of their ids.
interface Matching {
  total: number
  digest: string
}

// SelectionVersion names one state of the selection: every change makes a
// new one, so a bulk request can tell whether the selection it was sent for
// is still the current one (settle).
export type SelectionVersion = symbol

interface SelectionState {
  ids: ReadonlySet<string>
  // Every item the list query matches, on every page, as it was then.
  matching: Matching | null
  version: SelectionVersion
}

function next(
  ids: Iterable<string>,
  matching: Matching | null,
  version: SelectionVersion = Symbol("selection")
): SelectionState {
  return { ids: new Set(ids), matching, version }
}

export interface CommoditySelectionControls {
  /** True after "Select all N matching". */
  allMatching: boolean
  /** The ids picked one by one or a page at a time. Empty when allMatching. */
  ids: ReadonlySet<string>
  /** The rows of a page to render as checked. */
  checkedOn: (pageIds: string[]) => ReadonlySet<string>
  /** How many items the selection holds. */
  count: number
  /** Changes with every change of the selection; see settle. */
  version: SelectionVersion
  toggle: (id: string, pageIds: string[]) => void
  togglePage: (pageIds: string[]) => void
  /** Selects the total items the query matches now, which digest names. */
  selectAllMatching: (total: number, digest: string) => void
  /**
   * Applies the outcome of a bulk request sent for the selection at version:
   * keeps the given ids selected, or clears the selection when there are
   * none. Does nothing when the selection changed since, so a request that
   * ends late never replaces or clears one the user made in the meantime.
   * When it applies, the new selection gets the version `as`, if given, so
   * the caller can later send a request for exactly that selection (a retry
   * of the failed items it kept).
   */
  settle: (version: SelectionVersion, keep: string[], as?: SelectionVersion) => void
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
//
// Unchecking one row of "all matching" turns the selection into the ids of
// the page's rows minus that one. Every row of the page shows as checked in
// "all matching" (checkedOn), so the result is what the user sees checked,
// whatever a refetch put on the page; the bar's count changes with it, and
// a request for ids needs no digest, since the user sees each item.
export function useCommoditySelection(
  queryKey: string,
  pageKey: string
): CommoditySelectionControls {
  const [state, setState] = useState<SelectionState>(() => next([], null))

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- a new query starts a new selection
    setState(next([], null))
  }, [queryKey])
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- ids picked on a page do not outlive it
    setState((s) => (s.matching ? s : next([], null)))
  }, [pageKey])

  const toggle = useCallback((id: string, pageIds: string[]) => {
    setState((s) => {
      // Unchecking a row of "all matching" leaves the rest of the page.
      const ids = new Set(s.matching ? pageIds : s.ids)
      if (ids.has(id)) ids.delete(id)
      else ids.add(id)
      return next(ids, null)
    })
  }, [])

  const togglePage = useCallback((pageIds: string[]) => {
    setState((s) => {
      if (s.matching) return next([], null)
      const all = pageIds.length > 0 && pageIds.every((id) => s.ids.has(id))
      const ids = new Set(s.ids)
      for (const id of pageIds) {
        if (all) ids.delete(id)
        else ids.add(id)
      }
      return next(ids, null)
    })
  }, [])

  const selectAllMatching = useCallback(
    (total: number, digest: string) => setState(next([], { total, digest })),
    []
  )
  const settle = useCallback(
    (version: SelectionVersion, keep: string[], as?: SelectionVersion) =>
      setState((s) => (s.version === version ? next(keep, null, as) : s)),
    []
  )
  const clear = useCallback(() => setState(next([], null)), [])

  return useMemo(
    () => ({
      allMatching: state.matching !== null,
      ids: state.ids,
      checkedOn: (pageIds) => (state.matching ? new Set(pageIds) : state.ids),
      count: state.matching ? state.matching.total : state.ids.size,
      version: state.version,
      toggle,
      togglePage,
      selectAllMatching,
      settle,
      clear,
      toRequest: (query) =>
        state.matching
          ? { query, expectedCount: state.matching.total, expectedDigest: state.matching.digest }
          : { ids: [...state.ids] },
    }),
    [state, toggle, togglePage, selectAllMatching, settle, clear]
  )
}
