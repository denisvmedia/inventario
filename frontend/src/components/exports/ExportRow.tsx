import { Download, Trash2 } from "lucide-react"
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
}

function totalItemCount(e: Export): number {
  return (
    (e.location_count ?? 0) + (e.area_count ?? 0) + (e.commodity_count ?? 0) + (e.file_count ?? 0)
  )
}

// Renders a single export as a card-style row. The download CTA is
// rendered as an `<a>` so the browser handles the stream natively; the
// JWT-protected endpoint receives the access token via `?token=` (the
// BE accepts it in addition to Authorization headers).
export function ExportRow({ export: exp, detailHref, groupSlug, onDelete }: ExportRowProps) {
  const { t } = useTranslation(["exports"])
  const isDeleted = !!exp.deleted_at
  const isTerminal = isExportTerminal(exp.status)
  const isCompleted = exp.status === "completed"
  const downloadHref = useExportDownloadHref(exp.id, groupSlug)
  const scopeLabel =
    exp.type === "selected_items"
      ? t("exports:detail.scopeSelectedItems", { count: exp.selected_items?.length ?? 0 })
      : t(`exports:scope.${exp.type ?? "full_database"}`, {
          defaultValue: t("exports:scope.full_database"),
        })

  return (
    <div
      data-testid={`export-row-${exp.id}`}
      className={cn(
        "flex flex-wrap items-center justify-between gap-3 rounded-md border bg-card px-4 py-3",
        isDeleted && "opacity-60"
      )}
    >
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex flex-wrap items-center gap-2">
          <Link
            to={detailHref}
            className="truncate text-sm font-medium hover:underline"
            data-testid={`export-row-${exp.id}-link`}
          >
            {exp.description?.trim() ? exp.description : scopeLabel}
          </Link>
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
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <span>{scopeLabel}</span>
          <span aria-hidden="true">·</span>
          <span>
            {t("exports:detail.counts.locations", { count: exp.location_count ?? 0 })} /{" "}
            {t("exports:detail.counts.commodities", { count: exp.commodity_count ?? 0 })} /{" "}
            {t("exports:detail.counts.files", { count: exp.file_count ?? 0 })}
          </span>
          <span aria-hidden="true">·</span>
          <span>{formatBytes(exp.file_size ?? 0)}</span>
          <span aria-hidden="true">·</span>
          <span>{exp.created_date ? formatDateTime(exp.created_date) : "—"}</span>
        </div>
      </div>

      <div className="flex items-center gap-2">
        {exp.status && <ExportStatusBadge status={exp.status} />}
        <Button
          asChild
          size="sm"
          variant="outline"
          disabled={!isTerminal || !isCompleted || isDeleted || !downloadHref}
          aria-disabled={!isTerminal || !isCompleted || isDeleted || !downloadHref}
          className={cn(
            (!isTerminal || !isCompleted || isDeleted || !downloadHref) && "pointer-events-none"
          )}
        >
          <a href={downloadHref ?? "#"} data-testid={`export-row-${exp.id}-download`}>
            <Download className="mr-1.5 size-4" aria-hidden="true" />
            {t("exports:actions.download")}
          </a>
        </Button>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={onDelete}
          disabled={isDeleted}
          data-testid={`export-row-${exp.id}-delete`}
          aria-label={t("exports:actions.delete")}
        >
          <Trash2 className="size-4" aria-hidden="true" />
        </Button>
      </div>

      <span className="sr-only">{totalItemCount(exp)}</span>
    </div>
  )
}
