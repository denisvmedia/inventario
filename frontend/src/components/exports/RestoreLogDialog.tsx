import { Eye, Loader2, RotateCcw } from "lucide-react"
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
import { ScrollArea } from "@/components/ui/scroll-area"
import { useRestore } from "@/features/export/hooks"
import type { RestoreStep } from "@/features/export/api"
import { cn } from "@/lib/utils"

export interface RestoreLogDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  exportId: string
  restoreId: string
  dryRun?: boolean
}

const RESULT_EMOJI: Record<string, string> = {
  success: "✅",
  error: "❌",
  skipped: "⏭️",
  in_progress: "🔄",
  todo: "📝",
}

function emojiFor(step: RestoreStep): string {
  return RESULT_EMOJI[step.result ?? "todo"] ?? "📝"
}

function isError(step: RestoreStep): boolean {
  return step.result === "error"
}

// Per-step restore log. Renders the monospaced scroll log from the mock's
// RestoreLogsDialog (design-mocks/src/views/BackupView.tsx lines 637-664)
// against the BE `models.RestoreStep` rows already loaded by
// GET /exports/:id/restores/:restoreId (see go/apiserver/export_restores.go).
// useRestore polls while the restore is non-terminal so the log streams in
// as the worker processes records.
export function RestoreLogDialog({
  open,
  onOpenChange,
  exportId,
  restoreId,
  dryRun,
}: RestoreLogDialogProps) {
  const { t } = useTranslation(["exports"])
  const restoreQuery = useRestore(exportId, restoreId, { enabled: open && !!restoreId })
  const restore = restoreQuery.data
  const steps = restore?.steps ?? []
  const isLoading = restoreQuery.isLoading
  const isPreview = dryRun ?? restore?.options?.dry_run ?? false

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg" data-testid="restore-log-dialog">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {isPreview ? (
              <Eye className="size-4" aria-hidden="true" />
            ) : (
              <RotateCcw className="size-4" aria-hidden="true" />
            )}
            {isPreview
              ? t("exports:restore.log.titlePreview")
              : t("exports:restore.log.titleComplete")}
          </DialogTitle>
          <DialogDescription>
            {isPreview
              ? t("exports:restore.log.descriptionPreview")
              : t("exports:restore.log.descriptionComplete")}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className="h-64 rounded-lg border bg-muted/30 p-4">
          {isLoading ? (
            <p className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
              <Loader2 className="size-3 animate-spin" aria-hidden="true" />
              {t("exports:restore.log.loading")}
            </p>
          ) : steps.length === 0 ? (
            <p className="font-mono text-xs text-muted-foreground" data-testid="restore-log-empty">
              {t("exports:restore.log.empty")}
            </p>
          ) : (
            <ul className="flex flex-col gap-1.5 font-mono text-xs" data-testid="restore-log-list">
              {steps.map((step) => {
                const reason = step.reason?.trim()
                const errored = isError(step)
                return (
                  <li
                    key={step.id ?? `${step.name}-${step.created_date}`}
                    className={cn("leading-relaxed", errored && "text-destructive")}
                    data-testid={`restore-log-step-${step.result ?? "todo"}`}
                  >
                    <span aria-hidden="true">{emojiFor(step)}</span> <span>{step.name}</span>
                    {reason && (
                      <>
                        <span aria-hidden="true"> — </span>
                        <span>{reason}</span>
                      </>
                    )}
                  </li>
                )
              })}
            </ul>
          )}
        </ScrollArea>

        <DialogFooter>
          <Button type="button" onClick={() => onOpenChange(false)}>
            {t("exports:restore.log.done")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
