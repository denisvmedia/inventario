import { useEffect, useMemo, useRef, useState } from "react"
import { Link, useSearchParams } from "react-router-dom"
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
  Plus,
  Search,
  X,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAreas } from "@/features/areas/hooks"
import {
  useBulkDeleteCommodities,
  useBulkMoveCommodities,
  useCommodities,
  useCreateCommodity,
} from "@/features/commodities/hooks"
import {
  COMMODITY_SORT_OPTIONS,
  COMMODITY_STATUSES,
  COMMODITY_STATUS_TONES,
  COMMODITY_TYPES,
  COMMODITY_TYPE_ICONS,
  type CommoditySortOption,
  type CommodityStatusValue,
  type CommodityTypeValue,
} from "@/features/commodities/constants"
import type { Commodity, CreateCommodityRequest } from "@/features/commodities/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatCurrency } from "@/lib/intl"
import { cn } from "@/lib/utils"

const PER_PAGE = 24
const VIEW_MODE_KEY = "commodities:viewMode"

type ViewMode = "grid" | "list"

// /commodities — full Items list with server-side pagination, filter,
// sort, search, bulk select + actions, grid/list toggle, and a
// hide-inactive switch. URL state keeps everything refresh-survivable.
//
// Filter dropdowns are checkbox-multi (Type, Status); area is single-
// select. Sort is a radio dropdown that emits `field` or `-field` for
// the BE. The grid/list toggle persists per-user via localStorage and
// is also URL-syncable for shared links.
//
// Bulk actions: delete + move-to-area, with confirm dialogs and toast.
// Selection state is page-local (cleared when filters/page change) so
// "Select all" never silently picks rows the user can't see.
export function CommoditiesListPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const enabled = !!currentGroup
  const slug = currentGroup?.slug
  const [searchParams, setSearchParams] = useSearchParams()

  // ---- URL → state ------------------------------------------------------
  const page = Math.max(1, Number(searchParams.get("page") ?? "1"))
  const search = searchParams.get("q") ?? ""
  const types = searchParams.getAll("type") as CommodityTypeValue[]
  const statuses = searchParams.getAll("status") as CommodityStatusValue[]
  const areaId = searchParams.get("area") ?? ""
  const includeInactive = searchParams.get("inactive") === "1"
  const sortRaw = searchParams.get("sort") ?? "name"
  const sortDesc = sortRaw.startsWith("-")
  const sortField = (sortDesc ? sortRaw.slice(1) : sortRaw) as CommoditySortOption
  const validSort = COMMODITY_SORT_OPTIONS.includes(sortField) ? sortField : "name"
  const urlView = searchParams.get("view") as ViewMode | null
  const [storedView, setStoredView] = useState<ViewMode>(() => {
    if (typeof window === "undefined") return "grid"
    return (localStorage.getItem(VIEW_MODE_KEY) as ViewMode) || "grid"
  })
  const viewMode: ViewMode = urlView === "grid" || urlView === "list" ? urlView : storedView

  // ---- Live search input (decoupled from URL) ---------------------------
  // Typing pushes the URL on a debounce; the URL still drives the query.
  // This lets us keep the input snappy without firing a refetch on every
  // keystroke.
  //
  // The debounce captures `searchParams` at scheduling time, so a
  // filter / sort / page change fired during the 300ms window would be
  // clobbered when the timeout finally writes back the URL. Read the
  // latest URL directly via a ref instead so the search update merges
  // into whatever is current.
  const [searchInput, setSearchInput] = useState(search)
  const setSearchParamsRef = useRef(setSearchParams)
  setSearchParamsRef.current = setSearchParams
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

  // ---- Data --------------------------------------------------------------
  const list = useCommodities(
    {
      page,
      perPage: PER_PAGE,
      search,
      types,
      statuses,
      areaId: areaId || undefined,
      includeInactive,
      sort: validSort,
      sortDesc,
    },
    { enabled }
  )
  const areas = useAreas({ enabled })

  const bulkDelete = useBulkDeleteCommodities()
  const bulkMove = useBulkMoveCommodities()
  const create = useCreateCommodity()
  const toast = useAppToast()
  const confirm = useConfirm()

  // ---- Selection (page-local) -------------------------------------------
  const [selected, setSelected] = useState<Set<string>>(new Set())
  // Reset selection whenever the visible page changes — the user can't
  // see the rows they had checked, so silently keeping them queued for a
  // bulk action would surprise them.
  const typesKey = types.join(",")
  const statusesKey = statuses.join(",")
  useEffect(() => {
    setSelected(new Set())
  }, [page, search, typesKey, statusesKey, areaId, includeInactive, sortRaw])

  function toggleSelected(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }
  function toggleSelectAll(rows: Commodity[]) {
    setSelected((prev) => {
      if (prev.size === rows.length) return new Set()
      return new Set(rows.map((r) => r.id ?? "").filter(Boolean))
    })
  }

  // ---- Dialogs -----------------------------------------------------------
  const [createOpen, setCreateOpen] = useState(false)
  const [moveOpen, setMoveOpen] = useState(false)
  const [moveTargetArea, setMoveTargetArea] = useState<string>("")
  // ---- Sheet preview overlay -------------------------------------------
  // The mock renders the detail as a slide-over Sheet when reached from
  // the list. Cmd/Ctrl-click on a card still opens the full detail page
  // in a new tab thanks to the `<Link>` underneath; bare click triggers
  // this Sheet instead. Closing the Sheet clears state — no URL hop, so
  // the list page never unmounts.
  const [previewId, setPreviewId] = useState<string | null>(null)

  // ---- URL helpers -------------------------------------------------------
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
      for (const t of next) p.append("type", t)
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
  function setAreaFilter(id: string) {
    updateParams((p) => {
      if (id) p.set("area", id)
      else p.delete("area")
    })
  }
  function setSort(field: CommoditySortOption) {
    updateParams((p) => {
      // Toggle direction when the same field is selected; otherwise
      // start the new field in its natural direction (ascending for
      // name, descending for dates / price — matching what users
      // typically expect).
      const isDateOrPrice = field !== "name" && field !== "count"
      const current = p.get("sort") ?? "name"
      const currentField = current.startsWith("-") ? current.slice(1) : current
      const currentDesc = current.startsWith("-")
      const desc = currentField === field ? !currentDesc : isDateOrPrice
      p.set("sort", desc ? `-${field}` : field)
    })
  }
  function clearFilters() {
    updateParams((p) => {
      p.delete("type")
      p.delete("status")
      p.delete("area")
      p.delete("inactive")
      p.delete("q")
    })
    setSearchInput("")
  }
  function toggleInactive() {
    updateParams((p) => {
      if (includeInactive) p.delete("inactive")
      else p.set("inactive", "1")
    })
  }
  function setViewMode(mode: ViewMode) {
    setStoredView(mode)
    if (typeof window !== "undefined") localStorage.setItem(VIEW_MODE_KEY, mode)
    updateParams((p) => p.set("view", mode), { keepPage: true })
  }
  function goToPage(p: number) {
    updateParams((params) => params.set("page", String(p)), { keepPage: true })
  }

  // ---- Derived -----------------------------------------------------------
  const rows = list.data?.commodities ?? []
  const previewRow = previewId ? (rows.find((r) => r.id === previewId) ?? null) : null
  const total = list.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE))
  const isLoading = list.isLoading
  const isError = list.isError
  const isEmpty = !isLoading && !isError && rows.length === 0
  const hasFilters =
    types.length > 0 || statuses.length > 0 || areaId !== "" || search !== "" || includeInactive

  const areaName = useMemo(() => {
    const map = new Map<string, string>()
    for (const a of areas.data ?? []) {
      if (a.id) map.set(a.id, a.name ?? "")
    }
    return (id?: string) => (id ? (map.get(id) ?? "") : "")
  }, [areas.data])

  // ---- Handlers ----------------------------------------------------------
  async function handleCreate(values: CreateCommodityRequest) {
    await create.mutateAsync(values)
    toast.success(t("commodities:toast.created"))
    setCreateOpen(false)
  }

  async function handleBulkDelete() {
    const ids = [...selected]
    if (ids.length === 0) return
    const ok = await confirm({
      title: t("commodities:bulk.deleteTitle", { count: ids.length }),
      description: t("commodities:bulk.deleteDescription", { count: ids.length }),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (!ok) return
    try {
      await bulkDelete.mutateAsync(ids)
      toast.success(t("commodities:toast.bulkDeleted", { count: ids.length }))
      setSelected(new Set())
    } catch {
      toast.error(t("commodities:toast.bulkDeleteError"))
    }
  }

  async function handleBulkMove() {
    if (!moveTargetArea) return
    const ids = [...selected]
    if (ids.length === 0) return
    try {
      await bulkMove.mutateAsync({ ids, areaId: moveTargetArea })
      toast.success(t("commodities:toast.bulkMoved", { count: ids.length }))
      setSelected(new Set())
      setMoveOpen(false)
      setMoveTargetArea("")
    } catch {
      toast.error(t("commodities:toast.bulkMoveError"))
    }
  }

  return (
    <>
      <RouteTitle title={t("commodities:list.documentTitle")} />
      <div
        className="flex flex-col gap-6 p-6 max-w-6xl mx-auto w-full"
        data-testid="page-commodities"
      >
        <header className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
              {t("commodities:list.heading")}
            </h1>
            <p className="mt-1 text-muted-foreground leading-7">
              {t("commodities:list.subtitle", { count: total })}
            </p>
          </div>
          <Button
            type="button"
            onClick={() => setCreateOpen(true)}
            data-testid="commodities-add-button"
            className="gap-2"
          >
            <Plus className="size-4" aria-hidden="true" />
            {t("commodities:list.addItem")}
          </Button>
        </header>

        <Toolbar
          searchInput={searchInput}
          onSearchInput={setSearchInput}
          types={types}
          statuses={statuses}
          areaId={areaId}
          includeInactive={includeInactive}
          sort={validSort}
          sortDesc={sortDesc}
          viewMode={viewMode}
          areas={areas.data ?? []}
          hasFilters={hasFilters}
          onToggleType={toggleType}
          onToggleStatus={toggleStatus}
          onSetArea={setAreaFilter}
          onSetSort={setSort}
          onToggleInactive={toggleInactive}
          onClearFilters={clearFilters}
          onSetViewMode={setViewMode}
        />

        {selected.size > 0 ? (
          <BulkActionBar
            count={selected.size}
            onClear={() => setSelected(new Set())}
            onDelete={handleBulkDelete}
            onMove={() => setMoveOpen(true)}
            isDeleting={bulkDelete.isPending}
          />
        ) : null}

        {isError ? (
          <Alert variant="destructive" data-testid="commodities-error">
            <AlertTitle>{t("commodities:list.errorTitle")}</AlertTitle>
            <AlertDescription>{t("commodities:list.errorDescription")}</AlertDescription>
          </Alert>
        ) : isLoading ? (
          <ListLoading viewMode={viewMode} />
        ) : isEmpty ? (
          <EmptyState
            hasFilters={hasFilters}
            onClear={clearFilters}
            onAdd={() => setCreateOpen(true)}
          />
        ) : viewMode === "grid" ? (
          <CommoditiesGrid
            rows={rows}
            slug={slug}
            selected={selected}
            onToggleSelected={toggleSelected}
            onPreview={setPreviewId}
            areaName={areaName}
            currency={currentGroup?.main_currency ?? "USD"}
          />
        ) : (
          <CommoditiesTable
            rows={rows}
            slug={slug}
            selected={selected}
            onToggleSelected={toggleSelected}
            onToggleSelectAll={() => toggleSelectAll(rows)}
            onPreview={setPreviewId}
            areaName={areaName}
            currency={currentGroup?.main_currency ?? "USD"}
          />
        )}

        {totalPages > 1 ? (
          <Pagination page={page} totalPages={totalPages} onChange={goToPage} />
        ) : null}
      </div>

      <CommodityFormDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        mode="create"
        areas={areas.data ?? []}
        defaultCurrency={currentGroup?.main_currency ?? "USD"}
        onSubmit={handleCreate}
        isPending={create.isPending}
      />

      <Dialog open={moveOpen} onOpenChange={setMoveOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("commodities:bulk.moveTitle", { count: selected.size })}</DialogTitle>
            <DialogDescription>{t("commodities:bulk.moveDescription")}</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <Label htmlFor="bulk-move-area">{t("commodities:bulk.moveTargetLabel")}</Label>
            <select
              id="bulk-move-area"
              value={moveTargetArea}
              onChange={(e) => setMoveTargetArea(e.target.value)}
              className="border-input bg-transparent rounded-md border px-3 py-2 text-sm"
              data-testid="bulk-move-area"
            >
              <option value="">{t("commodities:bulk.moveTargetPlaceholder")}</option>
              {(areas.data ?? []).map((a) => (
                <option key={a.id} value={a.id ?? ""}>
                  {a.name}
                </option>
              ))}
            </select>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setMoveOpen(false)}>
              {t("common:actions.cancel")}
            </Button>
            <Button
              onClick={handleBulkMove}
              disabled={!moveTargetArea || bulkMove.isPending}
              data-testid="bulk-move-confirm"
            >
              {t("commodities:bulk.moveConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Sheet open={!!previewId} onOpenChange={(open) => !open && setPreviewId(null)}>
        <SheetContent className="sm:max-w-md w-full overflow-y-auto" data-testid="commodity-preview-sheet">
          {previewRow ? (
            <CommodityPreview
              row={previewRow}
              slug={slug}
              areaName={areaName(previewRow.area_id)}
              groupCurrency={currentGroup?.main_currency ?? "USD"}
              onClose={() => setPreviewId(null)}
            />
          ) : null}
        </SheetContent>
      </Sheet>
    </>
  )
}

// ---- Preview Sheet ------------------------------------------------------

interface CommodityPreviewProps {
  row: Commodity
  slug?: string
  areaName: string
  groupCurrency: string
  onClose: () => void
}

// CommodityPreview is the slide-over rendered when a list row is clicked.
// It surfaces the most-likely-needed fields (name, type, area, prices,
// tags, comments) and a "View full details" link to the canonical
// detail page; deeper actions (Edit, Delete, Print) live on the full
// page so the Sheet stays scannable.
function CommodityPreview({
  row,
  slug,
  areaName,
  groupCurrency,
  onClose,
}: CommodityPreviewProps) {
  const { t } = useTranslation()
  const id = row.id ?? ""
  const detailHref =
    slug && id ? `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}` : "#"
  const status = row.status as CommodityStatusValue | undefined
  const tone = status ? COMMODITY_STATUS_TONES[status] : ""
  const type = row.type as CommodityTypeValue | undefined
  const purchaseCurrency = row.original_price_currency ?? groupCurrency
  return (
    <>
      <SheetHeader>
        <SheetTitle className="flex items-center gap-2">
          <span aria-hidden="true" className="text-2xl">
            {type ? COMMODITY_TYPE_ICONS[type] : "📦"}
          </span>
          <span className="truncate">{row.name}</span>
        </SheetTitle>
        <SheetDescription>
          {row.short_name ? `${row.short_name} · ` : ""}
          {type ? t(`commodities:type.${type}`) : ""}
        </SheetDescription>
      </SheetHeader>
      <div className="flex flex-col gap-4 px-4 pb-4">
        <div className="flex flex-wrap items-center gap-1.5">
          {row.draft ? (
            <Badge variant="outline" className="border-dashed">
              {t("commodities:list.draftBadge")}
            </Badge>
          ) : null}
          {status ? (
            <span
              className={cn("text-xs font-medium px-2 py-0.5 rounded-full border", tone)}
            >
              {t(`commodities:status.${status}`)}
            </span>
          ) : null}
        </div>
        <dl className="grid grid-cols-2 gap-3 text-sm">
          <PreviewRow label={t("commodities:detail.fields.area")} value={areaName || "—"} />
          <PreviewRow
            label={t("commodities:detail.fields.count")}
            value={String(row.count ?? "—")}
          />
          <PreviewRow
            label={t("commodities:detail.fields.currentPrice")}
            value={
              row.current_price !== undefined
                ? formatCurrency(Number(row.current_price), groupCurrency)
                : "—"
            }
          />
          <PreviewRow
            label={t("commodities:detail.fields.originalPrice")}
            value={
              row.original_price !== undefined
                ? formatCurrency(Number(row.original_price), purchaseCurrency)
                : "—"
            }
          />
        </dl>
        {row.tags && row.tags.length > 0 ? (
          <div className="flex flex-col gap-1.5">
            <span className="text-xs uppercase tracking-wide text-muted-foreground">
              {t("commodities:detail.fields.tags")}
            </span>
            <div className="flex flex-wrap gap-1.5">
              {row.tags.map((tag) => (
                <Badge key={tag} variant="secondary">
                  {tag}
                </Badge>
              ))}
            </div>
          </div>
        ) : null}
        {row.comments ? (
          <div className="flex flex-col gap-1.5">
            <span className="text-xs uppercase tracking-wide text-muted-foreground">
              {t("commodities:detail.fields.comments")}
            </span>
            <p className="text-sm whitespace-pre-wrap">{row.comments}</p>
          </div>
        ) : null}
      </div>
      <SheetFooter>
        <Button variant="ghost" onClick={onClose}>
          {t("common:actions.cancel")}
        </Button>
        <Button asChild data-testid="commodity-preview-open">
          <Link to={detailHref} onClick={onClose}>
            {t("commodities:list.openFull")}
          </Link>
        </Button>
      </SheetFooter>
    </>
  )
}

function PreviewRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs uppercase tracking-wide text-muted-foreground">{label}</dt>
      <dd className="text-sm">{value}</dd>
    </div>
  )
}

// ---- Toolbar -------------------------------------------------------------

interface ToolbarProps {
  searchInput: string
  onSearchInput: (v: string) => void
  types: CommodityTypeValue[]
  statuses: CommodityStatusValue[]
  areaId: string
  includeInactive: boolean
  sort: CommoditySortOption
  sortDesc: boolean
  viewMode: ViewMode
  areas: { id?: string; name?: string }[]
  hasFilters: boolean
  onToggleType: (t: CommodityTypeValue) => void
  onToggleStatus: (s: CommodityStatusValue) => void
  onSetArea: (id: string) => void
  onSetSort: (f: CommoditySortOption) => void
  onToggleInactive: () => void
  onClearFilters: () => void
  onSetViewMode: (v: ViewMode) => void
}

function Toolbar(props: ToolbarProps) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="relative flex-1 min-w-48 max-w-md">
        <Search
          className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground"
          aria-hidden="true"
        />
        <Input
          type="search"
          placeholder={t("commodities:list.searchPlaceholder")}
          value={props.searchInput}
          onChange={(e) => props.onSearchInput(e.target.value)}
          className="pl-9"
          data-testid="commodities-search"
        />
      </div>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant={props.types.length > 0 ? "default" : "outline"}
            size="sm"
            className="gap-1.5"
            data-testid="commodities-filter-type"
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
            data-testid="commodities-filter-status"
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
            variant={props.areaId ? "default" : "outline"}
            size="sm"
            className="gap-1.5"
            data-testid="commodities-filter-area"
          >
            <ListFilter className="size-3.5" aria-hidden="true" />
            {t("commodities:filter.area")}
            <ChevronDown className="size-3.5" aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-56">
          <DropdownMenuLabel>{t("commodities:filter.area")}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuRadioGroup value={props.areaId} onValueChange={props.onSetArea}>
            <DropdownMenuRadioItem value="">
              {t("commodities:filter.areaAll")}
            </DropdownMenuRadioItem>
            {props.areas.map((a) => (
              <DropdownMenuRadioItem key={a.id} value={a.id ?? ""}>
                {a.name}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuContent>
      </DropdownMenu>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm" className="gap-1.5" data-testid="commodities-sort">
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
          data-testid="commodities-clear-filters"
        >
          {t("commodities:filter.clear")}
        </Button>
      ) : null}

      <Button
        variant={props.includeInactive ? "secondary" : "ghost"}
        size="sm"
        className="gap-1.5"
        onClick={props.onToggleInactive}
        data-testid="commodities-toggle-inactive"
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
          data-testid="commodities-view-grid"
        >
          <LayoutGrid className="size-4" aria-hidden="true" />
        </Button>
        <Button
          variant={props.viewMode === "list" ? "secondary" : "ghost"}
          size="icon"
          className="size-8"
          onClick={() => props.onSetViewMode("list")}
          aria-label={t("commodities:list.viewList")}
          data-testid="commodities-view-list"
        >
          <List className="size-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
  )
}

// ---- Bulk action bar ----------------------------------------------------

interface BulkActionBarProps {
  count: number
  onClear: () => void
  onDelete: () => void
  onMove: () => void
  isDeleting: boolean
}

function BulkActionBar({ count, onClear, onDelete, onMove, isDeleting }: BulkActionBarProps) {
  const { t } = useTranslation()
  return (
    <div
      className="flex flex-wrap items-center gap-2 rounded-md border border-primary/30 bg-primary/5 px-3 py-2"
      role="region"
      aria-label={t("commodities:bulk.regionLabel")}
      data-testid="commodities-bulk-bar"
    >
      <span className="text-sm font-medium">{t("commodities:bulk.selected", { count })}</span>
      <Separator orientation="vertical" className="h-4" />
      <Button variant="outline" size="sm" onClick={onMove} data-testid="commodities-bulk-move">
        {t("commodities:bulk.moveButton")}
      </Button>
      <Button
        variant="outline"
        size="sm"
        onClick={onDelete}
        disabled={isDeleting}
        data-testid="commodities-bulk-delete"
      >
        {t("commodities:bulk.deleteButton")}
      </Button>
      <Button variant="ghost" size="sm" onClick={onClear} className="ml-auto gap-1">
        <X className="size-3.5" aria-hidden="true" />
        {t("common:actions.cancel")}
      </Button>
    </div>
  )
}

// ---- Loading / Empty ----------------------------------------------------

function ListLoading({ viewMode }: { viewMode: ViewMode }) {
  if (viewMode === "list") {
    return (
      <Card className="overflow-hidden p-0" data-testid="commodities-loading">
        <ul>
          {Array.from({ length: 5 }).map((_, i) => (
            <li key={i}>
              {i > 0 ? <Separator /> : null}
              <div className="flex items-center gap-3 px-5 py-3.5">
                <Skeleton className="size-9 rounded-lg" />
                <div className="flex-1 space-y-1.5">
                  <Skeleton className="h-3 w-40" />
                  <Skeleton className="h-3 w-24" />
                </div>
                <Skeleton className="h-3 w-20" />
              </div>
            </li>
          ))}
        </ul>
      </Card>
    )
  }
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" data-testid="commodities-loading">
      {Array.from({ length: 6 }).map((_, i) => (
        <Card key={i}>
          <CardHeader>
            <Skeleton className="size-9 rounded-lg" />
            <Skeleton className="mt-2 h-4 w-32" />
            <Skeleton className="h-3 w-24" />
          </CardHeader>
          <CardContent>
            <div className="flex justify-between">
              <Skeleton className="h-3 w-20" />
              <Skeleton className="h-3 w-12" />
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

interface EmptyStateProps {
  hasFilters: boolean
  onClear: () => void
  onAdd: () => void
}

function EmptyState({ hasFilters, onClear, onAdd }: EmptyStateProps) {
  const { t } = useTranslation()
  if (hasFilters) {
    return (
      <Card data-testid="commodities-empty-filtered">
        <CardHeader>
          <CardTitle>{t("commodities:list.emptyFilteredTitle")}</CardTitle>
          <CardDescription>{t("commodities:list.emptyFilteredDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="outline" onClick={onClear}>
            {t("commodities:filter.clear")}
          </Button>
        </CardContent>
      </Card>
    )
  }
  return (
    <Card data-testid="commodities-empty">
      <CardHeader>
        <CardTitle>{t("commodities:list.emptyTitle")}</CardTitle>
        <CardDescription>{t("commodities:list.emptyDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <Button onClick={onAdd} className="gap-2">
          <Plus className="size-4" aria-hidden="true" />
          {t("commodities:list.addItem")}
        </Button>
      </CardContent>
    </Card>
  )
}

// ---- Grid + Table -------------------------------------------------------

interface CommoditiesGridProps {
  rows: Commodity[]
  slug?: string
  selected: Set<string>
  onToggleSelected: (id: string) => void
  onPreview: (id: string) => void
  areaName: (id?: string) => string
  currency: string
}

function CommoditiesGrid({
  rows,
  slug,
  selected,
  onToggleSelected,
  onPreview,
  areaName,
  currency,
}: CommoditiesGridProps) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" data-testid="commodities-grid">
      {rows.map((row) => (
        <CommodityGridCard
          key={row.id}
          row={row}
          slug={slug}
          selected={selected.has(row.id ?? "")}
          onToggleSelected={onToggleSelected}
          onPreview={onPreview}
          areaName={areaName}
          currency={currency}
        />
      ))}
    </div>
  )
}

interface CommodityCardProps {
  row: Commodity
  slug?: string
  selected: boolean
  onToggleSelected: (id: string) => void
  onPreview: (id: string) => void
  areaName: (id?: string) => string
  currency: string
}

function CommodityGridCard({
  row,
  slug,
  selected,
  onToggleSelected,
  onPreview,
  areaName,
  currency,
}: CommodityCardProps) {
  const { t } = useTranslation()
  const id = row.id ?? ""
  const detailHref =
    slug && id ? `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}` : "#"
  const status = row.status as CommodityStatusValue | undefined
  const tone = status ? COMMODITY_STATUS_TONES[status] : ""
  const draftLabel = t("commodities:list.draftBadge")
  const statusLabel = status ? t(`commodities:status.${status}`) : ""
  // Bare click on the title opens the Sheet preview; ctrl/cmd-click and
  // middle-click fall through to the underlying Link so the user can
  // open the canonical URL in a new tab. shiftKey/aux-button check
  // mirrors react-router's own DOM-link guard.
  function handleTitleClick(e: React.MouseEvent<HTMLAnchorElement>) {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return
    e.preventDefault()
    onPreview(id)
  }
  return (
    <Card
      className={cn(
        "gap-3 transition-all hover:shadow-md",
        row.draft && "opacity-70 border-dashed"
      )}
      data-testid="commodity-card"
      data-commodity-id={id}
    >
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <div className="flex items-center gap-2">
            <Checkbox
              checked={selected}
              onCheckedChange={() => onToggleSelected(id)}
              aria-label={`select ${row.name ?? ""}`}
              data-testid="commodity-select"
            />
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-lg">
              {row.type ? COMMODITY_TYPE_ICONS[row.type as CommodityTypeValue] : "📦"}
            </div>
          </div>
          <div className="flex items-center gap-1.5 flex-wrap justify-end">
            {row.draft ? (
              <Badge variant="outline" className="text-[10px] h-4 px-1 border-dashed">
                {draftLabel}
              </Badge>
            ) : null}
            {status && status !== "in_use" ? (
              <span
                className={cn("text-[10px] font-medium px-1.5 py-0.5 rounded-full border", tone)}
              >
                {statusLabel}
              </span>
            ) : null}
          </div>
        </div>
        <CardTitle className="mt-2 text-sm font-semibold leading-tight">
          <Link
            to={detailHref}
            className="hover:underline"
            onClick={handleTitleClick}
            data-testid="commodity-card-link"
          >
            {row.name}
          </Link>
        </CardTitle>
        <CardDescription className="text-xs">{areaName(row.area_id)}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>{row.short_name || ""}</span>
          <span className="font-medium text-foreground">
            {formatCurrency(Number(row.current_price ?? 0), currency)}
          </span>
        </div>
        {row.tags && row.tags.length > 0 ? (
          <div className="mt-2 flex flex-wrap gap-1">
            {row.tags.slice(0, 3).map((tag) => (
              <Badge key={tag} variant="secondary" className="h-4 px-1.5 text-[10px]">
                {tag}
              </Badge>
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

interface CommoditiesTableProps extends CommoditiesGridProps {
  onToggleSelectAll: () => void
}

function CommoditiesTable({
  rows,
  slug,
  selected,
  onToggleSelected,
  onToggleSelectAll,
  onPreview,
  areaName,
  currency,
}: CommoditiesTableProps) {
  const { t } = useTranslation()
  const allSelected = rows.length > 0 && rows.every((r) => selected.has(r.id ?? ""))
  function handleRowClick(id: string, e: React.MouseEvent<HTMLAnchorElement>) {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return
    e.preventDefault()
    onPreview(id)
  }
  return (
    <Card className="overflow-hidden p-0" data-testid="commodities-table">
      <div className="flex items-center gap-3 border-b border-border px-5 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        <Checkbox
          checked={allSelected}
          onCheckedChange={onToggleSelectAll}
          aria-label={t("commodities:list.selectAll")}
        />
        <span className="flex-1">{t("commodities:list.headerName")}</span>
        <span className="hidden sm:block w-32">{t("commodities:list.headerArea")}</span>
        <span className="hidden sm:block w-24">{t("commodities:list.headerStatus")}</span>
        <span className="w-24 text-right">{t("commodities:list.headerValue")}</span>
      </div>
      <ul>
        {rows.map((row, i) => {
          const id = row.id ?? ""
          const detailHref =
            slug && id
              ? `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`
              : "#"
          const status = row.status as CommodityStatusValue | undefined
          const tone = status ? COMMODITY_STATUS_TONES[status] : ""
          return (
            <li key={id} data-testid="commodity-row">
              {i > 0 ? <Separator /> : null}
              <div
                className={cn(
                  "flex items-center gap-3 px-5 py-3 transition-colors hover:bg-muted/50",
                  row.draft && "opacity-70"
                )}
              >
                <Checkbox
                  checked={selected.has(id)}
                  onCheckedChange={() => onToggleSelected(id)}
                  aria-label={`select ${row.name ?? ""}`}
                />
                <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-lg">
                  {row.type ? COMMODITY_TYPE_ICONS[row.type as CommodityTypeValue] : "📦"}
                </div>
                <div className="flex-1 min-w-0">
                  <Link
                    to={detailHref}
                    className="text-sm font-medium hover:underline truncate"
                    onClick={(e) => handleRowClick(id, e)}
                  >
                    {row.name}
                  </Link>
                  {row.short_name ? (
                    <p className="text-xs text-muted-foreground truncate">{row.short_name}</p>
                  ) : null}
                </div>
                <span className="hidden sm:block w-32 truncate text-xs text-muted-foreground">
                  {areaName(row.area_id)}
                </span>
                <span className="hidden sm:block w-24">
                  {status ? (
                    <span
                      className={cn(
                        "text-xs font-medium px-2 py-0.5 rounded-full border inline-block",
                        tone
                      )}
                    >
                      {t(`commodities:status.${status}`)}
                    </span>
                  ) : null}
                </span>
                <span className="w-24 text-right text-sm font-medium">
                  {formatCurrency(Number(row.current_price ?? 0), currency)}
                </span>
              </div>
            </li>
          )
        })}
      </ul>
    </Card>
  )
}

// ---- Pagination ---------------------------------------------------------

interface PaginationProps {
  page: number
  totalPages: number
  onChange: (page: number) => void
}

function Pagination({ page, totalPages, onChange }: PaginationProps) {
  const { t } = useTranslation()
  const pages = pageRange(page, totalPages)
  return (
    <nav
      className="flex items-center justify-center gap-1"
      aria-label={t("commodities:pagination.label")}
      data-testid="commodities-pagination"
    >
      <Button
        variant="ghost"
        size="icon"
        className="size-8"
        onClick={() => onChange(page - 1)}
        disabled={page <= 1}
        aria-label={t("commodities:pagination.previous")}
      >
        <ChevronLeft className="size-4" aria-hidden="true" />
      </Button>
      {pages.map((p, i) =>
        p === "ellipsis" ? (
          <span key={`e-${i}`} className="px-2 text-sm text-muted-foreground" aria-hidden="true">
            …
          </span>
        ) : (
          <Button
            key={p}
            variant={p === page ? "secondary" : "ghost"}
            size="sm"
            className="size-8"
            onClick={() => onChange(p)}
            aria-current={p === page ? "page" : undefined}
            data-testid={`pagination-page-${p}`}
          >
            {p}
          </Button>
        )
      )}
      <Button
        variant="ghost"
        size="icon"
        className="size-8"
        onClick={() => onChange(page + 1)}
        disabled={page >= totalPages}
        aria-label={t("commodities:pagination.next")}
      >
        <ChevronRight className="size-4" aria-hidden="true" />
      </Button>
    </nav>
  )
}

// pageRange returns the page numbers to render plus "ellipsis" markers.
// Always includes 1 and totalPages; collapses the middle when there are
// gaps. Caller treats "ellipsis" as a non-clickable separator.
function pageRange(current: number, total: number): Array<number | "ellipsis"> {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1)
  }
  const out: Array<number | "ellipsis"> = [1]
  const start = Math.max(2, current - 1)
  const end = Math.min(total - 1, current + 1)
  if (start > 2) out.push("ellipsis")
  for (let p = start; p <= end; p++) out.push(p)
  if (end < total - 1) out.push("ellipsis")
  out.push(total)
  return out
}
