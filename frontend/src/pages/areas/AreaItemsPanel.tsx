import { useEffect, useRef, useState } from "react"
import {
  Link,
  useLocation as useRouterLocation,
  useNavigate,
  useSearchParams,
} from "react-router-dom"
import { useTranslation } from "react-i18next"
import {
  ArrowUpDown,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Eye,
  EyeOff,
  LayoutGrid,
  List,
  ListFilter,
  Package,
  Search,
  ShieldCheck,
  TrendingUp,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import { useCommodities, useCommoditiesValue } from "@/features/commodities/hooks"
import {
  COMMODITY_SORT_OPTIONS,
  COMMODITY_STATUSES,
  COMMODITY_STATUS_TONES,
  COMMODITY_TYPES,
  COMMODITY_TYPE_ICONS,
  COMMODITY_WARRANTY_STATUSES,
  warrantyStatus,
  type CommoditySortOption,
  type CommodityStatusValue,
  type CommodityTypeValue,
  type CommodityWarrantyStatus,
} from "@/features/commodities/constants"
import type { Commodity } from "@/features/commodities/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { formatCurrency } from "@/lib/intl"
import { cn } from "@/lib/utils"

const PER_PAGE = 24
const VIEW_MODE_KEY = "areaItems:viewMode"

type ViewMode = "grid" | "list"

interface AreaItemsPanelProps {
  areaId: string
}

// Per-area items panel: stats strip + URL-driven toolbar (search,
// filters, sort, inactive toggle, view-mode toggle) + grid OR list +
// pagination. Mirrors `design-mocks/src/views/LocationPickerView.tsx`
// Level 3 `<ItemsPanel showStats defaultViewMode="list" />` with the
// area filter pre-bound so the user is always scoped to this area.
//
// URL state ('?q=', '?type=…', '?status=…', '?warranty=…', '?sort=…',
// '?inactive=1', '?view=…', '?page=…') is preserved across refresh /
// back / forward like `CommoditiesListPage`. The area-detail route's
// search params are independent of the global Items list page so the
// two surfaces don't fight over the URL.
export function AreaItemsPanel({ areaId }: AreaItemsPanelProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const enabled = !!currentGroup
  const slug = currentGroup?.slug
  const navigate = useNavigate()
  const listLocation = useRouterLocation()
  const [searchParams, setSearchParams] = useSearchParams()

  // ---- URL → state ------------------------------------------------------
  const page = Math.max(1, Number(searchParams.get("page") ?? "1"))
  const search = searchParams.get("q") ?? ""
  const types = searchParams.getAll("type") as CommodityTypeValue[]
  const statuses = searchParams.getAll("status") as CommodityStatusValue[]
  const warrantyFilter = searchParams.getAll("warranty") as CommodityWarrantyStatus[]
  const includeInactive = searchParams.get("inactive") === "1"
  const sortRaw = searchParams.get("sort") ?? "name"
  const sortDesc = sortRaw.startsWith("-")
  const sortField = (sortDesc ? sortRaw.slice(1) : sortRaw) as CommoditySortOption
  const validSort = COMMODITY_SORT_OPTIONS.includes(sortField) ? sortField : "name"
  const urlView = searchParams.get("view") as ViewMode | null
  const [storedView, setStoredView] = useState<ViewMode>(() => {
    if (typeof window === "undefined") return "list"
    return (localStorage.getItem(VIEW_MODE_KEY) as ViewMode) || "list"
  })
  const viewMode: ViewMode = urlView === "grid" || urlView === "list" ? urlView : storedView

  // ---- Live search input (decoupled from URL via debounce) --------------
  const [searchInput, setSearchInput] = useState(search)
  const setSearchParamsRef = useRef(setSearchParams)
  // eslint-disable-next-line react-hooks/refs -- assigning the latest setter is the well-known "always-current ref" pattern; the ref isn't read during render.
  setSearchParamsRef.current = setSearchParams
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setSearchInput(search), [search])
  useEffect(() => {
    const handle = window.setTimeout(() => {
      const live = new URLSearchParams(window.location.search)
      const trimmed = searchInput.trim()
      if ((live.get("q") ?? "") === trimmed) return
      if (trimmed) {
        live.set("q", trimmed)
      } else {
        live.delete("q")
      }
      live.delete("page")
      setSearchParamsRef.current(live, { replace: true })
    }, 300)
    return () => window.clearTimeout(handle)
  }, [searchInput])

  // ---- Data -------------------------------------------------------------
  const list = useCommodities(
    {
      areaId,
      page,
      perPage: PER_PAGE,
      search,
      types,
      statuses,
      includeInactive,
      sort: validSort,
      sortDesc,
    },
    { enabled: enabled && !!areaId }
  )
  // Estimated value rollup — area totals come from the dedicated summary
  // endpoint keyed by area_id (not name), so multi-area-with-same-name
  // setups still resolve to the right value.
  const values = useCommoditiesValue({ enabled })
  const currency = currentGroup?.group_currency ?? "USD"
  const areaValueEntry = values.data?.areaTotals.find((entry) => entry.id === areaId)
  const valueCell: { value: string; loading: boolean } = values.isLoading
    ? { value: "", loading: true }
    : values.isError || !areaValueEntry
      ? { value: "—", loading: false }
      : { value: formatCurrency(areaValueEntry.value, currency), loading: false }

  // ---- Derived ----------------------------------------------------------
  const allRows = list.data?.commodities ?? []
  // Client-side warranty-bucket filter; same approach as
  // CommoditiesListPage. Switch to server-side `warranty_status=` once
  // both lists consolidate filter state.
  const rows =
    warrantyFilter.length === 0
      ? allRows
      : allRows.filter((r) =>
          warrantyFilter.includes(warrantyStatus({ warranty_expires_at: r.warranty_expires_at }))
        )
  // Total used for stats + pagination. The BE total reflects the
  // server-side filters (search/type/status/inactive/area) but NOT the
  // client-side warranty filter, so we render the BE total as the upper
  // bound and let pagination operate on the full server page.
  const total = list.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE))

  // Active-warranties stat — derived from the SAME page-level fetch
  // that powers the list. On an area with more items than PER_PAGE the
  // count is a partial sample; the cell appends "+" via the same
  // truncation cue the locations list uses so it never silently
  // undercounts.
  const isTruncated = total > allRows.length
  const activeWarrantiesSampled = allRows.filter(
    (r) => warrantyStatus({ warranty_expires_at: r.warranty_expires_at }) === "active"
  ).length
  const activeWarrantiesLabel = list.isLoading
    ? "—"
    : isTruncated
      ? activeWarrantiesSampled >= 1
        ? `${activeWarrantiesSampled}+`
        : "—"
      : String(activeWarrantiesSampled)

  const hasFilters =
    types.length > 0 ||
    statuses.length > 0 ||
    warrantyFilter.length > 0 ||
    search !== "" ||
    includeInactive

  // ---- URL helpers ------------------------------------------------------
  function updateParams(patch: (params: URLSearchParams) => void, opts?: { keepPage?: boolean }) {
    const next = new URLSearchParams(searchParams)
    patch(next)
    if (!opts?.keepPage) next.delete("page")
    setSearchParams(next, { replace: true })
  }
  function toggleType(type: CommodityTypeValue) {
    updateParams((p) => {
      const cur = p.getAll("type")
      p.delete("type")
      const next = cur.includes(type) ? cur.filter((t) => t !== type) : [...cur, type]
      for (const x of next) p.append("type", x)
    })
  }
  function toggleStatus(status: CommodityStatusValue) {
    updateParams((p) => {
      const cur = p.getAll("status")
      p.delete("status")
      const next = cur.includes(status) ? cur.filter((s) => s !== status) : [...cur, status]
      for (const s of next) p.append("status", s)
    })
  }
  function toggleWarranty(status: CommodityWarrantyStatus) {
    updateParams((p) => {
      const cur = p.getAll("warranty")
      p.delete("warranty")
      const next = cur.includes(status) ? cur.filter((s) => s !== status) : [...cur, status]
      for (const s of next) p.append("warranty", s)
    })
  }
  function setSort(field: CommoditySortOption) {
    updateParams((p) => {
      // Toggle direction when the same field is picked; otherwise start
      // dates / prices descending, everything else ascending. Mirrors
      // CommoditiesListPage so users get consistent sort UX.
      const isDateOrPrice = field !== "name" && field !== "count"
      const current = p.get("sort") ?? "name"
      const currentField = current.startsWith("-") ? current.slice(1) : current
      const currentDesc = current.startsWith("-")
      const desc = currentField === field ? !currentDesc : isDateOrPrice
      p.set("sort", desc ? `-${field}` : field)
    })
  }
  function toggleInactive() {
    updateParams((p) => {
      if (includeInactive) p.delete("inactive")
      else p.set("inactive", "1")
    })
  }
  function clearFilters() {
    updateParams((p) => {
      p.delete("type")
      p.delete("status")
      p.delete("warranty")
      p.delete("inactive")
      p.delete("q")
    })
    setSearchInput("")
  }
  function setViewMode(mode: ViewMode) {
    setStoredView(mode)
    if (typeof window !== "undefined") localStorage.setItem(VIEW_MODE_KEY, mode)
    updateParams((p) => p.set("view", mode), { keepPage: true })
  }
  function goToPage(p: number) {
    updateParams((params) => params.set("page", String(p)), { keepPage: true })
  }

  // #1546-style modal-route overlay: click a row to open the
  // CommodityDetailSheet on top of this page. The `state.background`
  // stamp lets the router render the area detail beneath the sheet so
  // closing the sheet returns the user to the SAME panel state instead
  // of a clean reload.
  function openCommodityInSheet(id: string) {
    if (!slug || !id) return
    const next = `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`
    navigate(next, { state: { background: listLocation } })
  }

  return (
    <section className="flex flex-col gap-4" data-testid="area-detail-items">
      {/* Stats strip — 3 cells matching the mock's Level 3 ItemsPanel. */}
      <div className="grid grid-cols-3 gap-3" data-testid="area-detail-items-stats">
        <StatCell
          icon={Package}
          label={t("locations:areaDetail.items.statsItems")}
          value={String(total)}
          loading={list.isLoading}
        />
        <StatCell
          icon={ShieldCheck}
          label={t("locations:areaDetail.items.statsActiveWarranties")}
          value={activeWarrantiesLabel}
          loading={list.isLoading}
          valueClassName="text-status-active"
          iconClassName="text-status-active"
          testId="area-detail-items-active-warranties"
        />
        <StatCell
          icon={TrendingUp}
          label={t("locations:areaDetail.items.statsValue")}
          value={valueCell.value}
          loading={valueCell.loading}
          testId="area-detail-items-value"
        />
      </div>

      <Toolbar
        searchInput={searchInput}
        onSearchInput={setSearchInput}
        types={types}
        statuses={statuses}
        warrantyFilter={warrantyFilter}
        includeInactive={includeInactive}
        sort={validSort}
        sortDesc={sortDesc}
        viewMode={viewMode}
        hasFilters={hasFilters}
        onToggleType={toggleType}
        onToggleStatus={toggleStatus}
        onToggleWarranty={toggleWarranty}
        onSetSort={setSort}
        onToggleInactive={toggleInactive}
        onClearFilters={clearFilters}
        onSetViewMode={setViewMode}
      />

      {list.isError ? (
        <Alert variant="destructive" data-testid="area-detail-items-error">
          <AlertTitle>{t("locations:areaDetail.items.errorTitle")}</AlertTitle>
          <AlertDescription>{t("locations:areaDetail.items.errorDescription")}</AlertDescription>
        </Alert>
      ) : list.isLoading ? (
        <ItemsLoading viewMode={viewMode} />
      ) : rows.length === 0 ? (
        <ItemsEmpty hasFilters={hasFilters} onClear={clearFilters} />
      ) : viewMode === "grid" ? (
        <Grid rows={rows} onPreview={openCommodityInSheet} currency={currency} slug={slug} />
      ) : (
        <List_ rows={rows} onPreview={openCommodityInSheet} currency={currency} slug={slug} />
      )}

      {totalPages > 1 ? (
        <Pagination page={page} totalPages={totalPages} onChange={goToPage} />
      ) : null}
    </section>
  )
}

interface StatCellProps {
  icon: React.ElementType
  label: string
  value: string
  loading?: boolean
  testId?: string
  valueClassName?: string
  iconClassName?: string
}

function StatCell({
  icon: Icon,
  label,
  value,
  loading,
  testId,
  valueClassName,
  iconClassName,
}: StatCellProps) {
  return (
    <div
      className="flex items-center gap-3 rounded-xl border border-border bg-card px-4 py-3"
      data-testid={testId}
    >
      <Icon
        className={cn("size-4 shrink-0 text-muted-foreground", iconClassName)}
        aria-hidden="true"
      />
      <div className="min-w-0">
        {loading ? (
          <Skeleton className="h-4 w-16" />
        ) : (
          <p className={cn("text-sm font-semibold leading-tight", valueClassName)}>{value}</p>
        )}
        <p className="text-xs text-muted-foreground">{label}</p>
      </div>
    </div>
  )
}

interface ToolbarProps {
  searchInput: string
  onSearchInput: (v: string) => void
  types: CommodityTypeValue[]
  statuses: CommodityStatusValue[]
  warrantyFilter: CommodityWarrantyStatus[]
  includeInactive: boolean
  sort: CommoditySortOption
  sortDesc: boolean
  viewMode: ViewMode
  hasFilters: boolean
  onToggleType: (t: CommodityTypeValue) => void
  onToggleStatus: (s: CommodityStatusValue) => void
  onToggleWarranty: (s: CommodityWarrantyStatus) => void
  onSetSort: (f: CommoditySortOption) => void
  onToggleInactive: () => void
  onClearFilters: () => void
  onSetViewMode: (v: ViewMode) => void
}

function Toolbar(props: ToolbarProps) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-wrap items-center gap-2" data-testid="area-detail-items-toolbar">
      <div className="relative min-w-48 max-w-md flex-1">
        <Search
          className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden="true"
        />
        <Input
          type="search"
          placeholder={t("commodities:list.searchPlaceholder")}
          value={props.searchInput}
          onChange={(e) => props.onSearchInput(e.target.value)}
          className="pl-9"
          data-testid="area-detail-items-search"
        />
      </div>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant={props.types.length > 0 ? "default" : "outline"}
            size="sm"
            className="gap-1.5"
            data-testid="area-detail-items-filter-type"
          >
            <ListFilter className="size-3.5" aria-hidden="true" />
            {t("commodities:filter.type")}
            {props.types.length > 0 ? (
              <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
                {props.types.length}
              </Badge>
            ) : null}
            <ChevronDown className="size-3.5" aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-44">
          <DropdownMenuLabel>{t("commodities:filter.type")}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {COMMODITY_TYPES.map((tp) => (
            <DropdownMenuCheckboxItem
              key={tp}
              checked={props.types.includes(tp)}
              onCheckedChange={() => props.onToggleType(tp)}
            >
              <span className="mr-1.5">{COMMODITY_TYPE_ICONS[tp]}</span>
              {t(`commodities:type.${tp}`)}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant={props.statuses.length > 0 ? "default" : "outline"}
            size="sm"
            className="gap-1.5"
            data-testid="area-detail-items-filter-status"
          >
            <ListFilter className="size-3.5" aria-hidden="true" />
            {t("commodities:filter.status")}
            {props.statuses.length > 0 ? (
              <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
                {props.statuses.length}
              </Badge>
            ) : null}
            <ChevronDown className="size-3.5" aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          <DropdownMenuLabel>{t("commodities:filter.status")}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {COMMODITY_STATUSES.map((s) => (
            <DropdownMenuCheckboxItem
              key={s}
              checked={props.statuses.includes(s)}
              onCheckedChange={() => props.onToggleStatus(s)}
            >
              {t(`commodities:status.${s}`)}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant={props.warrantyFilter.length > 0 ? "default" : "outline"}
            size="sm"
            className="gap-1.5"
            data-testid="area-detail-items-filter-warranty"
          >
            <ListFilter className="size-3.5" aria-hidden="true" />
            {t("commodities:filter.warranty")}
            {props.warrantyFilter.length > 0 ? (
              <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
                {props.warrantyFilter.length}
              </Badge>
            ) : null}
            <ChevronDown className="size-3.5" aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-44">
          <DropdownMenuLabel>{t("commodities:filter.warranty")}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {COMMODITY_WARRANTY_STATUSES.map((w) => (
            <DropdownMenuCheckboxItem
              key={w}
              checked={props.warrantyFilter.includes(w)}
              onCheckedChange={() => props.onToggleWarranty(w)}
            >
              {t(`commodities:warranty.${w}`)}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="outline"
            size="sm"
            className="gap-1.5"
            data-testid="area-detail-items-sort"
          >
            <ArrowUpDown className="size-3.5" aria-hidden="true" />
            {t(`commodities:sort.${props.sort}`)}
            {props.sortDesc ? (
              <span aria-hidden="true" className="text-xs">
                ↓
              </span>
            ) : (
              <span aria-hidden="true" className="text-xs">
                ↑
              </span>
            )}
            <ChevronDown className="size-3.5" aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          <DropdownMenuLabel>{t("commodities:sort.label")}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {COMMODITY_SORT_OPTIONS.map((f) => (
            <DropdownMenuCheckboxItem
              key={f}
              checked={props.sort === f}
              onCheckedChange={() => props.onSetSort(f)}
            >
              {t(`commodities:sort.${f}`)}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {props.hasFilters ? (
        <Button
          variant="ghost"
          size="sm"
          onClick={props.onClearFilters}
          data-testid="area-detail-items-clear-filters"
        >
          {t("commodities:filter.clear")}
        </Button>
      ) : null}

      <Button
        variant={props.includeInactive ? "secondary" : "ghost"}
        size="sm"
        className="gap-1.5"
        onClick={props.onToggleInactive}
        data-testid="area-detail-items-toggle-inactive"
      >
        {props.includeInactive ? (
          <Eye className="size-3.5" aria-hidden="true" />
        ) : (
          <EyeOff className="size-3.5" aria-hidden="true" />
        )}
        {props.includeInactive
          ? t("commodities:filter.inactiveShown")
          : t("commodities:filter.inactiveHidden")}
      </Button>

      <div className="ml-auto flex gap-1">
        <Button
          variant={props.viewMode === "grid" ? "secondary" : "ghost"}
          size="icon"
          className="size-8"
          onClick={() => props.onSetViewMode("grid")}
          aria-label={t("commodities:list.viewGrid")}
          data-testid="area-detail-items-view-grid"
        >
          <LayoutGrid className="size-4" aria-hidden="true" />
        </Button>
        <Button
          variant={props.viewMode === "list" ? "secondary" : "ghost"}
          size="icon"
          className="size-8"
          onClick={() => props.onSetViewMode("list")}
          aria-label={t("commodities:list.viewList")}
          data-testid="area-detail-items-view-list"
        >
          <List className="size-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
  )
}

interface RowListProps {
  rows: Commodity[]
  onPreview: (id: string) => void
  currency: string
  slug?: string
}

function List_({ rows, onPreview, currency, slug }: RowListProps) {
  const { t } = useTranslation()
  if (!slug) return null
  return (
    <Card className="overflow-hidden p-0" data-testid="area-detail-items-list">
      <ul>
        {rows
          .filter((row): row is Commodity & { id: string } => Boolean(row.id))
          .map((row, index) => {
            const detailHref = `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(row.id)}`
            const status = row.status as CommodityStatusValue | undefined
            const tone = status ? COMMODITY_STATUS_TONES[status] : ""
            const typeIcon = COMMODITY_TYPE_ICONS[row.type as CommodityTypeValue] ?? "📦"
            const showStatusPill = status !== undefined && status !== "in_use"
            const wStatus = warrantyStatus({ warranty_expires_at: row.warranty_expires_at })
            return (
              <li key={row.id}>
                {index > 0 ? <Separator /> : null}
                <Link
                  to={detailHref}
                  onClick={(e) => {
                    if (
                      e.defaultPrevented ||
                      e.metaKey ||
                      e.ctrlKey ||
                      e.shiftKey ||
                      e.altKey ||
                      e.button !== 0
                    ) {
                      return
                    }
                    e.preventDefault()
                    onPreview(row.id)
                  }}
                  className={cn(
                    "flex w-full items-center gap-4 px-5 py-3.5 text-left transition-colors hover:bg-muted/50",
                    row.draft && "opacity-70"
                  )}
                  data-testid="area-detail-items-row"
                  data-commodity-id={row.id}
                >
                  <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-lg">
                    <span aria-hidden="true">{typeIcon}</span>
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <p className="truncate text-sm font-medium">{row.name}</p>
                      {row.draft ? (
                        <Badge
                          variant="outline"
                          className="h-4 shrink-0 border-dashed px-1 text-[10px] text-muted-foreground"
                        >
                          {t("commodities:list.draftBadge")}
                        </Badge>
                      ) : null}
                    </div>
                    {row.short_name ? (
                      <p className="truncate text-xs text-muted-foreground">{row.short_name}</p>
                    ) : null}
                  </div>
                  {showStatusPill && status ? (
                    <span
                      className={cn(
                        "shrink-0 rounded-full border px-2 py-0.5 text-xs font-medium",
                        tone
                      )}
                    >
                      {t(`commodities:status.${status}`)}
                    </span>
                  ) : (
                    <WarrantyBadge status={wStatus} showIcon={false} className="shrink-0" />
                  )}
                  <p className="hidden w-20 shrink-0 text-right text-sm font-medium sm:block">
                    {formatCurrency(Number(row.current_price ?? 0), currency)}
                  </p>
                </Link>
              </li>
            )
          })}
      </ul>
    </Card>
  )
}

function Grid({ rows, onPreview, currency, slug }: RowListProps) {
  const { t } = useTranslation()
  if (!slug) return null
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" data-testid="area-detail-items-grid">
      {rows
        .filter((row): row is Commodity & { id: string } => Boolean(row.id))
        .map((row) => {
          const detailHref = `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(row.id)}`
          const status = row.status as CommodityStatusValue | undefined
          const tone = status ? COMMODITY_STATUS_TONES[status] : ""
          const typeIcon = COMMODITY_TYPE_ICONS[row.type as CommodityTypeValue] ?? "📦"
          const showStatusPill = status !== undefined && status !== "in_use"
          const wStatus = warrantyStatus({ warranty_expires_at: row.warranty_expires_at })
          return (
            <Link
              key={row.id}
              to={detailHref}
              onClick={(e) => {
                if (
                  e.defaultPrevented ||
                  e.metaKey ||
                  e.ctrlKey ||
                  e.shiftKey ||
                  e.altKey ||
                  e.button !== 0
                ) {
                  return
                }
                e.preventDefault()
                onPreview(row.id)
              }}
              className="block"
              data-testid="area-detail-items-card"
              data-commodity-id={row.id}
            >
              <Card
                className={cn(
                  "gap-3 transition-all hover:-translate-y-0.5 hover:shadow-sm",
                  row.draft && "border-dashed opacity-70"
                )}
              >
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-lg">
                      <span aria-hidden="true">{typeIcon}</span>
                    </div>
                    <div className="flex flex-wrap items-center justify-end gap-1.5">
                      {row.draft ? (
                        <Badge
                          variant="outline"
                          className="h-4 border-dashed px-1.5 text-[10px] text-muted-foreground"
                        >
                          {t("commodities:list.draftBadge")}
                        </Badge>
                      ) : null}
                      {showStatusPill && status ? (
                        <span
                          className={cn(
                            "rounded-full border px-1.5 py-0.5 text-[10px] font-medium",
                            tone
                          )}
                        >
                          {t(`commodities:status.${status}`)}
                        </span>
                      ) : (
                        <WarrantyBadge status={wStatus} showIcon={false} />
                      )}
                    </div>
                  </div>
                  <CardTitle className="mt-2 text-sm font-semibold leading-tight">
                    {row.name}
                  </CardTitle>
                  {row.short_name ? (
                    <p className="truncate text-xs text-muted-foreground">{row.short_name}</p>
                  ) : null}
                </CardHeader>
                <CardContent>
                  <p className="text-right text-sm font-medium">
                    {formatCurrency(Number(row.current_price ?? 0), currency)}
                  </p>
                </CardContent>
              </Card>
            </Link>
          )
        })}
    </div>
  )
}

function ItemsLoading({ viewMode }: { viewMode: ViewMode }) {
  if (viewMode === "list") {
    return (
      <Card className="overflow-hidden p-0" data-testid="area-detail-items-loading">
        <ul>
          {[0, 1, 2].map((i) => (
            <li key={i}>
              {i > 0 ? <Separator /> : null}
              <div className="flex items-center gap-4 px-5 py-3.5">
                <Skeleton className="size-9 shrink-0 rounded-lg" />
                <div className="flex flex-1 flex-col gap-2">
                  <Skeleton className="h-3 w-40" />
                  <Skeleton className="h-3 w-24" />
                </div>
                <Skeleton className="hidden h-4 w-20 sm:block" />
              </div>
            </li>
          ))}
        </ul>
      </Card>
    )
  }
  return (
    <div
      className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
      data-testid="area-detail-items-loading"
    >
      {Array.from({ length: 6 }).map((_, i) => (
        <Card key={i}>
          <CardHeader>
            <Skeleton className="size-9 rounded-lg" />
            <Skeleton className="mt-2 h-4 w-32" />
            <Skeleton className="h-3 w-24" />
          </CardHeader>
          <CardContent>
            <div className="flex justify-end">
              <Skeleton className="h-3 w-16" />
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

interface ItemsEmptyProps {
  hasFilters: boolean
  onClear: () => void
}

function ItemsEmpty({ hasFilters, onClear }: ItemsEmptyProps) {
  const { t } = useTranslation()
  if (hasFilters) {
    return (
      <div
        className="flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16"
        data-testid="area-detail-items-empty-filtered"
      >
        <Package className="size-8 text-muted-foreground/30" aria-hidden="true" />
        <p className="text-sm text-muted-foreground">
          {t("locations:areaDetail.items.emptyFiltered")}
        </p>
        <Button variant="outline" size="sm" onClick={onClear}>
          {t("commodities:filter.clear")}
        </Button>
      </div>
    )
  }
  return (
    <div
      className="flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16"
      data-testid="area-detail-items-empty"
    >
      <Package className="size-8 text-muted-foreground/30" aria-hidden="true" />
      <p className="text-sm text-muted-foreground">{t("locations:areaDetail.items.empty")}</p>
    </div>
  )
}

interface PaginationProps {
  page: number
  totalPages: number
  onChange: (page: number) => void
}

function Pagination({ page, totalPages, onChange }: PaginationProps) {
  const { t } = useTranslation()
  return (
    <div
      className="flex items-center justify-center gap-2"
      data-testid="area-detail-items-pagination"
    >
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={page <= 1}
        onClick={() => onChange(Math.max(1, page - 1))}
        aria-label={t("commodities:pagination.previous")}
        data-testid="area-detail-items-pagination-prev"
      >
        <ChevronLeft className="size-4" aria-hidden="true" />
      </Button>
      <span className="text-sm text-muted-foreground">
        {t("commodities:pagination.pageOf", { page, total: totalPages })}
      </span>
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={page >= totalPages}
        onClick={() => onChange(Math.min(totalPages, page + 1))}
        aria-label={t("commodities:pagination.next")}
        data-testid="area-detail-items-pagination-next"
      >
        <ChevronRight className="size-4" aria-hidden="true" />
      </Button>
    </div>
  )
}
