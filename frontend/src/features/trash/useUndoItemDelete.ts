import { useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"

import { useAppToast } from "@/hooks/useAppToast"

import { restoreCommodities } from "./api"

// useUndoItemDelete returns the "Undo" action for the toast shown after items
// are deleted. Undo restores them from Trash. It calls the API directly
// rather than through a mutation hook: the item page navigates away right
// after a delete, and the toast outlives the component that showed it.
//
// maybeDeleted are items whose delete request failed after it may have
// reached the server (an unknown outcome): they may be in Trash or not, so
// Undo restores them too. A restore the server refuses because one of them
// is not in Trash (not_found) is not counted as a failure, since it means
// the item never left; any other failure for one of them is.
export function useUndoItemDelete() {
  const { t } = useTranslation()
  const toast = useAppToast()
  const qc = useQueryClient()
  return (ids: string[], maybeDeleted: string[] = []) => ({
    label: t("trash:undo"),
    onClick: () => {
      const deleted = new Set(ids)
      restoreCommodities([...ids, ...maybeDeleted]).then(
        (result) => {
          void qc.invalidateQueries()
          const failed = result.failed.filter(
            (f) => deleted.has(f.id) || f.code !== "not_found"
          ).length
          const options =
            result.unknown.length > 0
              ? { description: t("trash:toast.restoreUnknown", { count: result.unknown.length }) }
              : undefined
          // As the Trash page does, a restore that got no answer for some
          // items is a warning: they may still be in Trash.
          if (failed > 0 || result.unknown.length > 0) {
            toast.warning(
              t("trash:toast.bulkPartial", { succeeded: result.succeeded.length, failed }),
              options
            )
            return
          }
          toast.success(t("trash:toast.itemsRestored", { count: result.succeeded.length }), options)
        },
        (err: unknown) => toast.error(err instanceof Error ? err.message : String(err))
      )
    },
  })
}
