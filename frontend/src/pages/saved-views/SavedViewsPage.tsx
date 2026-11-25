import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  Bookmark,
  LayoutDashboard,
  MoreHorizontal,
  PanelLeft,
  Pencil,
  Trash2,
  Users,
} from "lucide-react"

import { RouteTitle } from "@/components/routing/RouteTitle"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Page, PageHeader } from "@/components/ui/page"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { useAreas } from "@/features/areas/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import type { SavedView } from "@/features/saved-views/api"
import {
  useDeleteSavedView,
  useReorderSavedViews,
  useSavedViewCounts,
  useSavedViews,
  useUpdateSavedView,
} from "@/features/saved-views/hooks"
import { QueryConditions } from "@/features/saved-views/QueryConditions"
import { SaveViewDialog, type SaveViewValues } from "@/features/saved-views/SaveViewDialog"
import {
  hasStaleReferences,
  savedViewHref,
  savedViewQuery,
  staleReferences,
} from "@/features/saved-views/view"
import { SavedViewIcon } from "@/features/saved-views/SavedViewIcon"
import { useTagIndex } from "@/features/tags/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { parseServerError } from "@/lib/server-error"

// /g/:slug/views lists the saved views a member sees: their own and the
// shared ones. Each row says what the view filters, where the member shows
// it and how many items it selects. Every member puts any view they see in
// their own sidebar or on their own dashboard; they edit, reorder and delete
// the views they may change. Views are created from the items list, which
// holds the filter.
export function SavedViewsPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const toast = useAppToast()
  const confirm = useConfirm()

  const savedViews = useSavedViews()
  const counts = useSavedViewCounts()
  const areas = useAreas()
  const tagIndex = useTagIndex("commodity")
  const update = useUpdateSavedView()
  const remove = useDeleteSavedView()
  const reorder = useReorderSavedViews()
  const [editing, setEditing] = useState<SavedView | null>(null)

  const views = useMemo(() => savedViews.data?.views ?? [], [savedViews.data])
  const limits = savedViews.data?.limits
  const areaNames = useMemo(
    () => new Map((areas.data ?? []).map((a) => [a.id ?? "", a.name ?? ""])),
    [areas.data]
  )
  const referencesLoaded = !!areas.data && !tagIndex.isLoading
  const areaIds = useMemo(() => new Set(areaNames.keys()), [areaNames])
  const tagSlugs = useMemo(() => new Set(tagIndex.tags.map((tag) => tag.slug)), [tagIndex.tags])
  const editable = views.filter((v) => v.canEdit)

  async function move(view: SavedView, delta: -1 | 1) {
    const i = editable.findIndex((v) => v.id === view.id)
    const other = editable[i + delta]
    if (!other) return
    try {
      // The two views swap the positions they hold.
      await reorder.mutateAsync(delta < 0 ? [view.id, other.id] : [other.id, view.id])
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:savedViews.toast.reorderFailed")))
    }
  }

  async function toggle(view: SavedView, field: "showInSidebar" | "showOnDashboard") {
    try {
      await update.mutateAsync({ id: view.id, patch: { [field]: !view[field] } })
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:savedViews.toast.updateFailed")))
    }
  }

  async function saveEdit(values: SaveViewValues) {
    if (!editing) return
    const saved = await update.mutateAsync({ id: editing.id, patch: values })
    setEditing(null)
    toast.success(t("commodities:savedViews.toast.updated", { name: saved.name }))
  }

  async function deleteView(view: SavedView) {
    const ok = await confirm({
      title: t("commodities:savedViews.delete.title", { name: view.name }),
      description: view.shared
        ? t("commodities:savedViews.delete.sharedDescription")
        : t("commodities:savedViews.delete.description"),
      confirmLabel: t("commodities:savedViews.delete.confirm"),
      destructive: true,
    })
    if (!ok) return
    try {
      await remove.mutateAsync(view.id)
      toast.success(t("commodities:savedViews.toast.deleted", { name: view.name }))
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:savedViews.toast.deleteFailed")))
    }
  }

  return (
    <>
      <RouteTitle title={t("commodities:savedViews.page.documentTitle")} />
      <Page width="narrow" data-testid="page-saved-views">
        <PageHeader
          title={t("commodities:savedViews.page.heading")}
          subtitle={t("commodities:savedViews.page.subtitle")}
          actions={
            <Button asChild variant="outline">
              <Link to={`/g/${encodeURIComponent(slug)}/commodities`}>
                {t("commodities:savedViews.page.openItems")}
              </Link>
            </Button>
          }
        />

        {savedViews.isError ? (
          <Alert variant="destructive" data-testid="saved-views-error">
            <AlertTitle>{t("commodities:savedViews.page.errorTitle")}</AlertTitle>
            <AlertDescription>{t("commodities:savedViews.page.errorDescription")}</AlertDescription>
          </Alert>
        ) : savedViews.isLoading ? (
          <Card className="gap-0 py-0" aria-busy="true">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i}>
                {i > 0 ? <Separator /> : null}
                <div className="flex items-center gap-3 px-5 py-4">
                  <Skeleton className="size-8 rounded-md" />
                  <div className="flex-1 space-y-1.5">
                    <Skeleton className="h-3 w-40" />
                    <Skeleton className="h-3 w-64" />
                  </div>
                </div>
              </div>
            ))}
          </Card>
        ) : views.length === 0 ? (
          <Card
            className="items-center gap-2 px-6 py-10 text-center"
            data-testid="saved-views-empty"
          >
            <Bookmark aria-hidden="true" className="size-8 text-muted-foreground" />
            <p className="font-medium">{t("commodities:savedViews.page.emptyTitle")}</p>
            <p className="max-w-sm text-sm text-muted-foreground">
              {t("commodities:savedViews.page.emptyDescription")}
            </p>
          </Card>
        ) : (
          <Card className="gap-0 py-0">
            <ul>
              {views.map((view, i) => {
                const query = savedViewQuery(view)
                const stale = referencesLoaded
                  ? staleReferences(query, areaIds, tagSlugs)
                  : undefined
                const editIndex = editable.findIndex((v) => v.id === view.id)
                const count = counts.data?.[view.id]
                return (
                  <li key={view.id} data-testid="saved-view-row">
                    {i > 0 ? <Separator /> : null}
                    <div className="flex items-start gap-3 px-5 py-4">
                      <div
                        aria-hidden="true"
                        className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-muted"
                      >
                        <SavedViewIcon name={view.icon} className="size-4 text-muted-foreground" />
                      </div>
                      <div className="flex min-w-0 flex-1 flex-col gap-2">
                        <div className="flex flex-wrap items-center gap-2">
                          {view.queryError ? (
                            <span className="truncate font-medium">{view.name}</span>
                          ) : (
                            <Link
                              to={savedViewHref(slug, view)}
                              className="truncate font-medium hover:underline"
                              data-testid="saved-view-open"
                            >
                              {view.name}
                            </Link>
                          )}
                          {view.shared ? (
                            <Badge variant="secondary" className="gap-1 font-normal">
                              <Users aria-hidden="true" className="size-3" />
                              {t("commodities:savedViews.page.shared")}
                            </Badge>
                          ) : (
                            <Badge variant="outline" className="font-normal">
                              {t("commodities:savedViews.page.personal")}
                            </Badge>
                          )}
                          {count !== undefined ? (
                            <span
                              className="text-sm text-muted-foreground tabular-nums"
                              data-testid="saved-view-count"
                            >
                              {t("commodities:savedViews.dashboard.count", { count })}
                            </span>
                          ) : null}
                        </div>
                        {view.queryError ? (
                          <p
                            className="flex items-start gap-1.5 text-sm text-status-expiring"
                            data-testid="saved-view-query-error"
                          >
                            <AlertTriangle
                              aria-hidden="true"
                              className="mt-0.5 size-3.5 shrink-0"
                            />
                            {t("commodities:savedViews.queryError.long", {
                              parameter: view.queryError.parameter || "-",
                            })}
                          </p>
                        ) : (
                          <QueryConditions
                            query={query}
                            areaNames={areaNames}
                            tagLabel={(s) => tagIndex.resolve(s).label}
                            stale={stale}
                          />
                        )}
                        {stale && hasStaleReferences(stale) ? (
                          <p className="text-xs text-muted-foreground">
                            {t("commodities:savedViews.stale.hint")}
                          </p>
                        ) : null}
                      </div>
                      <div className="flex shrink-0 items-center gap-1">
                        <Button
                          variant={view.showInSidebar ? "secondary" : "ghost"}
                          size="icon"
                          className="size-8"
                          disabled={update.isPending}
                          onClick={() => void toggle(view, "showInSidebar")}
                          aria-pressed={view.showInSidebar}
                          aria-label={t("commodities:savedViews.dialog.showInSidebar")}
                          title={t("commodities:savedViews.dialog.showInSidebar")}
                          data-testid="saved-view-toggle-sidebar"
                        >
                          <PanelLeft aria-hidden="true" className="size-4" />
                        </Button>
                        <Button
                          variant={view.showOnDashboard ? "secondary" : "ghost"}
                          size="icon"
                          className="size-8"
                          disabled={update.isPending}
                          onClick={() => void toggle(view, "showOnDashboard")}
                          aria-pressed={view.showOnDashboard}
                          aria-label={t("commodities:savedViews.dialog.showOnDashboard")}
                          title={t("commodities:savedViews.dialog.showOnDashboard")}
                          data-testid="saved-view-toggle-dashboard"
                        >
                          <LayoutDashboard aria-hidden="true" className="size-4" />
                        </Button>
                        {view.canEdit || view.canDelete ? (
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="size-8"
                                aria-label={t("commodities:savedViews.page.actions", {
                                  name: view.name,
                                })}
                                data-testid="saved-view-actions"
                              >
                                <MoreHorizontal aria-hidden="true" className="size-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              {view.canEdit ? (
                                <>
                                  <DropdownMenuItem
                                    onSelect={() => setEditing(view)}
                                    data-testid="saved-view-edit"
                                  >
                                    <Pencil aria-hidden="true" className="size-4" />
                                    {t("commodities:savedViews.page.edit")}
                                  </DropdownMenuItem>
                                  <DropdownMenuItem
                                    disabled={editIndex <= 0 || reorder.isPending}
                                    onSelect={() => void move(view, -1)}
                                    data-testid="saved-view-move-up"
                                  >
                                    <ArrowUp aria-hidden="true" className="size-4" />
                                    {t("commodities:savedViews.page.moveUp")}
                                  </DropdownMenuItem>
                                  <DropdownMenuItem
                                    disabled={
                                      editIndex < 0 ||
                                      editIndex >= editable.length - 1 ||
                                      reorder.isPending
                                    }
                                    onSelect={() => void move(view, 1)}
                                    data-testid="saved-view-move-down"
                                  >
                                    <ArrowDown aria-hidden="true" className="size-4" />
                                    {t("commodities:savedViews.page.moveDown")}
                                  </DropdownMenuItem>
                                </>
                              ) : null}
                              {view.canDelete ? (
                                <>
                                  {view.canEdit ? <DropdownMenuSeparator /> : null}
                                  <DropdownMenuItem
                                    variant="destructive"
                                    onSelect={() => void deleteView(view)}
                                    data-testid="saved-view-delete"
                                  >
                                    <Trash2 aria-hidden="true" className="size-4" />
                                    {t("commodities:savedViews.page.delete")}
                                  </DropdownMenuItem>
                                </>
                              ) : null}
                            </DropdownMenuContent>
                          </DropdownMenu>
                        ) : null}
                      </div>
                    </div>
                  </li>
                )
              })}
            </ul>
          </Card>
        )}
      </Page>

      <SaveViewDialog
        open={editing !== null}
        onOpenChange={(open) => {
          if (!open) setEditing(null)
        }}
        mode="edit"
        view={editing ?? undefined}
        canShare={limits?.canShare ?? false}
        maxNameLength={limits?.maxNameLength ?? 60}
        onSubmit={saveEdit}
      />
    </>
  )
}
