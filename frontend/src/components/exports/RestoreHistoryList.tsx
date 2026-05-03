import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"

import { ExportStatusBadge } from "@/components/exports/ExportStatusBadge"
import { Skeleton } from "@/components/ui/skeleton"
import type { Restore } from "@/features/export/api"
import { formatDateTime } from "@/lib/intl"

export interface RestoreHistoryListProps {
  exportId: string
  groupSlug: string
  loading: boolean
  restores: Restore[]
}

// Lists previous restore operations attached to an export. Each row
// links to the detail page for the restore (rendered inline on the
// export detail for now); when the steps tree lands as its own page
// the detailHref will switch to a dedicated route.
export function RestoreHistoryList({
  exportId,
  groupSlug,
  loading,
  restores,
}: RestoreHistoryListProps) {
  const { t } = useTranslation(["exports"])
  if (loading) {
    return (
      <div className="flex flex-col gap-2" data-testid="restores-loading">
        {Array.from({ length: 2 }).map((_, idx) => (
          <Skeleton key={idx} className="h-12 w-full" />
        ))}
      </div>
    )
  }
  if (restores.length === 0) {
    return (
      <p
        className="rounded-md border bg-muted/30 px-4 py-6 text-center text-sm text-muted-foreground"
        data-testid="restores-empty"
      >
        {t("exports:detail.noRestores")}
      </p>
    )
  }
  return (
    <ul className="flex flex-col gap-2" data-testid="restores-list">
      {restores.map((r) => (
        <li
          key={r.id}
          className="flex flex-wrap items-center justify-between gap-3 rounded-md border bg-card px-4 py-3"
          data-testid={`restore-row-${r.id}`}
        >
          <div className="flex min-w-0 flex-col gap-1">
            <Link
              to={`/g/${encodeURIComponent(groupSlug)}/exports/${encodeURIComponent(exportId)}?restore=${encodeURIComponent(r.id)}`}
              className="truncate text-sm font-medium hover:underline"
            >
              {r.description?.trim() || t("exports:restore.title")}
            </Link>
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span>{r.created_date ? formatDateTime(r.created_date) : "—"}</span>
              {r.options?.dry_run && (
                <>
                  <span aria-hidden="true">·</span>
                  <span>{t("exports:restore.dryRun")}</span>
                </>
              )}
              {typeof r.options?.strategy === "string" && r.options.strategy && (
                <>
                  <span aria-hidden="true">·</span>
                  <span>
                    {t(`exports:restore.strategyLabel.${r.options.strategy}`, r.options.strategy)}
                  </span>
                </>
              )}
            </div>
          </div>
          {r.status && <ExportStatusBadge status={r.status} />}
        </li>
      ))}
    </ul>
  )
}
