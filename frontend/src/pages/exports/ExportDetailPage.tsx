import { ArrowLeft, Download, RotateCcw, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams } from "react-router-dom"

import { ExportStatusBadge } from "@/components/exports/ExportStatusBadge"
import { RestoreHistoryList } from "@/components/exports/RestoreHistoryList"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { type Export, isExportTerminal, useExportDownloadHref } from "@/features/export/api"
import { useDeleteExport, useExport, useExportRestores } from "@/features/export/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatBytes, formatDateTime } from "@/lib/intl"

export function ExportDetailPage() {
  const { t } = useTranslation(["exports", "common"])
  const params = useParams()
  const navigate = useNavigate()
  const toast = useAppToast()
  const confirm = useConfirm()
  const { currentGroup } = useCurrentGroup()
  const groupReady = !!currentGroup
  const slug = currentGroup?.slug ?? ""
  const exportId = params.id

  const exportQuery = useExport(exportId, { enabled: groupReady && !!exportId })
  const restoresQuery = useExportRestores(exportId, { enabled: groupReady && !!exportId })
  const deleteMutation = useDeleteExport()
  const downloadHref = useExportDownloadHref(exportId, slug)

  const exp = exportQuery.data
  const isCompleted = exp?.status === "completed"
  const isTerminal = isExportTerminal(exp?.status)
  const isDeleted = !!exp?.deleted_at

  async function onDelete() {
    if (!exp) return
    const ok = await confirm({
      title: t("exports:removal.confirmTitle"),
      description: t("exports:removal.confirmDescription"),
      confirmLabel: t("exports:removal.confirmAction"),
      cancelLabel: t("exports:removal.confirmCancel"),
      destructive: true,
    })
    if (!ok) return
    try {
      await deleteMutation.mutateAsync(exp.id)
      toast.success(t("exports:removal.success"))
      navigate(`/g/${encodeURIComponent(slug)}/exports`)
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      toast.error(t("exports:errors.deleteFailed", { error: message }))
    }
  }

  if (!groupReady || exportQuery.isLoading) {
    return (
      <div className="flex flex-col gap-4 p-6" data-testid="page-export-detail-loading">
        <Skeleton className="h-8 w-1/2" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    )
  }

  if (exportQuery.isError || !exp) {
    return (
      <div className="flex flex-col gap-4 p-6" data-testid="page-export-detail-not-found">
        <Alert variant="destructive">
          <AlertTitle>{t("exports:detail.notFound")}</AlertTitle>
          {exportQuery.error instanceof Error && (
            <AlertDescription>{exportQuery.error.message}</AlertDescription>
          )}
        </Alert>
        <Button asChild variant="outline" className="self-start">
          <Link to={`/g/${encodeURIComponent(slug)}/exports`}>
            <ArrowLeft className="mr-1.5 size-4" aria-hidden="true" />
            {t("exports:list.title")}
          </Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6 p-6" data-testid="page-export-detail">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-2">
          <Button asChild variant="link" className="self-start px-0">
            <Link to={`/g/${encodeURIComponent(slug)}/exports`}>
              <ArrowLeft className="mr-1.5 size-4" aria-hidden="true" />
              {t("exports:list.title")}
            </Link>
          </Button>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-semibold tracking-tight">{t("exports:detail.title")}</h1>
            {exp.status && <ExportStatusBadge status={exp.status} />}
            {exp.imported && <Badge variant="secondary">{t("exports:list.imported")}</Badge>}
            {isDeleted && <Badge variant="destructive">{t("exports:list.deletedBadge")}</Badge>}
          </div>
          <p className="max-w-prose text-sm text-muted-foreground">
            {exp.description?.trim() || t("exports:detail.noDescription")}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            asChild
            variant="outline"
            disabled={!isTerminal || !isCompleted || isDeleted || !downloadHref}
            aria-disabled={!isTerminal || !isCompleted || isDeleted || !downloadHref}
          >
            <a href={downloadHref ?? "#"} data-testid="export-detail-download">
              <Download className="mr-1.5 size-4" aria-hidden="true" />
              {t("exports:actions.download")}
            </a>
          </Button>
          <Button
            asChild
            disabled={!isCompleted || isDeleted}
            aria-disabled={!isCompleted || isDeleted}
          >
            <Link
              to={`/g/${encodeURIComponent(slug)}/exports/${encodeURIComponent(exp.id)}/restore`}
              data-testid="export-detail-restore"
            >
              <RotateCcw className="mr-1.5 size-4" aria-hidden="true" />
              {t("exports:actions.restore")}
            </Link>
          </Button>
          <Button
            variant="ghost"
            type="button"
            onClick={onDelete}
            disabled={isDeleted}
            data-testid="export-detail-delete"
            aria-label={t("exports:actions.delete")}
          >
            <Trash2 className="size-4" aria-hidden="true" />
          </Button>
        </div>
      </header>

      {exp.imported && (
        <Alert data-testid="export-detail-imported-banner">
          <AlertDescription>{t("exports:detail.imported")}</AlertDescription>
        </Alert>
      )}
      {exp.status === "failed" && exp.error_message && (
        <Alert variant="destructive">
          <AlertTitle>{t("exports:detail.errorMessage")}</AlertTitle>
          <AlertDescription>{exp.error_message}</AlertDescription>
        </Alert>
      )}

      <section className="grid gap-4 sm:grid-cols-2" data-testid="export-detail-stats">
        <Stat label={t("exports:detail.scope")} value={scopeLabel(exp, t)} />
        <Stat
          label={t("exports:detail.createdAt")}
          value={exp.created_date ? formatDateTime(exp.created_date) : "—"}
        />
        <Stat
          label={t("exports:detail.completedAt")}
          value={exp.completed_date ? formatDateTime(exp.completed_date) : "—"}
        />
        <Stat label={t("exports:detail.totalSize")} value={formatBytes(exp.file_size ?? 0)} />
        <Stat
          label={t("exports:detail.binaryDataSize")}
          value={formatBytes(exp.binary_data_size ?? 0)}
        />
        <Stat
          label={t("exports:detail.includesFileData")}
          value={exp.include_file_data ? "✓" : "—"}
        />
      </section>

      <section
        className="grid gap-3 rounded-md border bg-muted/20 p-4 sm:grid-cols-4"
        data-testid="export-detail-counts"
      >
        <Count label={t("exports:scope.locations")} value={exp.location_count ?? 0} />
        <Count label={t("exports:scope.areas")} value={exp.area_count ?? 0} />
        <Count label={t("exports:scope.commodities")} value={exp.commodity_count ?? 0} />
        <Count label={t("exports:scope.files")} value={exp.file_count ?? 0} />
      </section>

      <section className="flex flex-col gap-3" data-testid="export-detail-restores">
        <h2 className="text-lg font-semibold">{t("exports:detail.restoreHistory")}</h2>
        <RestoreHistoryList
          exportId={exp.id}
          groupSlug={slug}
          loading={restoresQuery.isLoading}
          restores={restoresQuery.data?.restores ?? []}
        />
      </section>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs uppercase text-muted-foreground">{label}</span>
      <span className="text-sm font-medium">{value}</span>
    </div>
  )
}

function Count({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex flex-col gap-0.5 text-center">
      <span className="text-2xl font-semibold tabular-nums">{value}</span>
      <span className="text-xs text-muted-foreground">{label}</span>
    </div>
  )
}

function scopeLabel(exp: Export, t: ReturnType<typeof useTranslation>["t"]): string {
  if (exp.type === "selected_items") {
    return t("exports:detail.scopeSelectedItems", { count: exp.selected_items?.length ?? 0 })
  }
  return t(`exports:scope.${exp.type ?? "full_database"}`, {
    defaultValue: t("exports:scope.full_database"),
  })
}
