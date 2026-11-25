import { useState } from "react"
import { Link, useLocation, useNavigate } from "react-router-dom"
import { ArrowRight, Bookmark, Package, Plus, X } from "lucide-react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { useCommodities } from "@/features/commodities/hooks"
import { commodityQueryToListOptions, sameCommodityQuery } from "@/features/commodities/query"
import { useCurrentGroup } from "@/features/group/GroupContext"
import type { SavedView } from "@/features/saved-views/api"
import { useCreateSavedView, useSavedViewCounts, useSavedViews } from "@/features/saved-views/hooks"
import {
  SUGGESTED_VIEWS,
  savedViewHref,
  savedViewQuery,
  type SuggestedView,
} from "@/features/saved-views/view"
import { SavedViewIcon } from "@/features/saved-views/SavedViewIcon"
import { useAppToast } from "@/hooks/useAppToast"
import { parseServerError } from "@/lib/server-error"

// Rows a dashboard card lists before pointing at the full view.
const ROWS_PER_CARD = 5

// SavedViewsSection shows, on the dashboard, every saved view the member
// put there: its item count and first rows. Until the member keeps views of
// their own it offers a few ready-made ones instead.
export function SavedViewsSection() {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const savedViews = useSavedViews()
  const views = savedViews.data?.views ?? []
  const onDashboard = views.filter((v) => v.showOnDashboard && !v.queryError)
  const counts = useSavedViewCounts({ enabled: onDashboard.length > 0 })

  if (!savedViews.data) return null

  return (
    <>
      <SuggestedViews key={slug} slug={slug} views={views} />
      {onDashboard.length > 0 ? (
        <div className="grid gap-6 lg:grid-cols-2" data-testid="dashboard-saved-views">
          {onDashboard.map((view) => (
            <SavedViewCard key={view.id} slug={slug} view={view} count={counts.data?.[view.id]} />
          ))}
        </div>
      ) : null}
    </>
  )
}

interface SavedViewCardProps {
  slug: string
  view: SavedView
  count: number | undefined
}

function SavedViewCard({ slug, view, count }: SavedViewCardProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const options = {
    ...commodityQueryToListOptions(savedViewQuery(view)),
    page: 1,
    perPage: ROWS_PER_CARD,
  }
  const list = useCommodities(options)
  const items = list.data?.commodities ?? []
  const total = count ?? list.data?.total
  const href = savedViewHref(slug, view)

  return (
    <Card data-testid="dashboard-saved-view">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <SavedViewIcon name={view.icon} className="size-4" />
          <span className="truncate">{view.name}</span>
        </CardTitle>
        <CardDescription data-testid="dashboard-saved-view-count">
          {total === undefined
            ? " "
            : t("commodities:savedViews.dashboard.count", { count: total })}
        </CardDescription>
        <CardAction>
          <Button asChild variant="ghost" size="sm" className="gap-1">
            <Link to={href} data-testid="dashboard-saved-view-open">
              {t("commodities:savedViews.dashboard.open")}
              <ArrowRight aria-hidden="true" className="size-3.5" />
            </Link>
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="p-0">
        {list.isLoading ? (
          <ul aria-busy="true">
            {Array.from({ length: 3 }).map((_, i) => (
              <li key={i}>
                {i > 0 ? <Separator /> : null}
                <div className="flex items-center gap-3 px-6 py-3.5">
                  <Skeleton className="size-8 rounded-md" />
                  <Skeleton className="h-3 w-32" />
                </div>
              </li>
            ))}
          </ul>
        ) : list.isError ? (
          <p className="px-6 pb-6 text-sm text-muted-foreground">
            {t("commodities:savedViews.dashboard.error")}
          </p>
        ) : items.length === 0 ? (
          <p className="px-6 pb-6 text-sm text-muted-foreground">
            {t("commodities:savedViews.dashboard.empty")}
          </p>
        ) : (
          <ul>
            {items.map((item, i) => {
              const itemHref =
                slug && item.id
                  ? `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(item.id)}`
                  : "#"
              return (
                <li key={item.id ?? i}>
                  {i > 0 ? <Separator /> : null}
                  <Link
                    to={itemHref}
                    onClick={(e) => {
                      // A plain click opens the item over the dashboard, as
                      // the other dashboard lists do; modified clicks open a
                      // new tab.
                      if (!slug || !item.id) return
                      if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return
                      e.preventDefault()
                      navigate(itemHref, { state: { background: location } })
                    }}
                    className="flex w-full items-center gap-3 px-6 py-3.5 text-left transition-colors outline-none hover:bg-muted/50 focus-visible:bg-muted/50"
                    data-testid="dashboard-saved-view-row"
                  >
                    <div
                      aria-hidden="true"
                      className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted"
                    >
                      <Package className="size-4 text-muted-foreground" />
                    </div>
                    <p className="min-w-0 truncate text-sm font-medium">
                      {item.short_name || item.name}
                    </p>
                  </Link>
                </li>
              )
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}

function dismissKey(slug: string): string {
  return `savedViews:suggestionsDismissed:${slug}`
}

function readDismissed(slug: string): boolean {
  try {
    return localStorage.getItem(dismissKey(slug)) === "1"
  } catch {
    return false
  }
}

function suggestionName(t: TFunction, key: SuggestedView["key"]): string {
  switch (key) {
    case "expiringWarranties":
      return t("commodities:savedViews.suggestions.expiringWarranties.name")
    case "lentOut":
      return t("commodities:savedViews.suggestions.lentOut.name")
    case "noLocation":
      return t("commodities:savedViews.suggestions.noLocation.name")
    case "untagged":
      return t("commodities:savedViews.suggestions.untagged.name")
  }
}

function suggestionDescription(t: TFunction, key: SuggestedView["key"]): string {
  switch (key) {
    case "expiringWarranties":
      return t("commodities:savedViews.suggestions.expiringWarranties.description")
    case "lentOut":
      return t("commodities:savedViews.suggestions.lentOut.description")
    case "noLocation":
      return t("commodities:savedViews.suggestions.noLocation.description")
    case "untagged":
      return t("commodities:savedViews.suggestions.untagged.description")
  }
}

interface SuggestedViewsProps {
  slug: string
  views: SavedView[]
}

// SuggestedViews offers ready-made views to a member who keeps none of
// their own yet. Each one adds with one click, to the sidebar and the
// dashboard. A suggestion already kept drops off the list; the card goes
// once the member keeps another view or dismisses it.
function SuggestedViews({ slug, views }: SuggestedViewsProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const create = useCreateSavedView()
  const [dismissed, setDismissed] = useState(() => readDismissed(slug))
  const [pending, setPending] = useState<string | null>(null)

  const own = views.filter((v) => v.owned)
  const isSuggestion = (v: SavedView) =>
    SUGGESTED_VIEWS.some((s) => sameCommodityQuery(savedViewQuery(s), savedViewQuery(v)))
  const remaining = SUGGESTED_VIEWS.filter(
    (s) => !own.some((v) => sameCommodityQuery(savedViewQuery(s), savedViewQuery(v)))
  )
  if (dismissed || remaining.length === 0 || own.some((v) => !isSuggestion(v))) return null

  function dismiss() {
    try {
      localStorage.setItem(dismissKey(slug), "1")
    } catch {
      // Without storage the card comes back on the next visit.
    }
    setDismissed(true)
  }

  async function add(s: SuggestedView) {
    setPending(s.key)
    try {
      const name = suggestionName(t, s.key)
      await create.mutateAsync({
        name,
        icon: s.icon,
        query: s.query,
        layout: "",
        shared: false,
        showInSidebar: true,
        showOnDashboard: true,
      })
      toast.success(t("commodities:savedViews.toast.created", { name }))
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:savedViews.toast.createFailed")))
    } finally {
      setPending(null)
    }
  }

  return (
    <Card data-testid="dashboard-suggested-views">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Bookmark aria-hidden="true" className="size-4" />
          {t("commodities:savedViews.suggestions.title")}
        </CardTitle>
        <CardDescription>{t("commodities:savedViews.suggestions.description")}</CardDescription>
        <CardAction>
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            onClick={dismiss}
            aria-label={t("commodities:savedViews.suggestions.dismiss")}
            data-testid="dashboard-suggested-views-dismiss"
          >
            <X aria-hidden="true" className="size-4" />
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="p-0">
        <ul>
          {remaining.map((s, i) => {
            return (
              <li key={s.key}>
                {i > 0 ? <Separator /> : null}
                <div className="flex items-center gap-3 px-6 py-3.5">
                  <div
                    aria-hidden="true"
                    className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted"
                  >
                    <SavedViewIcon name={s.icon} className="size-4 text-muted-foreground" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{suggestionName(t, s.key)}</p>
                    <p className="truncate text-xs text-muted-foreground">
                      {suggestionDescription(t, s.key)}
                    </p>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    className="gap-1"
                    onClick={() => void add(s)}
                    disabled={pending !== null}
                    data-testid={`dashboard-suggested-view-add-${s.key}`}
                  >
                    <Plus aria-hidden="true" className="size-3.5" />
                    {t("commodities:savedViews.suggestions.add")}
                  </Button>
                </div>
              </li>
            )
          })}
        </ul>
      </CardContent>
    </Card>
  )
}
