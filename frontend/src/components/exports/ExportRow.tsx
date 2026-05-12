import {
  AlertTriangle,
  Download,
  HardDriveDownload,
  Loader2,
  RotateCcw,
  Trash2,
  XCircle,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"

import { ExportStatusBadge } from "@/components/exports/ExportStatusBadge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { type Export, isExportTerminal, useExportDownloadHref } from "@/features/export/api"
import { formatBytes, formatDateTime } from "@/lib/intl"
import { cn } from "@/lib/utils"

export interface ExportRowProps {
  export: Export
  detailHref: string
  groupSlug: string
  onDelete: () => void
  onRestore?: () => void
}

function totalItemCount(e: Export): number {
  return (
    (e.location_count ?? 0) + (e.area_count ?? 0) + (e.commodity_count ?? 0) + (e.file_count ?? 0)
  )
}

// Mock-spec row: tinted leading icon tile + (title row + date + stats
// line + error footer) + hover-revealed action cluster. The download CTA
// stays an `<a>` so the browser streams the JWT-protected file natively;
// the token is appended as `?token=…` (BE accepts that in addition to
// Authorization headers).
export function ExportRow({
  export: exp,
  detailHref,
  groupSlug,
  onDelete,
  onRestore,
}: ExportRowProps) {
  const { t } = useTranslation(["exports"])
  const isDeleted = !!exp.deleted_at
  const isTerminal = isExportTerminal(exp.status)
  const isCompleted = exp.status === "completed"
  const isFailed = exp.status === "failed"
  const isInFlight = exp.status === "in_progress" || exp.status === "pending"
  const downloadHref = useExportDownloadHref(exp.id, groupSlug)
  const scopeLabel =
    exp.type === "selected_items"
      ? t("exports:detail.scopeSelectedItems", { count: exp.selected_items?.length ?? 0 })
      : t(`exports:scope.${exp.type ?? "full_database"}`, {
          defaultValue: t("exports:scope.full_database"),
        })
  const showRestore = !!onRestore && isCompleted && !isDeleted

  return (
    <div
      data-testid={`export-row-${exp.id}`}
      className={cn(
        "group flex flex-col gap-3 rounded-xl border bg-card px-4 py-4 sm:flex-row sm:items-center sm:gap-4 sm:px-5",
        isDeleted && "opacity-60"
      )}
    >
      <div className="flex items-start gap-3 sm:contents">
        <div
          className={cn(
            "flex size-10 shrink-0 items-center justify-center rounded-lg",
            isCompleted && "bg-status-active/10",
            isFailed && "bg-destructive/10",
            !isCompleted && !isFailed && "bg-muted"
          )}
          aria-hidden="true"
        >
          {isInFlight ? (
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          ) : isFailed ? (
            <XCircle className="size-5 text-destructive" />
          ) : (
            <HardDriveDownload
              className={cn("size-5", isCompleted ? "text-status-active" : "text-muted-foreground")}
            />
          )}
        </div>

        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <div className="flex flex-wrap items-center gap-2">
            <Link
              to={detailHref}
              className="truncate text-sm font-semibold hover:underline"
              data-testid={`export-row-${exp.id}-link`}
            >
              {exp.description?.trim() ? exp.description : scopeLabel}
            </Link>
            {exp.status && <ExportStatusBadge status={exp.status} />}
            {exp.imported && (
              <Badge variant="secondary" data-testid={`export-row-${exp.id}-imported`}>
                {t("exports:list.imported")}
              </Badge>
            )}
            {isDeleted && (
              <Badge variant="destructive" data-testid={`export-row-${exp.id}-deleted`}>
                {t("exports:list.deletedBadge")}
              </Badge>
            )}
          </div>
          <p className="text-xs text-muted-foreground">
            {scopeLabel}
            {exp.created_date && (
              <>
                <span aria-hidden="true"> · </span>
                <span>{formatDateTime(exp.created_date)}</span>
              </>
            )}
          </p>
          {isTerminal && (
            <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
              <span>
                {t("exports:detail.counts.locations", { count: exp.location_count ?? 0 })}
              </span>
              <span>{t("exports:detail.counts.areas", { count: exp.area_count ?? 0 })}</span>
              <span>
                {t("exports:detail.counts.commodities", { count: exp.commodity_count ?? 0 })}
              </span>
              <span>{t("exports:detail.counts.files", { count: exp.file_count ?? 0 })}</span>
              <span>{formatBytes(exp.file_size ?? 0)}</span>
            </div>
          )}
          {isFailed && exp.error_message && (
            <p
              className="mt-0.5 flex items-start gap-1 text-xs text-destructive"
              data-testid={`export-row-${exp.id}-error`}
            >
              <AlertTriangle className="mt-0.5 size-3 shrink-0" aria-hidden="true" />
              <span>{exp.error_message}</span>
            </p>
          )}
        </div>

        <Button
          type="button"
          size="icon"
          variant="ghost"
          onClick={onDelete}
          disabled={isDeleted}
          data-testid={`export-row-${exp.id}-delete`}
          aria-label={t("exports:actions.delete")}
          className="size-8 shrink-0 text-muted-foreground transition-opacity hover:text-destructive sm:opacity-0 sm:group-hover:opacity-100"
        >
          <Trash2 className="size-4" aria-hidden="true" />
        </Button>
      </div>

      {isCompleted && !isDeleted && (
        <div className="flex items-center gap-2 sm:shrink-0 sm:opacity-0 sm:transition-opacity sm:group-hover:opacity-100">
          {showRestore && (
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={onRestore}
              data-testid={`export-row-${exp.id}-restore`}
              className="flex-1 gap-1.5 sm:flex-none"
            >
              <RotateCcw className="size-3.5" aria-hidden="true" />
              {t("exports:actions.restore")}
            </Button>
          )}
          <Button
            asChild
            size="sm"
            variant="outline"
            disabled={!downloadHref}
            aria-disabled={!downloadHref}
            className={cn("flex-1 gap-1.5 sm:flex-none", !downloadHref && "pointer-events-none")}
          >
            <a href={downloadHref ?? "#"} data-testid={`export-row-${exp.id}-download`}>
              <Download className="size-3.5" aria-hidden="true" />
              {t("exports:actions.download")}
            </a>
          </Button>
        </div>
      )}

      <span className="sr-only">{totalItemCount(exp)}</span>
    </div>
  )
}
