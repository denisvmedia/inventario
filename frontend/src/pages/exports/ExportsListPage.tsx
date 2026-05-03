import { Plus, Upload } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"

import { ExportRow } from "@/components/exports/ExportRow"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { type Export } from "@/features/export/api"
import { useDeleteExport, useExports } from "@/features/export/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"

function exportUrl(slug: string, ...segments: (string | undefined)[]): string {
  const tail = segments.filter(Boolean).join("/")
  return tail
    ? `/g/${encodeURIComponent(slug)}/exports/${tail}`
    : `/g/${encodeURIComponent(slug)}/exports`
}

export function ExportsListPage() {
  const { t } = useTranslation(["exports", "common"])
  const toast = useAppToast()
  const confirm = useConfirm()
  const [searchParams, setSearchParams] = useSearchParams()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""

  const includeDeleted = searchParams.get("show_deleted") === "1"
  const exportsQuery = useExports({ includeDeleted })
  const deleteMutation = useDeleteExport()

  const items = exportsQuery.data?.exports ?? []
  const isInitialLoading = exportsQuery.isLoading && !exportsQuery.data
  const visibleItems = includeDeleted ? items : items.filter((e) => !e.deleted_at)
  const isLive = items.some((e) => e.status === "pending" || e.status === "in_progress")

  function toggleShowDeleted(next: boolean) {
    const search = new URLSearchParams(searchParams)
    if (next) search.set("show_deleted", "1")
    else search.delete("show_deleted")
    setSearchParams(search, { replace: true })
  }

  async function onDelete(exp: Export) {
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
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      toast.error(t("exports:errors.deleteFailed", { error: message }))
    }
  }

  return (
    <div className="flex flex-col gap-6 p-6" data-testid="page-exports">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-semibold tracking-tight">{t("exports:list.title")}</h1>
            {isLive && (
              <span
                className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
                data-testid="exports-live-indicator"
              >
                <span
                  className="size-1.5 animate-pulse rounded-full bg-primary"
                  aria-hidden="true"
                />
                {t("exports:polling.live")}
              </span>
            )}
          </div>
          <p className="max-w-prose text-sm text-muted-foreground">
            {t("exports:list.description")}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button asChild variant="outline" data-testid="exports-import-button">
            <Link to={exportUrl(slug, "import")}>
              <Upload className="mr-1.5 size-4" aria-hidden="true" />
              {t("exports:actions.import")}
            </Link>
          </Button>
          <Button asChild data-testid="exports-create-button">
            <Link to={exportUrl(slug, "new")}>
              <Plus className="mr-1.5 size-4" aria-hidden="true" />
              {t("exports:actions.createExport")}
            </Link>
          </Button>
        </div>
      </header>

      <Alert data-testid="exports-retention-banner">
        <AlertTitle>{t("exports:retention.title")}</AlertTitle>
        <AlertDescription>{t("exports:retention.description")}</AlertDescription>
      </Alert>

      <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
        <label className="inline-flex items-center gap-2">
          <input
            type="checkbox"
            className="size-4"
            checked={includeDeleted}
            onChange={(e) => toggleShowDeleted(e.target.checked)}
            data-testid="exports-show-deleted"
          />
          {t("exports:actions.showDeleted")}
        </label>
      </div>

      {isInitialLoading ? (
        <div className="flex flex-col gap-2" data-testid="exports-list-loading">
          {Array.from({ length: 4 }).map((_, idx) => (
            <Skeleton key={idx} className="h-20 w-full" />
          ))}
        </div>
      ) : exportsQuery.isError ? (
        <div
          className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive"
          role="alert"
          data-testid="exports-list-error"
        >
          {t("exports:errors.loadFailed", {
            error: exportsQuery.error instanceof Error ? exportsQuery.error.message : "unknown",
          })}
        </div>
      ) : visibleItems.length === 0 ? (
        <div
          className="rounded-md border bg-muted/30 px-4 py-10 text-center text-sm text-muted-foreground"
          data-testid="exports-list-empty"
        >
          {includeDeleted ? t("exports:list.emptyFiltered") : t("exports:list.empty")}
        </div>
      ) : (
        <ul className="flex flex-col gap-2" data-testid="exports-list">
          {visibleItems.map((exp) => (
            <li key={exp.id}>
              <ExportRow
                export={exp}
                detailHref={exportUrl(slug, exp.id)}
                groupSlug={slug}
                onDelete={() => onDelete(exp)}
              />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
