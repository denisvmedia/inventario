import { useMemo, useState } from "react"
import { defaultFilter } from "cmdk"
import {
  ArrowRight,
  Box,
  FileText,
  FolderOpen,
  HardDriveDownload,
  LayoutDashboard,
  ListFilter,
  ListPlus,
  MapPin,
  Package,
  Search,
  Settings,
  Shapes,
  ShieldCheck,
  SlidersHorizontal,
  Store,
  Tag,
  Trash2,
  User,
  Users,
  type LucideIcon,
} from "lucide-react"
import { useNavigate } from "react-router-dom"
import { useTranslation } from "react-i18next"

import {
  CommandDialog,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command"
import { commodityListHref, primarySearchMatch } from "@/features/commodities/query"
import { useOptionalAuth } from "@/features/auth/AuthContext"
import { useCurrentGroup } from "@/features/group/GroupContext"
import type { LocationGroup } from "@/features/group/api"
import { isSectionHidden, type GroupSection } from "@/features/group/sections"
import type {
  AreaAttrs,
  CommodityAttrs,
  FileAttrs,
  LocationAttrs,
  TagAttrs,
} from "@/features/search/api"
import { useSearch } from "@/features/search/hooks"
import { getRecent, pushRecent, recentScope, type RecentEntry } from "@/features/search/recent"
import {
  areaHref,
  areaItemsHref,
  locationItemsHref,
  commodityHref,
  commoditySubtitle,
  fileHref,
  locationHref,
  tagHref,
  tagListHref,
} from "@/features/search/results"
import { useTagLabel } from "@/features/tags/hooks"
import { useCustomFieldName } from "@/features/customFields/hooks"
import { useDebouncedValue } from "@/hooks/useDebouncedValue"
import { withGroupQuery } from "@/lib/group-aware-url"
import { useNavLabel } from "@/lib/nav-labels"

interface PaletteEntry {
  // Translation key (under common:nav.*). The palette label is the same
  // string as the sidebar.
  labelKey: string
  // Resolves to a path; null hides the entry (used for group-scoped routes
  // when the user has no active group). Receives the active group so each
  // entry can pick what it needs (slug, id, or none) — same signature as
  // AppSidebar.NavEntry.
  to: (group: LocationGroup | null) => string | null
  icon: typeof Search
  // The group section the entry belongs to; the entry leaves the palette
  // while the active group hides it (#1607).
  section?: GroupSection
}

const NAVIGATION: PaletteEntry[] = [
  {
    labelKey: "common:nav.dashboard",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}` : null),
    icon: LayoutDashboard,
  },
  {
    labelKey: "common:nav.search",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/search` : null),
    icon: Search,
  },
  {
    labelKey: "common:nav.locations",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/locations` : null),
    icon: MapPin,
  },
  {
    labelKey: "common:nav.items",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/commodities` : null),
    icon: Package,
  },
  {
    labelKey: "common:nav.warranties",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/warranties` : null),
    icon: ShieldCheck,
    section: "warranties",
  },
  {
    labelKey: "common:nav.tags",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/tags` : null),
    icon: Tag,
  },
  {
    labelKey: "common:nav.sellers",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/sellers` : null),
    icon: Store,
  },
  {
    labelKey: "common:nav.itemTypes",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/item-types` : null),
    icon: Shapes,
  },
  {
    labelKey: "common:nav.customFields",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/custom-fields` : null),
    icon: ListPlus,
  },
  {
    labelKey: "common:nav.files",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/files` : null),
    icon: FolderOpen,
  },
  {
    labelKey: "common:nav.members",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/members` : null),
    icon: Users,
  },
  {
    labelKey: "common:nav.backup",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/exports` : null),
    icon: HardDriveDownload,
  },
  {
    // Settings → real GroupSettingsPage at /groups/:id/settings,
    // mirroring AppSidebar's MANAGE.system entry.
    labelKey: "common:nav.system",
    to: (g) => (g?.id ? `/groups/${encodeURIComponent(g.id)}/settings` : null),
    icon: Settings,
  },
  {
    labelKey: "common:nav.trash",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/trash` : null),
    icon: Trash2,
  },
  {
    labelKey: "common:nav.profile",
    to: (g) => withGroupQuery("/profile", g?.slug),
    icon: User,
  },
  {
    labelKey: "common:nav.preferences",
    to: (g) => withGroupQuery("/settings", g?.slug),
    icon: SlidersHorizontal,
  },
]

// Translation keys above are full namespace-qualified paths (e.g.
// "common:nav.dashboard") so we can pass them straight to t() as a
// variable and avoid template-literal extraction noise from
// i18next-cli (see the matching note in AppSidebar.tsx).

// The server search runs from this many characters on, a moment after the
// typing stops.
const MIN_SEARCH_CHARS = 2
const SEARCH_DEBOUNCE_MS = 250
// Rows per kind. Items get more room: they are what is searched for most.
const ITEM_LIMIT = 5
const OTHER_LIMIT = 3
const RECENT_LIMIT = 5

// Search results and recent entries carry values with this prefix. The
// server has already matched them, so the palette keeps them as they are;
// cmdk's own fuzzy match only narrows the navigation entries. Results score
// as high as an exact navigation match and come first in the list, so they
// stay above the navigation entries.
const RESULT_PREFIX = "result:"

function paletteFilter(value: string, search: string, keywords?: string[]): number {
  if (value.startsWith(RESULT_PREFIX)) return 1
  return defaultFilter(value, search, keywords)
}

const RESOURCE_ICON: Record<RecentEntry["type"], LucideIcon> = {
  commodity: Package,
  location: MapPin,
  area: Box,
  tag: Tag,
  file: FileText,
}

interface PaletteRowProps {
  entry: PaletteEntry
  target: string | null
  onSelect: (path: string | null) => void
}

// PaletteRow is broken out so `useNavLabel` (a hook) can be called once
// per entry without violating the rules-of-hooks ordering (mapping inside
// the parent component would call the hook in a loop, which is fine in
// React 19 but read-only-disallowed by the rule).
function PaletteRow({ entry, target, onSelect }: PaletteRowProps) {
  const label = useNavLabel(entry.labelKey)
  const Icon = entry.icon
  return (
    <CommandItem value={label} disabled={!target} onSelect={() => onSelect(target)}>
      <Icon className="size-4" />
      <span>{label}</span>
    </CommandItem>
  )
}

interface RelatedAction {
  label: string
  ariaLabel: string
  onSelect: () => void
  testId: string
}

interface ResultRowProps {
  value: string
  icon: LucideIcon
  title: string
  subtitle?: string
  onSelect: () => void
  testId: string
  // A second action on the row: the items in a location or an area, the
  // items or files carrying a tag. It is an option of its own, so arrow keys, a click and
  // a tap reach it like the row.
  related?: RelatedAction
}

function ResultRow({
  value,
  icon: Icon,
  title,
  subtitle,
  onSelect,
  testId,
  related,
}: ResultRowProps) {
  const row = (
    <CommandItem
      value={value}
      onSelect={onSelect}
      className="min-w-0 flex-1 gap-3"
      data-testid={testId}
    >
      <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted">
        <Icon className="size-3.5 text-muted-foreground" />
      </div>
      {/* Below sm the subtitle goes under the title, so a narrow screen
          gives the title the whole row. */}
      <div className="flex min-w-0 flex-1 flex-col sm:flex-row sm:items-baseline sm:gap-2">
        <span className="truncate text-sm font-medium">{title}</span>
        {subtitle ? (
          <span className="truncate text-xs text-muted-foreground">{subtitle}</span>
        ) : null}
      </div>
    </CommandItem>
  )
  if (!related) return row
  // Below sm the second action is its icon alone; its name stays in
  // aria-label and title.
  return (
    <div className="flex items-center gap-1">
      {row}
      <CommandItem
        value={`${value}:related`}
        onSelect={related.onSelect}
        aria-label={related.ariaLabel}
        title={related.ariaLabel}
        className="shrink-0 gap-1.5 text-xs text-muted-foreground"
        data-testid={related.testId}
      >
        <ListFilter className="size-3.5" />
        <span className="hidden sm:inline" data-testid={`${related.testId}-label`}>
          {related.label}
        </span>
      </CommandItem>
    </div>
  )
}

interface CommandPaletteProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCloseAutoFocus?: (event: Event) => void
  onInteractOutside?: (event: Event) => void
}

// CommandPalette is the global search and quick-nav. CommandPaletteProvider
// owns its open state: Cmd/Ctrl+K and the search entry in the TopBar open
// it. Typing searches items, locations, areas, files and tags of the active
// group through GET /search and lists the matches by kind; a row opens the
// result, and an area or tag row also offers its items. "Search 'X'…" hands
// the query to the search page (#1416), and the items group links to the
// items list with the same query.
export function CommandPalette({
  open,
  onOpenChange,
  onCloseAutoFocus,
  onInteractOutside,
}: CommandPaletteProps) {
  const { t } = useTranslation()
  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      onCloseAutoFocus={onCloseAutoFocus}
      onInteractOutside={onInteractOutside}
      contentTestId="command-palette"
      filter={paletteFilter}
      title={t("common:shell.commandTitle")}
      description={t("common:shell.commandDescription")}
    >
      {/* The dialog unmounts its content when it closes, so each open
          starts with an empty query and the hooks below run only while
          the palette is on screen. */}
      <PaletteBody onClose={() => onOpenChange(false)} />
    </CommandDialog>
  )
}

function PaletteBody({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const groupSlug = currentGroup?.slug ?? null
  const userId = useOptionalAuth()?.user?.id
  const recentKey = userId && groupSlug ? recentScope(userId, groupSlug) : null
  const [query, setQuery] = useState("")
  const trimmedQuery = query.trim()
  const debouncedQuery = useDebouncedValue(trimmedQuery, SEARCH_DEBOUNCE_MS)
  const searching = groupSlug !== null && trimmedQuery.length >= MIN_SEARCH_CHARS
  // Results are shown for the query as typed only; while the debounce
  // trails, the previous query's results would describe something else.
  const settledQuery = searching && debouncedQuery === trimmedQuery ? debouncedQuery : ""

  const searchOptions = { groupSlug, minChars: MIN_SEARCH_CHARS }
  const items = useSearch<CommodityAttrs>(settledQuery, "commodities", {
    ...searchOptions,
    limit: ITEM_LIMIT,
  })
  const locations = useSearch<LocationAttrs>(settledQuery, "locations", {
    ...searchOptions,
    limit: OTHER_LIMIT,
  })
  const areas = useSearch<AreaAttrs>(settledQuery, "areas", {
    ...searchOptions,
    limit: OTHER_LIMIT,
  })
  const files = useSearch<FileAttrs>(settledQuery, "files", {
    ...searchOptions,
    limit: OTHER_LIMIT,
  })
  const tags = useSearch<TagAttrs>(settledQuery, "tags", { ...searchOptions, limit: OTHER_LIMIT })
  const tagLabel = useTagLabel("commodity")
  const customFieldName = useCustomFieldName()

  // Read once per open; the palette is not on screen while a result is
  // being visited.
  const recent = useMemo(
    () => (recentKey ? getRecent(recentKey).slice(0, RECENT_LIMIT) : []),
    [recentKey]
  )

  const sections = [items, locations, areas, files, tags]
  // isLoading, not isFetching: a background refetch of a cached query keeps
  // its results on screen.
  const pending = searching && (settledQuery === "" || sections.some((s) => s.isLoading))
  const resultCount = sections.reduce((sum, s) => sum + (s.data?.results.length ?? 0), 0)
  const allFailed = sections.every((s) => s.isError)

  function go(path: string | null) {
    if (!path) return
    onClose()
    navigate(path)
  }

  // Opens a result and remembers it for the empty palette and the empty
  // search page.
  function openResult(entry: Omit<RecentEntry, "visitedAt">) {
    if (recentKey) pushRecent(recentKey, entry)
    go(entry.url)
  }

  function runSearch() {
    if (!groupSlug || !trimmedQuery) return
    go(`/g/${encodeURIComponent(groupSlug)}/search?q=${encodeURIComponent(trimmedQuery)}`)
  }

  let status: { text: string; testId: string } | null = null
  if (trimmedQuery.length < MIN_SEARCH_CHARS) {
    if (groupSlug) status = { text: t("common:shell.commandHint"), testId: "palette-hint" }
  } else if (searching && pending) {
    status = { text: t("search:loading"), testId: "palette-searching" }
  } else if (searching && resultCount === 0) {
    status = allFailed
      ? { text: t("search:errorTitle"), testId: "palette-search-error" }
      : {
          text: t("common:shell.commandNoResults", { query: trimmedQuery }),
          testId: "palette-no-results",
        }
  }

  const showResults = searching && !pending && groupSlug !== null

  return (
    <>
      <CommandInput
        placeholder={t("common:shell.commandPlaceholder")}
        value={query}
        onValueChange={setQuery}
      />
      {/* Outside the listbox, so it is announced as a status rather than
          read as an option. */}
      {status ? (
        <p
          role="status"
          className="px-4 pt-3 pb-1 text-xs text-muted-foreground"
          data-testid={status.testId}
        >
          {status.text}
        </p>
      ) : null}
      <CommandList>
        {trimmedQuery && groupSlug ? (
          <>
            <CommandGroup heading={t("common:shell.commandGroupSearch")}>
              <CommandItem
                value={`${RESULT_PREFIX}see-all`}
                onSelect={runSearch}
                data-testid="palette-search-handoff"
              >
                <Search className="size-4" />
                <span>{t("common:shell.commandSearchHandoff", { query: trimmedQuery })}</span>
              </CommandItem>
            </CommandGroup>
            <CommandSeparator />
          </>
        ) : null}

        {showResults && items.data && items.data.results.length > 0 ? (
          <CommandGroup heading={t("search:groups.commodities")} data-testid="palette-group-items">
            {items.data.results.map((r) => {
              const title = r.attributes.name ?? r.id
              return (
                <ResultRow
                  key={r.id}
                  value={`${RESULT_PREFIX}commodity:${r.id}`}
                  icon={Package}
                  title={title}
                  subtitle={commoditySubtitle(
                    t,
                    r.attributes,
                    primarySearchMatch(items.data?.matches[r.id]),
                    tagLabel,
                    customFieldName
                  )}
                  onSelect={() =>
                    openResult({
                      type: "commodity",
                      id: r.id,
                      title,
                      url: commodityHref(groupSlug, r.id),
                    })
                  }
                  testId={`palette-result-commodity-${r.id}`}
                />
              )
            })}
            {/* /search counts drafts and inactive items, so the list shows
                them too, as the search page's "see all" does. */}
            <CommandItem
              value={`${RESULT_PREFIX}all-items`}
              onSelect={() =>
                go(commodityListHref(groupSlug, { text: settledQuery, includeInactive: true }))
              }
              className="text-muted-foreground"
              data-testid="palette-all-items"
            >
              <ArrowRight className="size-4" />
              <span>{t("common:shell.commandAllItems", { total: items.data.total })}</span>
            </CommandItem>
          </CommandGroup>
        ) : null}

        {showResults && locations.data && locations.data.results.length > 0 ? (
          <CommandGroup
            heading={t("search:groups.locations")}
            data-testid="palette-group-locations"
          >
            {locations.data.results.map((r) => {
              const title = r.attributes.name ?? r.id
              return (
                <ResultRow
                  key={r.id}
                  value={`${RESULT_PREFIX}location:${r.id}`}
                  icon={MapPin}
                  title={title}
                  subtitle={r.attributes.address || undefined}
                  onSelect={() =>
                    openResult({
                      type: "location",
                      id: r.id,
                      title,
                      url: locationHref(groupSlug, r.id),
                    })
                  }
                  testId={`palette-result-location-${r.id}`}
                  related={{
                    label: t("search:resultCard.showItems"),
                    ariaLabel: t("search:resultCard.showItemsInLocation", { name: title }),
                    onSelect: () => go(locationItemsHref(groupSlug, r.id)),
                    testId: `palette-location-items-${r.id}`,
                  }}
                />
              )
            })}
          </CommandGroup>
        ) : null}

        {showResults && areas.data && areas.data.results.length > 0 ? (
          <CommandGroup heading={t("search:groups.areas")} data-testid="palette-group-areas">
            {areas.data.results.map((r) => {
              const title = r.attributes.name ?? r.id
              return (
                <ResultRow
                  key={r.id}
                  value={`${RESULT_PREFIX}area:${r.id}`}
                  icon={Box}
                  title={title}
                  onSelect={() =>
                    openResult({ type: "area", id: r.id, title, url: areaHref(groupSlug, r.id) })
                  }
                  testId={`palette-result-area-${r.id}`}
                  related={{
                    label: t("search:resultCard.showItems"),
                    ariaLabel: t("search:resultCard.showItemsInArea", { name: title }),
                    onSelect: () => go(areaItemsHref(groupSlug, r.id)),
                    testId: `palette-area-items-${r.id}`,
                  }}
                />
              )
            })}
          </CommandGroup>
        ) : null}

        {showResults && files.data && files.data.results.length > 0 ? (
          <CommandGroup heading={t("search:groups.files")} data-testid="palette-group-files">
            {files.data.results.map((r) => {
              const title = r.attributes.title || r.attributes.path || r.id
              return (
                <ResultRow
                  key={r.id}
                  value={`${RESULT_PREFIX}file:${r.id}`}
                  icon={FileText}
                  title={title}
                  subtitle={
                    r.attributes.path && r.attributes.path !== title ? r.attributes.path : undefined
                  }
                  onSelect={() =>
                    openResult({ type: "file", id: r.id, title, url: fileHref(groupSlug, r.id) })
                  }
                  testId={`palette-result-file-${r.id}`}
                />
              )
            })}
          </CommandGroup>
        ) : null}

        {showResults && tags.data && tags.data.results.length > 0 ? (
          <CommandGroup heading={t("search:groups.tags")} data-testid="palette-group-tags">
            {tags.data.results.map((r) => {
              const title = r.attributes.label || r.attributes.slug || r.id
              const isFileTag = r.attributes.kind === "file"
              return (
                <ResultRow
                  key={r.id}
                  value={`${RESULT_PREFIX}tag:${r.id}`}
                  icon={Tag}
                  title={title}
                  subtitle={
                    isFileTag ? t("search:resultCard.fileTag") : t("search:resultCard.itemTag")
                  }
                  onSelect={() =>
                    openResult({
                      type: "tag",
                      id: r.id,
                      title,
                      url: tagHref(groupSlug, r.attributes),
                    })
                  }
                  testId={`palette-result-tag-${r.id}`}
                  related={{
                    label: isFileTag
                      ? t("search:resultCard.showFiles")
                      : t("search:resultCard.showItems"),
                    ariaLabel: isFileTag
                      ? t("search:resultCard.showFilesTagged", { name: title })
                      : t("search:resultCard.showItemsTagged", { name: title }),
                    onSelect: () => go(tagListHref(groupSlug, r.attributes)),
                    testId: `palette-tag-list-${r.id}`,
                  }}
                />
              )
            })}
          </CommandGroup>
        ) : null}

        {!trimmedQuery && recent.length > 0 ? (
          <>
            <CommandGroup
              heading={t("common:shell.commandGroupRecent")}
              data-testid="palette-group-recent"
            >
              {recent.map((entry) => (
                <ResultRow
                  key={`${entry.type}:${entry.id}`}
                  value={`${RESULT_PREFIX}recent:${entry.type}:${entry.id}`}
                  icon={RESOURCE_ICON[entry.type] ?? FileText}
                  title={entry.title}
                  subtitle={t(`search:kinds.${entry.type}`)}
                  onSelect={() => openResult(entry)}
                  testId={`palette-recent-${entry.type}-${entry.id}`}
                />
              ))}
            </CommandGroup>
            <CommandSeparator />
          </>
        ) : null}

        <CommandGroup heading={t("common:shell.commandGroupNavigation")}>
          {NAVIGATION.filter(
            (entry) => !entry.section || !isSectionHidden(currentGroup, entry.section)
          ).map((entry) => (
            <PaletteRow
              key={entry.labelKey}
              entry={entry}
              target={entry.to(currentGroup)}
              onSelect={go}
            />
          ))}
        </CommandGroup>
      </CommandList>
    </>
  )
}
