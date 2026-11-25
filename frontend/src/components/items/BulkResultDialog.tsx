import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { CommodityBulkResult } from "@/features/commodities/api"

export interface BulkResultDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  result: CommodityBulkResult | null
  /** Links a failed item to its page; undefined leaves the names as text. */
  itemHref?: (id: string) => string | undefined
  /** Runs the action again on the failed items. */
  onRetry: () => void
  isRetrying?: boolean
}

// Reports a bulk action some items failed: how many changed, how many
// already had the change, and each failed item with the reason. Retry runs
// the action again on the failed items only.
export function BulkResultDialog({
  open,
  onOpenChange,
  result,
  itemHref,
  onRetry,
  isRetrying = false,
}: BulkResultDialogProps) {
  const { t } = useTranslation()
  if (!result) return null

  function reason(f: CommodityBulkResult["failed"][number]): string {
    if (f.code === "not_found") return t("commodities:bulk.result.notFound")
    if (f.code === "invalid" && f.error) return f.error
    return t("commodities:bulk.result.failed")
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg" data-testid="bulk-result-dialog">
        <DialogHeader>
          <DialogTitle>{t("commodities:bulk.result.title")}</DialogTitle>
          <DialogDescription>
            {[
              t("commodities:bulk.result.succeeded", { count: result.succeeded.length }),
              result.skipped.length > 0
                ? t("commodities:bulk.result.skipped", { count: result.skipped.length })
                : null,
              t("commodities:bulk.result.failedCount", { count: result.failed.length }),
            ]
              .filter(Boolean)
              .join(" · ")}
          </DialogDescription>
        </DialogHeader>
        <ul
          className="flex max-h-72 flex-col divide-y overflow-y-auto rounded-md border text-sm"
          data-testid="bulk-result-failed"
        >
          {result.failed.map((f) => {
            const name = f.name || t("commodities:bulk.result.unnamed")
            const href = f.code === "not_found" ? undefined : itemHref?.(f.id)
            return (
              <li key={f.id} className="flex flex-col gap-0.5 px-3 py-2">
                {href ? (
                  <Link to={href} className="font-medium hover:underline">
                    {name}
                  </Link>
                ) : (
                  <span className="font-medium">{name}</span>
                )}
                <span className="text-muted-foreground">{reason(f)}</span>
              </li>
            )
          })}
        </ul>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t("common:actions.close")}
          </Button>
          <Button
            type="button"
            onClick={onRetry}
            disabled={isRetrying}
            data-testid="bulk-result-retry"
          >
            {t("commodities:bulk.result.retry")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
