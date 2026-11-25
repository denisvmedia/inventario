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

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
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
// query and the layout and nothing else. A view whose stored query cannot
// run is never opened, from the menu or by Revert; while the list names one,
// the menu says why and offers only to close it.
export function SavedViewControl({ query, layout }: SavedViewControlProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const navigate = useNavigate()
  const toast = useAppToast()
  const [searchParams] = useSearchParams()
  const [dialogOpen, setDialogOpen] = useState(false)

  const savedViews = useSavedViews()
  const create = useCreateSavedView()
  const update = useUpdateSavedView()
  const views = savedViews.data?.views ?? []
  const limits = savedViews.data?.limits

  const activeId = searchParams.get(SAVED_VIEW_PARAM) ?? ""
  const active = views.find((v) => v.id === activeId)
  const runnable = !!active && !active.queryError
  const changed = runnable ? isSavedViewChanged(active, query, layout) : false
  const stale = useStaleReferences(runnable ? query : undefined)
  const closeView = useCloseSavedView()

  function openView(view: SavedView) {
    const href = savedViewHref(slug, view)
    if (href) navigate(href)
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
    const href = savedViewHref(slug, saved)
    if (href) navigate(href, { replace: true })
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
            {active?.queryError ? (
              <AlertTriangle
                className="size-3.5 shrink-0 text-status-expiring"
                aria-label={t("commodities:savedViews.queryError.short")}
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
                {active.queryError ? null : (
                  <span className="text-xs font-normal text-muted-foreground">
                    {changed
                      ? t("commodities:savedViews.menu.changedHint")
                      : t("commodities:savedViews.menu.unchangedHint")}
                  </span>
                )}
              </DropdownMenuLabel>
              {active.queryError ? (
                <QueryErrorNotice parameter={active.queryError.parameter} />
              ) : null}
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

// useCloseSavedView returns a function that takes the saved view out of the
// items list's URL; the list then shows its filters as an ordinary list.
function useCloseSavedView() {
  const [searchParams, setSearchParams] = useSearchParams()
  return () => {
    const next = new URLSearchParams(searchParams)
    next.delete(SAVED_VIEW_PARAM)
    setSearchParams(next, { replace: true })
  }
}

export interface OpenedSavedView {
  // The saved views are still loading, so the list cannot tell yet whether
  // the view it was opened from can run.
  pending: boolean
  // The view the list was opened from, when its stored query cannot run.
  broken?: SavedView
}

// useOpenedSavedView tells the items list about the saved view named in its
// URL. A link to a view whose query cannot run (an old bookmark, a view that
// went bad after the link was made) must not run: the URL holds only the
// part of the query the list can read, so the list would show more than the
// view.
export function useOpenedSavedView(): OpenedSavedView {
  const [searchParams] = useSearchParams()
  const id = searchParams.get(SAVED_VIEW_PARAM) ?? ""
  const savedViews = useSavedViews({ enabled: id !== "" })
  if (!id) return { pending: false }
  return {
    pending: savedViews.isLoading,
    broken: savedViews.data?.views.find((v) => v.id === id && !!v.queryError),
  }
}

// Shown by the items list in place of the items of a view whose stored query
// cannot run.
export function SavedViewQueryErrorAlert({ view }: { view: SavedView }) {
  const { t } = useTranslation()
  const closeView = useCloseSavedView()
  return (
    <Alert variant="destructive" data-testid="saved-view-query-error-alert">
      <AlertTitle>{t("commodities:savedViews.queryError.title")}</AlertTitle>
      <AlertDescription className="flex flex-col items-start gap-2">
        <span>
          {t("commodities:savedViews.queryError.long", {
            parameter: view.queryError?.parameter || "-",
          })}
        </span>
        <Button variant="outline" size="sm" onClick={closeView}>
          {t("commodities:savedViews.menu.close")}
        </Button>
      </AlertDescription>
    </Alert>
  )
}

function QueryErrorNotice({ parameter }: { parameter: string | undefined }) {
  const { t } = useTranslation()
  return (
    <div
      className="mx-1 mb-1 flex gap-2 rounded-md bg-status-expiring/10 px-2 py-1.5 text-xs"
      data-testid="saved-views-query-error"
    >
      <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-status-expiring" aria-hidden="true" />
      <span>{t("commodities:savedViews.queryError.long", { parameter: parameter || "-" })}</span>
    </div>
  )
}

// What the query names that no longer exists, once areas and tags have
// loaded; undefined before that, when either failed to load, or without a
// query. A catalogue that failed to load lists no tags, which would mark
// every tag of the query as deleted.
export function useStaleReferences(query: CommodityQuery | undefined): StaleReferences | undefined {
  const areas = useAreas({ enabled: !!query })
  const tagIndex = useTagIndex("commodity")
  const tagsUnknown = tagIndex.isLoading || tagIndex.isError
  return useMemo(() => {
    if (!query || !areas.data || tagsUnknown) return undefined
    const areaIds = new Set(areas.data.map((a) => a.id ?? ""))
    const tagSlugs = new Set(tagIndex.tags.map((tag) => tag.slug))
    return staleReferences(query, areaIds, tagSlugs)
  }, [query, areas.data, tagsUnknown, tagIndex.tags])
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
