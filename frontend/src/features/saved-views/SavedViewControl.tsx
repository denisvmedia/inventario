import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useSearchParams } from "react-router-dom"
import {
  AlertTriangle,
  Bookmark,
  BookmarkPlus,
  ChevronDown,
  RotateCcw,
  Save,
  Settings2,
  X,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { CommodityQuery } from "@/features/commodities/query"
import { useAreas } from "@/features/areas/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useTagIndex } from "@/features/tags/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"

import type { SavedView, SavedViewLayout } from "./api"
import { useCreateSavedView, useSavedViews, useUpdateSavedView } from "./hooks"
import { SavedViewIcon } from "./SavedViewIcon"
import { SaveViewDialog, type SaveViewValues } from "./SaveViewDialog"
import {
  SAVED_VIEW_PARAM,
  hasStaleReferences,
  isSavedViewChanged,
  savedViewHref,
  savedViewQueryString,
  staleReferences,
  type StaleReferences,
} from "./view"

export interface SavedViewControlProps {
  // The query the items list shows now.
  query: CommodityQuery
  // The layout the items list shows now.
  layout: Exclude<SavedViewLayout, "">
}

// SavedViewControl is the items list's entry to saved views: it saves the
// current filter as a view, opens one, and, while a view is open, tells
// whether the list still shows it and offers to save the change or go back.
// Everything it needs it reads itself, so the list page mounts it with the
// query and the layout and nothing else.
export function SavedViewControl({ query, layout }: SavedViewControlProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const navigate = useNavigate()
  const toast = useAppToast()
  const [searchParams, setSearchParams] = useSearchParams()
  const [dialogOpen, setDialogOpen] = useState(false)

  const savedViews = useSavedViews()
  const create = useCreateSavedView()
  const update = useUpdateSavedView()
  const views = savedViews.data?.views ?? []
  const limits = savedViews.data?.limits

  const activeId = searchParams.get(SAVED_VIEW_PARAM) ?? ""
  const active = views.find((v) => v.id === activeId)
  const changed = active ? isSavedViewChanged(active, query, layout) : false
  const stale = useStaleReferences(active ? query : undefined)

  function openView(view: SavedView) {
    navigate(savedViewHref(slug, view))
  }

  function closeView() {
    const next = new URLSearchParams(searchParams)
    next.delete(SAVED_VIEW_PARAM)
    setSearchParams(next, { replace: true })
  }

  async function saveChanges(view: SavedView) {
    try {
      const saved = await update.mutateAsync({
        id: view.id,
        patch: { query: savedViewQueryString(query), layout },
      })
      toast.success(t("commodities:savedViews.toast.updated", { name: saved.name }))
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:savedViews.toast.updateFailed")))
    }
  }

  async function createView(values: SaveViewValues) {
    const saved = await create.mutateAsync({
      ...values,
      query: savedViewQueryString(query),
      layout,
    })
    setDialogOpen(false)
    toast.success(t("commodities:savedViews.toast.created", { name: saved.name }))
    navigate(savedViewHref(slug, saved), { replace: true })
  }

  const showStale = !!active && !!stale && hasStaleReferences(stale)

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant={active ? "secondary" : "outline"}
            size="sm"
            className="max-w-56 gap-1.5"
            data-testid="saved-views-menu"
          >
            {active ? (
              <SavedViewIcon name={active.icon} className="size-3.5 shrink-0" />
            ) : (
              <Bookmark className="size-3.5 shrink-0" aria-hidden="true" />
            )}
            <span className="truncate" data-testid="saved-views-menu-label">
              {active ? active.name : t("commodities:savedViews.menu.button")}
            </span>
            {changed ? (
              <span
                className="size-1.5 shrink-0 rounded-full bg-primary"
                aria-label={t("commodities:savedViews.menu.changed")}
                data-testid="saved-views-changed"
              />
            ) : null}
            {showStale ? (
              <AlertTriangle
                className="size-3.5 shrink-0 text-status-expiring"
                aria-label={t("commodities:savedViews.stale.title")}
              />
            ) : null}
            <ChevronDown className="size-3.5 shrink-0" aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-72">
          {active ? (
            <>
              <DropdownMenuLabel className="flex flex-col gap-0.5">
                <span className="truncate">{active.name}</span>
                <span className="text-xs font-normal text-muted-foreground">
                  {changed
                    ? t("commodities:savedViews.menu.changedHint")
                    : t("commodities:savedViews.menu.unchangedHint")}
                </span>
              </DropdownMenuLabel>
              {showStale && stale ? <StaleNotice stale={stale} /> : null}
              <DropdownMenuGroup>
                {changed && active.canEdit ? (
                  <DropdownMenuItem
                    onSelect={() => void saveChanges(active)}
                    disabled={update.isPending}
                    data-testid="saved-views-save-changes"
                  >
                    <Save className="size-4" aria-hidden="true" />
                    {t("commodities:savedViews.menu.saveChanges")}
                  </DropdownMenuItem>
                ) : null}
                {changed ? (
                  <DropdownMenuItem
                    onSelect={() => openView(active)}
                    data-testid="saved-views-revert"
                  >
                    <RotateCcw className="size-4" aria-hidden="true" />
                    {t("commodities:savedViews.menu.revert")}
                  </DropdownMenuItem>
                ) : null}
                <DropdownMenuItem onSelect={closeView} data-testid="saved-views-close">
                  <X className="size-4" aria-hidden="true" />
                  {t("commodities:savedViews.menu.close")}
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
            </>
          ) : null}
          <DropdownMenuItem
            onSelect={() => setDialogOpen(true)}
            disabled={!limits}
            data-testid="saved-views-save-new"
          >
            <BookmarkPlus className="size-4" aria-hidden="true" />
            {active
              ? t("commodities:savedViews.menu.saveAsNew")
              : t("commodities:savedViews.menu.save")}
          </DropdownMenuItem>
          {views.length > 0 ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">
                {t("commodities:savedViews.menu.views")}
              </DropdownMenuLabel>
              <DropdownMenuGroup className="max-h-64 overflow-y-auto">
                {views.map((view) => {
                  return (
                    <DropdownMenuItem
                      key={view.id}
                      onSelect={() => openView(view)}
                      disabled={!!view.queryError}
                      className={cn(view.id === activeId && "font-medium")}
                      data-testid="saved-views-item"
                    >
                      <SavedViewIcon name={view.icon} className="size-4" />
                      <span className="truncate">{view.name}</span>
                      {view.queryError ? (
                        <AlertTriangle
                          className="ml-auto size-3.5 text-status-expiring"
                          aria-label={t("commodities:savedViews.queryError.short")}
                        />
                      ) : null}
                    </DropdownMenuItem>
                  )
                })}
              </DropdownMenuGroup>
            </>
          ) : null}
          <DropdownMenuSeparator />
          <DropdownMenuItem asChild>
            <Link to={`/g/${encodeURIComponent(slug)}/views`} data-testid="saved-views-manage">
              <Settings2 className="size-4" aria-hidden="true" />
              {t("commodities:savedViews.menu.manage")}
            </Link>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <SaveViewDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        mode="create"
        canShare={limits?.canShare ?? false}
        maxNameLength={limits?.maxNameLength ?? 60}
        onSubmit={createView}
      />
    </>
  )
}

// What the query names that no longer exists, once areas and tags have
// loaded; undefined before that or without a query.
export function useStaleReferences(query: CommodityQuery | undefined): StaleReferences | undefined {
  const areas = useAreas({ enabled: !!query })
  const tagIndex = useTagIndex("commodity")
  return useMemo(() => {
    if (!query || !areas.data || tagIndex.isLoading) return undefined
    const areaIds = new Set(areas.data.map((a) => a.id ?? ""))
    const tagSlugs = new Set(tagIndex.tags.map((tag) => tag.slug))
    return staleReferences(query, areaIds, tagSlugs)
  }, [query, areas.data, tagIndex.isLoading, tagIndex.tags])
}

// Names what a view's filter still asks for but no longer exists. The list
// keeps the condition, so it shows what the view would have shown: nothing
// for a deleted area or a required tag that is gone.
export function StaleNotice({ stale }: { stale: StaleReferences }) {
  const { t } = useTranslation()
  return (
    <div
      className="mx-1 mb-1 flex gap-2 rounded-md bg-status-expiring/10 px-2 py-1.5 text-xs"
      data-testid="saved-views-stale"
    >
      <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-status-expiring" aria-hidden="true" />
      <div className="flex flex-col gap-0.5">
        <span className="font-medium">{t("commodities:savedViews.stale.title")}</span>
        {stale.area ? <span>{t("commodities:savedViews.stale.area")}</span> : null}
        {stale.tags.length > 0 ? (
          <span>
            {t("commodities:savedViews.stale.tags", {
              count: stale.tags.length,
              tags: stale.tags.join(", "),
            })}
          </span>
        ) : null}
      </div>
    </div>
  )
}
