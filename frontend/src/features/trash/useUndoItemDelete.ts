import { useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"

import { useAppToast } from "@/hooks/useAppToast"

import { restoreCommodities } from "./api"

// useUndoItemDelete returns the "Undo" action for the toast shown after items
// are deleted. Undo restores them from Trash. It calls the API directly
// rather than through a mutation hook: the item page navigates away right
// after a delete, and the toast outlives the component that showed it.
export function useUndoItemDelete() {
  const { t } = useTranslation()
  const toast = useAppToast()
  const qc = useQueryClient()
  return (ids: string[]) => ({
    label: t("trash:undo"),
    onClick: () => {
      restoreCommodities(ids).then(
        (result) => {
          void qc.invalidateQueries()
          if (result.failed.length > 0) {
            toast.warning(
              t("trash:toast.bulkPartial", {
                succeeded: result.succeeded.length,
                failed: result.failed.length,
              })
            )
            return
          }
          toast.success(t("trash:toast.itemsRestored", { count: result.succeeded.length }))
        },
        (err: unknown) => toast.error(err instanceof Error ? err.message : String(err))
      )
    },
  })
}
