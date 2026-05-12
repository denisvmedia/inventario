import { Loader2, RotateCcw } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import {
  RestoreOptionsForm,
  type RestoreOptionsFormValue,
} from "@/components/exports/RestoreOptionsForm"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useGroupMigrationLock } from "@/features/currency-migration/lock"
import { type Export } from "@/features/export/api"
import { useCreateRestore } from "@/features/export/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { formatDateTime } from "@/lib/intl"

export interface RestoreDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  export: Export | null
  onCompleted?: (restoreId: string, dryRun: boolean) => void
}

const defaultState: RestoreOptionsFormValue = {
  description: "",
  strategy: "merge_add",
  include_file_data: true,
  dry_run: true,
}

// In-context restore dialog. Mirrors design-mocks/src/views/BackupView.tsx
// RestoreDialog. The standalone /exports/:id/restore page is preserved for
// shareable URLs and uses the same RestoreOptionsForm body so the two
// surfaces stay visually aligned.
//
// Wrapper-only: form state lives inside `RestoreDialogContent` so a fresh
// state is acquired every time the dialog opens (the inner component is
// keyed on `exp.id` and only mounts while open). Avoids the React-19
// `react-hooks/set-state-in-effect` smell of resetting state in a useEffect.
export function RestoreDialog({
  open,
  onOpenChange,
  export: exp,
  onCompleted,
}: RestoreDialogProps) {
  return (
    <Dialog open={open && !!exp} onOpenChange={onOpenChange}>
      {exp && open && (
        <RestoreDialogContent
          key={exp.id}
          export={exp}
          onOpenChange={onOpenChange}
          onCompleted={onCompleted}
        />
      )}
    </Dialog>
  )
}

interface RestoreDialogContentProps {
  export: Export
  onOpenChange: (open: boolean) => void
  onCompleted?: (restoreId: string, dryRun: boolean) => void
}

function RestoreDialogContent({
  export: exp,
  onOpenChange,
  onCompleted,
}: RestoreDialogContentProps) {
  const { t } = useTranslation(["exports", "errors"])
  const toast = useAppToast()
  const migrationLock = useGroupMigrationLock()
  const createRestoreMutation = useCreateRestore()
  const [state, setState] = useState<RestoreOptionsFormValue>(defaultState)

  const isPending = createRestoreMutation.isPending
  const isDestructive = state.strategy === "full_replace"
  const scopeLabel =
    exp.type === "selected_items"
      ? t("exports:detail.scopeSelectedItems", { count: exp.selected_items?.length ?? 0 })
      : t(`exports:scope.${exp.type ?? "full_database"}`, {
          defaultValue: t("exports:scope.full_database"),
        })
  const dateLabel = exp.created_date ? formatDateTime(exp.created_date) : ""
  const description = dateLabel ? `${scopeLabel} — ${dateLabel}` : scopeLabel
  const canSubmit = !isPending && !!state.description.trim() && !migrationLock.locked

  function onSubmit() {
    createRestoreMutation.mutate(
      {
        exportId: exp.id,
        req: {
          description: state.description,
          options: {
            strategy: state.strategy,
            include_file_data: state.include_file_data,
            dry_run: state.dry_run,
          },
        },
      },
      {
        onSuccess: (restore) => {
          toast.success(
            state.dry_run ? t("exports:restore.successDryRun") : t("exports:restore.success")
          )
          onOpenChange(false)
          onCompleted?.(restore.id, state.dry_run)
        },
        onError: (err) => {
          const message = err instanceof Error ? err.message : String(err)
          toast.error(t("exports:errors.restoreCreateFailed", { error: message }))
        },
      }
    )
  }

  return (
    <DialogContent className="sm:max-w-lg" data-testid="restore-dialog">
      <DialogHeader>
        <DialogTitle>{t("exports:restore.dialog.title")}</DialogTitle>
        <DialogDescription>{description}</DialogDescription>
      </DialogHeader>

      <RestoreOptionsForm value={state} onChange={setState} disabled={isPending} />

      <DialogFooter>
        <Button
          variant="outline"
          type="button"
          onClick={() => onOpenChange(false)}
          disabled={isPending}
        >
          {t("exports:wizard.cancel")}
        </Button>
        <Button
          type="button"
          variant={isDestructive && !state.dry_run ? "destructive" : "default"}
          onClick={onSubmit}
          disabled={!canSubmit}
          title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
          aria-disabled={migrationLock.locked || undefined}
          data-testid="restore-dialog-submit"
          className="gap-2"
        >
          {isPending ? (
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
          ) : (
            <RotateCcw className="size-4" aria-hidden="true" />
          )}
          {isPending
            ? t("exports:restore.submitting")
            : state.dry_run
              ? t("exports:restore.dialog.submitDryRun")
              : t("exports:restore.dialog.submit")}
        </Button>
      </DialogFooter>
    </DialogContent>
  )
}
