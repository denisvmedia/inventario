import { ArrowLeft, Loader2, RotateCcw } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams } from "react-router-dom"

import {
  RestoreOptionsForm,
  type RestoreOptionsFormValue,
} from "@/components/exports/RestoreOptionsForm"
import { Alert, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useGroupMigrationLock } from "@/features/currency-migration/lock"
import { useCreateRestore, useExport } from "@/features/export/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"

const defaultState: RestoreOptionsFormValue = {
  description: "",
  strategy: "merge_add",
  include_file_data: true,
  dry_run: true,
}

export function ExportRestorePage() {
  const { t } = useTranslation(["exports", "common"])
  const params = useParams()
  const navigate = useNavigate()
  const toast = useAppToast()
  const { currentGroup } = useCurrentGroup()
  const groupReady = !!currentGroup
  const slug = currentGroup?.slug ?? ""
  const exportId = params.id ?? ""
  const migrationLock = useGroupMigrationLock()

  const exportQuery = useExport(exportId, { enabled: groupReady && !!exportId })
  const createRestoreMutation = useCreateRestore()
  const [state, setState] = useState<RestoreOptionsFormValue>(defaultState)

  const exp = exportQuery.data
  const detailHref = `/g/${encodeURIComponent(slug)}/exports/${encodeURIComponent(exportId)}`
  const isPending = createRestoreMutation.isPending
  const isDestructive = state.strategy === "full_replace"

  function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    // Use mutate({ onSuccess }) instead of `await mutateAsync` —
    // navigate() inside an async-callback after `await` was dropping
    // under load on CI (same flake class as the wizard step-3 fix on
    // ExportNewPage). Per-call onSuccess fires reliably.
    createRestoreMutation.mutate(
      {
        exportId,
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
        onSuccess: () => {
          toast.success(
            state.dry_run ? t("exports:restore.successDryRun") : t("exports:restore.success")
          )
          navigate(detailHref)
        },
        onError: (err) => {
          const message = err instanceof Error ? err.message : String(err)
          toast.error(t("exports:errors.restoreCreateFailed", { error: message }))
        },
      }
    )
  }

  if (!groupReady || exportQuery.isLoading) {
    return (
      <div className="flex flex-col gap-4 p-6" data-testid="page-export-restore-loading">
        <Skeleton className="h-8 w-1/2" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (!exp) {
    return (
      <div className="flex flex-col gap-4 p-6" data-testid="page-export-restore-not-found">
        <Alert variant="destructive">
          <AlertTitle>{t("exports:detail.notFound")}</AlertTitle>
        </Alert>
        <Button asChild variant="outline" className="self-start">
          <Link to={`/g/${encodeURIComponent(slug)}/exports`}>{t("exports:list.title")}</Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6 p-6" data-testid="page-export-restore">
      <header className="flex flex-col gap-2">
        <Button asChild variant="link" className="self-start px-0">
          <Link to={detailHref}>
            <ArrowLeft className="mr-1.5 size-4" aria-hidden="true" />
            {t("exports:detail.title")}
          </Link>
        </Button>
        <h1 className="text-2xl font-semibold tracking-tight">{t("exports:restore.title")}</h1>
        <p className="max-w-prose text-sm text-muted-foreground">{t("exports:restore.intro")}</p>
      </header>

      <form
        onSubmit={onSubmit}
        className="flex max-w-2xl flex-col gap-6"
        data-testid="restore-form"
      >
        <RestoreOptionsForm value={state} onChange={setState} disabled={isPending} />

        <div className="flex flex-wrap justify-end gap-2">
          <Button asChild variant="ghost" type="button">
            <Link to={detailHref}>{t("exports:wizard.cancel")}</Link>
          </Button>
          <Button
            type="submit"
            variant={isDestructive && !state.dry_run ? "destructive" : "default"}
            disabled={isPending || !state.description.trim() || migrationLock.locked}
            title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
            aria-disabled={migrationLock.locked || undefined}
            data-testid="restore-submit"
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
        </div>
      </form>
    </div>
  )
}
