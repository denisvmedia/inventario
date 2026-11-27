import { createContext, useCallback, useContext, useEffect } from "react"

import { useConfirm, type ConfirmOptions } from "@/hooks/useConfirm"

// After Mark as reviewed saves, the item page opens the next item unless
// something open on the page holds it on this one (CommodityDetailPage,
// markReviewedAndNext). The tabs keep their dialogs in their own state, out of
// the page's reach, so each one holds the item through these hooks while it is
// open. Outside the item page there is nothing to hold and they do nothing.

/** Holds the item until the returned release is called. */
export type HoldItem = () => () => void

export const ItemHoldContext = createContext<HoldItem | null>(null)

/** Holds the item page on its item while `open` is true. */
export function useHoldItem(open: boolean): void {
  const hold = useContext(ItemHoldContext)
  useEffect(() => {
    if (!open || !hold) return
    return hold()
  }, [open, hold])
}

/** useConfirm that holds the item page on its item while the confirm is open. */
export function useHoldingConfirm(): (options: ConfirmOptions) => Promise<boolean> {
  const confirm = useConfirm()
  const hold = useContext(ItemHoldContext)
  return useCallback(
    async (options: ConfirmOptions) => {
      const release = hold?.()
      try {
        return await confirm(options)
      } finally {
        release?.()
      }
    },
    [confirm, hold]
  )
}
