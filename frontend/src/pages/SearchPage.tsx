import { useEffect, useState, type FormEvent, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"
import {
  ArrowRight,
  Box,
  ChevronRight,
  FileText,
  ListFilter,
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
import { Page, PageHeader } from "@/components/ui/page"
import {
  commodityListHref,
  primarySearchMatch,
  type CommoditySearchMatch,
} from "@/features/commodities/query"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useSearch } from "@/features/search/hooks"
import {
  areaHref,
  areaItemsHref,
  commodityHref,
  commoditySubtitle,
  fileHref,
  locationHref,
  tagHref,
  tagListHref,
} from "@/features/search/results"
import { useTagLabel } from "@/features/tags/hooks"
import { clearRecent, getRecent, pushRecent, type RecentEntry } from "@/features/search/recent"
import type {
  AreaAttrs,
  CommodityAttrs,
  FileAttrs,
  LocationAttrs,
  SearchableType,
  SearchPage as SearchPageData,
  SearchResource,
  TagAttrs,
} from "@/features/search/api"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useCustomFieldName } from "@/features/customFields/hooks"
import { cn } from "@/lib/utils"

// /g/:slug/search?q=... — global search across the active group's
// resources. URL holds the query (so refresh / share / browser-back
// behaves naturally); the input mirrors `searchParams.get("q")`. We fire
// one TanStack query per resource type in parallel; sections render as
// soon as their slice resolves. The command palette's "Search 'X'…" row
// opens this page with its query.
export function SearchPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get("q") ?? ""

  // Local input draft so typing doesn't push every keystroke into the
  // URL bar. Submitting the form (Enter or button) is what writes back.
  const [draft, setDraft] = useState(query)
  // Re-seed draft when URL query changes (back/forward navigation).
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
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
      <Page width="narrow" className="gap-8" data-testid="search-page">
        <PageHeader title={t("search:title")} subtitle={t("search:subtitle")} />

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
      </Page>
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
  // Re-snapshot localStorage when the group slug changes.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
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
                    <p className="text-[11px] text-muted-foreground">
                      {t(`search:kinds.${entry.type}`)}
                    </p>
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
    type === "commodity"
      ? Package
      : type === "location"
        ? MapPin
        : type === "area"
          ? Box
          : type === "tag"
            ? Tag
            : FileText
  return (
    <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted/60">
      <Icon className="size-4 text-muted-foreground" aria-hidden="true" />
    </div>
  )
}

// --- Results layout --------------------------------------------------------

function SearchResults({ query, groupSlug }: { query: string; groupSlug: string | null }) {
  const { t } = useTranslation()

  // Fire all five searches in parallel — TanStack dedupes per cache key
  // so re-renders don't refetch.
  const commodities = useSearch<CommodityAttrs>(query, "commodities", { groupSlug })
  const tagLabel = useTagLabel("commodity")
  const locations = useSearch<LocationAttrs>(query, "locations", { groupSlug })
  const areas = useSearch<AreaAttrs>(query, "areas", { groupSlug })
  const files = useSearch<FileAttrs>(query, "files", { groupSlug })
  const tags = useSearch<TagAttrs>(query, "tags", { groupSlug })

  const sections = [commodities, locations, areas, files, tags]

  // "No matches across the board" only fires when every section actually
  // returned a usable result with total === 0. Errored sections and the
  // 501-unavailable ones are excluded so we don't claim "no matches" when
  // really the BE just couldn't answer.
  const allDone = sections.every((q) => !q.isLoading && !q.isFetching)
  const usableSections = sections.filter(
    (q) => !q.isError && q.data !== undefined && !q.data.unavailable
  )
  const allEmpty =
    allDone && usableSections.length > 0 && usableSections.every((q) => (q.data?.total ?? 0) === 0)
  const allErrored = allDone && sections.every((q) => q.isError && q.data === undefined)

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
        renderItem={(r) => (
          <CommodityCard
            key={r.id}
            resource={r}
            groupSlug={groupSlug}
            match={primarySearchMatch(commodities.data?.matches[r.id])}
            tagLabel={tagLabel}
          />
        )}
        // The search includes drafts and inactive items; so does the list
        // it links to, or "see all" would show fewer items than counted.
        seeAllHref={
          groupSlug ? commodityListHref(groupSlug, { text: query, includeInactive: true }) : null
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
        // list); until then the BE returns 501, which the api wrapper
        // folds into `unavailable: true` so we render the stub. Once the
        // BE ships, `unavailable` flips to false on the next fetch and
        // the section renders normally without any FE change.
        renderItem={(r) => <FileCard key={r.id} resource={r} groupSlug={groupSlug} />}
        seeAllHref={
          groupSlug
            ? `/g/${encodeURIComponent(groupSlug)}/files?q=${encodeURIComponent(query)}`
            : null
        }
        unavailableTracker="#1398"
      />
      <ResultGroup
        type="tags"
        groupSlug={groupSlug}
        page={tags.data}
        isLoading={tags.isLoading}
        renderItem={(r) => <TagCard key={r.id} resource={r} groupSlug={groupSlug} />}
        seeAllHref={
          groupSlug
            ? `/g/${encodeURIComponent(groupSlug)}/tags?q=${encodeURIComponent(query)}`
            : null
        }
      />
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
  // GitHub issue ref to cite in the unavailable stub. The decision to
  // show the stub comes from `page.unavailable` (set by the api wrapper
  // when the BE replies 501), so an unimplemented endpoint is the only
  // path that triggers it — a real "0 matches" response stays as the
  // normal empty body.
  unavailableTracker?: string
}

function ResultGroup<TAttrs>({
  type,
  page,
  isLoading,
  renderItem,
  seeAllHref,
  unavailableTracker,
}: ResultGroupProps<TAttrs>) {
  const { t } = useTranslation()
  const groupLabel = t(`search:groups.${type}`)
  const total = page?.total ?? 0
  const showUnavailable =
    !isLoading && unavailableTracker !== undefined && page?.unavailable === true

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

interface RelatedLink {
  href: string
  label: string
  ariaLabel: string
  testId: string
}

function CardShell({
  href,
  title,
  subtitle,
  icon,
  testId,
  ariaLabel,
  recent,
  related,
}: {
  href: string | null
  title: string
  subtitle?: string
  icon: ReactNode
  testId: string
  ariaLabel: string
  // Remembered when the card is opened, for the empty page and the empty
  // command palette.
  recent?: { scope: string; entry: Omit<RecentEntry, "visitedAt" | "url"> }
  // A second link on the card: the items in an area, the items or files
  // carrying a tag.
  related?: RelatedLink | null
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
      {related ? null : (
        <ChevronRight className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      )}
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
  const onOpen = recent ? () => pushRecent(recent.scope, { ...recent.entry, url: href }) : undefined
  if (!related) {
    return (
      <li>
        <Link
          to={href}
          onClick={onOpen}
          className="block rounded-xl border border-border bg-card p-3 hover:bg-muted/40 transition-colors"
          data-testid={testId}
          aria-label={ariaLabel}
        >
          {inner}
        </Link>
      </li>
    )
  }
  return (
    <li className="flex items-center gap-1 rounded-xl border border-border bg-card pr-2 hover:bg-muted/40 transition-colors">
      <Link
        to={href}
        onClick={onOpen}
        className="min-w-0 flex-1 rounded-xl p-3"
        data-testid={testId}
        aria-label={ariaLabel}
      >
        {inner}
      </Link>
      <Button asChild size="sm" variant="ghost" className="shrink-0 text-muted-foreground">
        <Link to={related.href} aria-label={related.ariaLabel} data-testid={related.testId}>
          <ListFilter aria-hidden="true" />
          {related.label}
        </Link>
      </Button>
    </li>
  )
}

function CommodityCard({
  resource,
  groupSlug,
  match,
  tagLabel,
}: {
  resource: SearchResource<CommodityAttrs>
  groupSlug: string | null
  match: CommoditySearchMatch | undefined
  tagLabel: (slug: string) => string
}) {
  const { t } = useTranslation()
  const customFieldName = useCustomFieldName()
  const title = resource.attributes.name ?? resource.id
  return (
    <CardShell
      href={groupSlug ? commodityHref(groupSlug, resource.id) : null}
      title={title}
      subtitle={commoditySubtitle(t, resource.attributes, match, tagLabel, customFieldName)}
      icon={<Package className="size-4 text-muted-foreground" aria-hidden="true" />}
      testId={`result-commodity-${resource.id}`}
      ariaLabel={t("search:resultCard.openItem")}
      recent={
        groupSlug
          ? { scope: groupSlug, entry: { type: "commodity", id: resource.id, title } }
          : undefined
      }
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
  const subtitle = resource.attributes.address || undefined
  return (
    <CardShell
      href={groupSlug ? locationHref(groupSlug, resource.id) : null}
      title={title}
      subtitle={subtitle}
      icon={<MapPin className="size-4 text-muted-foreground" aria-hidden="true" />}
      testId={`result-location-${resource.id}`}
      ariaLabel={t("search:resultCard.openLocation")}
      recent={
        groupSlug
          ? { scope: groupSlug, entry: { type: "location", id: resource.id, title } }
          : undefined
      }
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
      href={groupSlug ? areaHref(groupSlug, resource.id) : null}
      title={title}
      icon={<Box className="size-4 text-muted-foreground" aria-hidden="true" />}
      testId={`result-area-${resource.id}`}
      ariaLabel={t("search:resultCard.openArea")}
      recent={
        groupSlug
          ? { scope: groupSlug, entry: { type: "area", id: resource.id, title } }
          : undefined
      }
      related={
        groupSlug
          ? {
              href: areaItemsHref(groupSlug, resource.id),
              label: t("search:resultCard.showItems"),
              ariaLabel: t("search:resultCard.showItemsInArea", { name: title }),
              testId: `result-area-items-${resource.id}`,
            }
          : null
      }
    />
  )
}

function TagCard({
  resource,
  groupSlug,
}: {
  resource: SearchResource<TagAttrs>
  groupSlug: string | null
}) {
  const { t } = useTranslation()
  const title = resource.attributes.label || resource.attributes.slug || resource.id
  const isFileTag = resource.attributes.kind === "file"
  return (
    <CardShell
      href={groupSlug ? tagHref(groupSlug, resource.attributes) : null}
      title={title}
      subtitle={isFileTag ? t("search:resultCard.fileTag") : t("search:resultCard.itemTag")}
      icon={<Tag className="size-4 text-muted-foreground" aria-hidden="true" />}
      testId={`result-tag-${resource.id}`}
      ariaLabel={t("search:resultCard.openTag")}
      recent={
        groupSlug ? { scope: groupSlug, entry: { type: "tag", id: resource.id, title } } : undefined
      }
      related={
        groupSlug
          ? {
              href: tagListHref(groupSlug, resource.attributes),
              label: isFileTag
                ? t("search:resultCard.showFiles")
                : t("search:resultCard.showItems"),
              ariaLabel: isFileTag
                ? t("search:resultCard.showFilesTagged", { name: title })
                : t("search:resultCard.showItemsTagged", { name: title }),
              testId: `result-tag-list-${resource.id}`,
            }
          : null
      }
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
      href={groupSlug ? fileHref(groupSlug, resource.id) : null}
      title={title}
      subtitle={subtitle}
      icon={<FileText className="size-4 text-muted-foreground" aria-hidden="true" />}
      testId={`result-file-${resource.id}`}
      ariaLabel={t("search:resultCard.openFile")}
      recent={
        groupSlug
          ? { scope: groupSlug, entry: { type: "file", id: resource.id, title } }
          : undefined
      }
    />
  )
}
