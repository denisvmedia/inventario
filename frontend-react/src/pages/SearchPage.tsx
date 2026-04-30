import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"
import {
  ArrowRight,
  Box,
  ChevronRight,
  FileText,
  MapPin,
  Package,
  Search as SearchIcon,
  Tag,
  X,
} from "lucide-react"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useSearch } from "@/features/search/hooks"
import { clearRecent, getRecent, type RecentEntry } from "@/features/search/recent"
import type {
  AreaAttrs,
  CommodityAttrs,
  FileAttrs,
  LocationAttrs,
  SearchableType,
  SearchPage as SearchPageData,
  SearchResource,
} from "@/features/search/api"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { cn } from "@/lib/utils"

// /g/:slug/search?q=... — global search across the active group's
// resources. URL holds the query (so refresh / share / browser-back
// behaves naturally); the input mirrors `searchParams.get("q")`. We fire
// one TanStack query per resource type in parallel; sections render as
// soon as their slice resolves.
//
// Tags + Files are gated behind backend issues (#1400 + #1398); they
// render as "coming soon" stubs today.
export function SearchPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get("q") ?? ""

  // Local input draft so typing doesn't push every keystroke into the
  // URL bar. Submitting the form (Enter or button) is what writes back.
  const [draft, setDraft] = useState(query)
  useEffect(() => {
    setDraft(query)
  }, [query])

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const next = draft.trim()
    if (next === query) return
    if (next) {
      setSearchParams({ q: next }, { replace: false })
    } else {
      // Clear the param entirely — `?q=` is uglier than no param at all.
      setSearchParams({}, { replace: false })
    }
  }

  function onClear() {
    setDraft("")
    setSearchParams({}, { replace: false })
  }

  return (
    <>
      <RouteTitle title={t("search:title")} />
      <div className="mx-auto flex w-full max-w-4xl flex-col gap-8" data-testid="search-page">
        <header className="space-y-2">
          <h1 className="text-2xl font-semibold tracking-tight">{t("search:title")}</h1>
          <p className="text-sm text-muted-foreground">{t("search:subtitle")}</p>
        </header>

        <form onSubmit={onSubmit} className="flex items-center gap-2" role="search">
          <div className="relative flex-1">
            <SearchIcon
              aria-hidden="true"
              className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              type="search"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder={t("search:inputPlaceholder")}
              aria-label={t("search:inputLabel")}
              className="pl-9 pr-9"
              data-testid="search-input"
            />
            {draft ? (
              <button
                type="button"
                onClick={onClear}
                aria-label={t("search:clear")}
                data-testid="search-clear"
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
              >
                <X className="size-4" aria-hidden="true" />
              </button>
            ) : null}
          </div>
          <Button type="submit" data-testid="search-submit">
            {t("search:submit")}
          </Button>
        </form>

        {query ? (
          <SearchResults query={query} groupSlug={currentGroup?.slug ?? null} />
        ) : (
          <EmptyState groupSlug={currentGroup?.slug ?? null} />
        )}
      </div>
    </>
  )
}

// --- Empty state with recent items + hints --------------------------------

function EmptyState({ groupSlug }: { groupSlug: string | null }) {
  const { t } = useTranslation()
  const scope = groupSlug ?? "default"
  // Recents come from localStorage scoped to the group slug. We snapshot
  // once on mount + on slug change; the page is single-instance so we
  // don't need a live-cross-tab subscription.
  const [recent, setRecent] = useState<RecentEntry[]>(() => getRecent(scope))
  useEffect(() => {
    setRecent(getRecent(scope))
  }, [scope])

  const hints: Array<keyof typeof HINT_KEYS> = ["item", "tag", "location", "category"]

  return (
    <div className="space-y-8" data-testid="search-empty">
      <div className="rounded-xl border border-border bg-card p-6 text-center space-y-3">
        <div className="flex justify-center">
          <div className="flex size-12 items-center justify-center rounded-full bg-muted">
            <SearchIcon className="size-5 text-muted-foreground" aria-hidden="true" />
          </div>
        </div>
        <div className="space-y-1">
          <p className="text-sm font-semibold">{t("search:queryEmptyTitle")}</p>
          <p className="text-xs text-muted-foreground leading-relaxed">
            {t("search:queryEmptyBody")}
          </p>
        </div>
        <div className="flex flex-wrap justify-center gap-2 pt-1">
          <span className="text-[11px] text-muted-foreground">{t("search:queryHints.title")}</span>
          {hints.map((h) => (
            <Badge key={h} variant="secondary" className="text-[11px]">
              {t(HINT_KEYS[h])}
            </Badge>
          ))}
        </div>
      </div>

      <div>
        <div className="flex items-center justify-between mb-3">
          <div>
            <h2 className="text-base font-semibold">{t("search:recent.title")}</h2>
            <p className="text-xs text-muted-foreground mt-0.5">{t("search:recent.subtitle")}</p>
          </div>
          {recent.length ? (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              data-testid="recent-clear"
              onClick={() => {
                clearRecent(scope)
                setRecent([])
              }}
            >
              {t("search:recent.clear")}
            </Button>
          ) : null}
        </div>
        {recent.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="recent-empty">
            {t("search:recent.empty")}
          </p>
        ) : (
          <ul className="grid grid-cols-1 sm:grid-cols-2 gap-2" data-testid="recent-list">
            {recent.map((entry) => (
              <li key={`${entry.type}:${entry.id}`}>
                <Link
                  to={entry.url}
                  className="flex items-center gap-3 rounded-xl border border-border bg-card p-3 hover:bg-muted/40 transition-colors"
                  data-testid={`recent-${entry.type}-${entry.id}`}
                >
                  <RecentIcon type={entry.type} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{entry.title}</p>
                    <p className="text-[11px] text-muted-foreground capitalize">{entry.type}</p>
                  </div>
                  <ChevronRight
                    className="size-4 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

const HINT_KEYS = {
  item: "search:queryHints.item",
  tag: "search:queryHints.tag",
  location: "search:queryHints.location",
  category: "search:queryHints.category",
} as const

function RecentIcon({ type }: { type: RecentEntry["type"] }) {
  const Icon =
    type === "commodity" ? Package : type === "location" ? MapPin : type === "area" ? Box : FileText
  return (
    <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted/60">
      <Icon className="size-4 text-muted-foreground" aria-hidden="true" />
    </div>
  )
}

// --- Results layout --------------------------------------------------------

function SearchResults({ query, groupSlug }: { query: string; groupSlug: string | null }) {
  const { t } = useTranslation()

  // Fire all four searches in parallel — TanStack dedupes per cache key
  // so re-renders don't refetch.
  const commodities = useSearch<CommodityAttrs>(query, "commodities")
  const locations = useSearch<LocationAttrs>(query, "locations")
  const areas = useSearch<AreaAttrs>(query, "areas")
  const files = useSearch<FileAttrs>(query, "files")

  // Aggregate counts feed the "no results across the board" empty state.
  const totalAcrossGroups = useMemo(
    () => [commodities, locations, areas, files].reduce((sum, q) => sum + (q.data?.total ?? 0), 0),
    [commodities, locations, areas, files]
  )

  const allDone = [commodities, locations, areas, files].every((q) => !q.isLoading && !q.isFetching)
  const allEmpty = allDone && totalAcrossGroups === 0
  const allErrored =
    allDone &&
    [commodities, locations, areas, files].every((q) => q.isError && q.data === undefined)

  if (allErrored) {
    return (
      <Alert variant="destructive" data-testid="search-error">
        <AlertDescription>
          <strong>{t("search:errorTitle")}</strong> — {t("search:errorBody")}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="space-y-8">
      {allEmpty ? (
        <div
          className="rounded-xl border border-dashed border-border bg-muted/20 p-8 text-center"
          data-testid="search-no-results"
        >
          <p className="text-sm font-semibold">{t("search:noResultsTitle", { query })}</p>
          <p className="mt-1 text-xs text-muted-foreground">{t("search:noResultsBody")}</p>
        </div>
      ) : null}
      <ResultGroup
        type="commodities"
        groupSlug={groupSlug}
        page={commodities.data}
        isLoading={commodities.isLoading}
        renderItem={(r) => <CommodityCard key={r.id} resource={r} groupSlug={groupSlug} />}
        seeAllHref={
          groupSlug
            ? `/g/${encodeURIComponent(groupSlug)}/commodities?q=${encodeURIComponent(query)}`
            : null
        }
      />
      <ResultGroup
        type="locations"
        groupSlug={groupSlug}
        page={locations.data}
        isLoading={locations.isLoading}
        renderItem={(r) => <LocationCard key={r.id} resource={r} groupSlug={groupSlug} />}
        seeAllHref={
          groupSlug
            ? `/g/${encodeURIComponent(groupSlug)}/locations?q=${encodeURIComponent(query)}`
            : null
        }
      />
      <ResultGroup
        type="areas"
        groupSlug={groupSlug}
        page={areas.data}
        isLoading={areas.isLoading}
        renderItem={(r) => <AreaCard key={r.id} resource={r} groupSlug={groupSlug} />}
        seeAllHref={null}
      />
      <ResultGroup
        type="files"
        groupSlug={groupSlug}
        page={files.data}
        isLoading={files.isLoading}
        // Files BE search depends on #1398 (files.category enum + filtered
        // list) and the legacy fallback may 501 today. Render gracefully
        // empty regardless; the page-level empty state still fires when
        // every other section is also empty.
        renderItem={(r) => <FileCard key={r.id} resource={r} groupSlug={groupSlug} />}
        seeAllHref={
          groupSlug
            ? `/g/${encodeURIComponent(groupSlug)}/files?q=${encodeURIComponent(query)}`
            : null
        }
        unavailableTracker="#1398"
        unavailableHidden={(files.data?.total ?? 0) > 0}
      />
      {/* Tags is BE-blocked on #1400 and stays a stub regardless of query. */}
      <UnavailableGroup type="tags" tracker="#1400" />
    </div>
  )
}

// --- Per-resource section --------------------------------------------------

interface ResultGroupProps<TAttrs> {
  type: SearchableType
  groupSlug: string | null
  page: SearchPageData<TAttrs> | undefined
  isLoading: boolean
  renderItem: (resource: SearchResource<TAttrs>) => ReactNode
  // Per-group "see all" link to the resource's own list page with the
  // query forwarded as `?q=`. Pass null to hide.
  seeAllHref: string | null
  // When the BE doesn't implement this resource yet, surface a stub
  // instead of a misleading "0 matches". `unavailableTracker` is the
  // GitHub issue ref; `unavailableHidden` is true when the BE has
  // returned anything (so we don't claim "unavailable" once it works).
  unavailableTracker?: string
  unavailableHidden?: boolean
}

function ResultGroup<TAttrs>({
  type,
  page,
  isLoading,
  renderItem,
  seeAllHref,
  unavailableTracker,
  unavailableHidden,
}: ResultGroupProps<TAttrs>) {
  const { t } = useTranslation()
  const groupLabel = t(`search:groups.${type}`)
  const total = page?.total ?? 0
  const showUnavailable = unavailableTracker && !isLoading && !unavailableHidden && total === 0

  return (
    <section className="space-y-3" data-testid={`group-${type}`} data-group-empty={total === 0}>
      <div className="flex items-end justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold">{groupLabel}</h2>
          <p className="text-xs text-muted-foreground mt-0.5">
            {isLoading
              ? t("search:loading")
              : showUnavailable
                ? t("search:groupUnavailableTitle", { group: groupLabel })
                : t("search:groupCount", { count: total })}
          </p>
        </div>
        {!isLoading && total > 0 && seeAllHref ? (
          <Link
            to={seeAllHref}
            className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground"
            aria-label={t("search:seeAllResults", { count: total, group: groupLabel })}
            data-testid={`group-${type}-see-all`}
          >
            {t("search:seeAll")}
            <ArrowRight className="size-3" aria-hidden="true" />
          </Link>
        ) : null}
      </div>
      {showUnavailable ? (
        <p className="text-[11px] text-muted-foreground">
          {t("search:groupUnavailableBody", { ref: unavailableTracker })}
        </p>
      ) : isLoading ? (
        <ResultSkeleton />
      ) : total === 0 ? (
        <p className="text-xs text-muted-foreground" data-testid={`group-${type}-empty`}>
          {t("search:groupEmpty", { group: groupLabel })}
        </p>
      ) : (
        <ul className="grid grid-cols-1 gap-2 sm:grid-cols-2">{page?.results.map(renderItem)}</ul>
      )}
    </section>
  )
}

function UnavailableGroup({ type, tracker }: { type: "tags"; tracker: string }) {
  const { t } = useTranslation()
  const groupLabel = t(`search:groups.${type}`)
  return (
    <section
      className="space-y-2 rounded-xl border border-dashed border-border bg-muted/20 p-4"
      data-testid={`group-${type}`}
    >
      <div className="flex items-center gap-2">
        <Tag className="size-4 text-muted-foreground" aria-hidden="true" />
        <h2 className="text-sm font-semibold">
          {t("search:groupUnavailableTitle", { group: groupLabel })}
        </h2>
      </div>
      <p className="text-xs text-muted-foreground">
        {t("search:groupUnavailableBody", { ref: tracker })}
      </p>
    </section>
  )
}

function ResultSkeleton() {
  return (
    <div
      className="grid grid-cols-1 gap-2 sm:grid-cols-2"
      role="status"
      aria-live="polite"
      aria-busy="true"
    >
      <span className="sr-only">Loading results…</span>
      {[0, 1, 2, 3].map((i) => (
        <div key={i} className="h-14 rounded-xl border border-border bg-muted/30" />
      ))}
    </div>
  )
}

// --- Result cards ---------------------------------------------------------

function CardShell({
  href,
  title,
  subtitle,
  icon,
  testId,
  ariaLabel,
}: {
  href: string | null
  title: string
  subtitle?: string
  icon: ReactNode
  testId: string
  ariaLabel: string
}) {
  const inner = (
    <div className="flex items-center gap-3">
      <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted/60">
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{title}</p>
        {subtitle ? <p className="truncate text-[11px] text-muted-foreground">{subtitle}</p> : null}
      </div>
      <ChevronRight className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
    </div>
  )
  if (!href) {
    return (
      <li>
        <div
          className={cn(
            "rounded-xl border border-border bg-card p-3",
            "opacity-70 cursor-not-allowed"
          )}
          data-testid={testId}
          aria-label={ariaLabel}
        >
          {inner}
        </div>
      </li>
    )
  }
  return (
    <li>
      <Link
        to={href}
        className="block rounded-xl border border-border bg-card p-3 hover:bg-muted/40 transition-colors"
        data-testid={testId}
        aria-label={ariaLabel}
      >
        {inner}
      </Link>
    </li>
  )
}

function CommodityCard({
  resource,
  groupSlug,
}: {
  resource: SearchResource<CommodityAttrs>
  groupSlug: string | null
}) {
  const { t } = useTranslation()
  const title = resource.attributes.name ?? resource.id
  const subtitle = resource.attributes.short_name ?? undefined
  return (
    <CardShell
      href={
        groupSlug
          ? `/g/${encodeURIComponent(groupSlug)}/commodities/${encodeURIComponent(resource.id)}`
          : null
      }
      title={title}
      subtitle={subtitle}
      icon={<Package className="size-4 text-muted-foreground" aria-hidden="true" />}
      testId={`result-commodity-${resource.id}`}
      ariaLabel={t("search:resultCard.openItem")}
    />
  )
}

function LocationCard({
  resource,
  groupSlug,
}: {
  resource: SearchResource<LocationAttrs>
  groupSlug: string | null
}) {
  const { t } = useTranslation()
  const title = resource.attributes.name ?? resource.id
  const subtitle = resource.attributes.address ?? undefined
  return (
    <CardShell
      href={
        groupSlug
          ? `/g/${encodeURIComponent(groupSlug)}/locations/${encodeURIComponent(resource.id)}`
          : null
      }
      title={title}
      subtitle={subtitle}
      icon={<MapPin className="size-4 text-muted-foreground" aria-hidden="true" />}
      testId={`result-location-${resource.id}`}
      ariaLabel={t("search:resultCard.openLocation")}
    />
  )
}

function AreaCard({
  resource,
  groupSlug,
}: {
  resource: SearchResource<AreaAttrs>
  groupSlug: string | null
}) {
  const { t } = useTranslation()
  const title = resource.attributes.name ?? resource.id
  return (
    <CardShell
      href={
        groupSlug
          ? `/g/${encodeURIComponent(groupSlug)}/areas/${encodeURIComponent(resource.id)}`
          : null
      }
      title={title}
      icon={<Box className="size-4 text-muted-foreground" aria-hidden="true" />}
      testId={`result-area-${resource.id}`}
      ariaLabel={t("search:resultCard.openArea")}
    />
  )
}

function FileCard({
  resource,
  groupSlug,
}: {
  resource: SearchResource<FileAttrs>
  groupSlug: string | null
}) {
  const { t } = useTranslation()
  const title = resource.attributes.title ?? resource.id
  const subtitle = resource.attributes.path ?? undefined
  return (
    <CardShell
      href={
        groupSlug
          ? `/g/${encodeURIComponent(groupSlug)}/files/${encodeURIComponent(resource.id)}`
          : null
      }
      title={title}
      subtitle={subtitle}
      icon={<FileText className="size-4 text-muted-foreground" aria-hidden="true" />}
      testId={`result-file-${resource.id}`}
      ariaLabel={t("search:resultCard.openFile")}
    />
  )
}
