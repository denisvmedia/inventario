import { useCallback, useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react"
import { Link, useLocation, useNavigate, useSearchParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import {
  ArrowUpDown,
  Calendar,
  ChevronDown,
  Eye,
  EyeOff,
  FileUp,
  LayoutGrid,
  LayoutList,
  List,
  ListFilter,
  MapPinOff,
  Plus,
  Search,
  type LucideIcon,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Page, PageHeader } from "@/components/ui/page"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { CommodityBulkActions } from "@/components/items/CommodityBulkActions"
import {
  CommodityPhotoCard,
  CommodityPhotoGrid,
  CommodityPhotoGridSkeleton,
  type CommodityCardProps,
} from "@/components/items/CommodityPhotoCard"
import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { InboxNotice } from "@/components/items/InboxNotice"
import { IntakeCard } from "@/components/items/IntakeCard"
import { IntakeUploadDialog } from "@/components/items/IntakeUploadDialog"
import { SearchMatchHint } from "@/components/items/SearchMatchHint"
import { TagFilterMenu } from "@/components/tags/TagFilterMenu"
import { TagPill } from "@/components/tags/TagPill"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import { useAreas } from "@/features/areas/hooks"
import { useLocations } from "@/features/locations/hooks"
import { brandModelText } from "@/features/commodities/brand"
import { CommodityThumb } from "@/features/commodities/CommodityThumb"
import { formatPrice } from "@/features/commodities/price"
import { useCommodities, useCommodityTagCounts } from "@/features/commodities/hooks"
import { useSubmitNewItem, type DraftRef } from "@/features/drafts/hooks"
import {
  COMMODITY_SORT_OPTIONS,
  COMMODITY_STATUSES,
  COMMODITY_STATUS_TONES,
  COMMODITY_WARRANTY_STATUSES,
  type CommoditySortOption,
  type CommodityStatusValue,
  type CommodityWarrantyStatus,
} from "@/features/commodities/constants"
import {
  useItemTypeFilterOptions,
  useItemTypeLookup,
  type ItemTypeDisplay,
} from "@/features/itemTypes/hooks"
import type { Commodity, CreateCommodityRequest } from "@/features/commodities/api"
import {
  DEFAULT_COMMODITY_QUERY,
  hasCommodityFilters,
  parseCommodityQuery,
  primarySearchMatch,
  unreadableTagParamCount,
  withIncludedTag,
  withTagFilter,
  writeCommodityQuery,
  type CommodityQuery,
  type CommoditySearchMatch,
  type CommodityTagFilter,
} from "@/features/commodities/query"
import {
  COMMODITIES_PAGE_SIZE,
  COMMODITIES_PAGE_SIZES,
  COMMODITIES_PAGE_SIZE_PARAM,
  commodityHref,
  commoditiesListPath,
  commoditiesListOptions,
  parsePageSize,
  parseCommoditiesListParams,
  storeCommoditiesPageSize,
  type CommoditiesPageSize,
} from "@/features/commodities/listContext"
import { useCommoditySelection } from "@/features/commodities/selection"
import { COMMODITIES_VIEW_MODE_KEY } from "@/features/commodities/viewMode"
import { useGroupMigrationLock } from "@/features/currency-migration/lock"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useMyGroupRole } from "@/features/group/roles"
import type { IntakeSessionDetail } from "@/features/intake/api"
import { useIntakeLimits } from "@/features/intake/hooks"
import { useFileDrop } from "@/features/intake/useFileDrop"
import { useUserSettings } from "@/features/settings/hooks"
import { useLoanCounts } from "@/features/loans/hooks"
import { useServiceCounts } from "@/features/services/hooks"
import type { Seller } from "@/features/sellers/api"
import { useSellers } from "@/features/sellers/hooks"
import { useTagIndex, type ResolvedTag } from "@/features/tags/hooks"
import {
  SavedViewControl,
  SavedViewQueryErrorAlert,
  SavedViewUnavailableAlert,
  useOpenedSavedView,
} from "@/features/saved-views/SavedViewControl"
import { useAppToast } from "@/hooks/useAppToast"
import { formatCurrency, formatDate } from "@/lib/intl"
import { listContextFromLocation, type ListContext } from "@/lib/list-context"
import { getServerErrorCode, getServerErrorMeta } from "@/lib/server-error"
import { cn } from "@/lib/utils"
import { Pagination } from "@/components/common/Pagination"

// "grid" is the photo card grid, "compact" the cards with a small
// thumbnail, "list" the table.
type ViewMode = "grid" | "compact" | "list"

function isViewMode(value: unknown): value is ViewMode {
  return value === "grid" || value === "compact" || value === "list"
}

// /commodities — full Items list with server-side pagination, filter,
// sort, search, bulk select + actions, a photo cards / compact cards / list
// toggle, a page size choice, and a hide-inactive switch. URL state keeps
// everything refresh-survivable.
//
// Filter dropdowns are checkbox-multi (Type, Status); area is single-
// select. Sort is a radio dropdown that emits `field` or `-field` for
// the BE. The layout toggle and the page size persist per device via
// localStorage and are also URL-syncable for shared links.
//
// Bulk actions (CommodityBulkActions): move, edit tags, type and status,
// print labels and delete, on the picked rows or on every item the query
// matches. A new query clears the selection.
export function CommoditiesListPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const enabled = !!currentGroup
  const slug = currentGroup?.slug
  const migrationLock = useGroupMigrationLock()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  // `useLocation` is captured here (not inside the row click handler)
  // so each navigation pushes the SAME backdrop URL — i.e. the list
  // location at the moment the user opened the row. The router
  // re-renders on tab/filter changes, so this hook re-runs and the
  // closure always carries the freshest list URL.
  const listLocation = useLocation()

  // ---- URL → state ------------------------------------------------------
  // The filter half of the URL is the item query contract
  // (features/commodities/query). The item page reads the same URL through
  // the same parser when it walks this list (features/commodities/listContext),
  // so both share the cache.
  const listParams = useMemo(() => parseCommoditiesListParams(searchParams), [searchParams])
  const { page, perPage, query } = listParams
  const search = query.text
  const types = query.types
  const statuses = query.statuses as CommodityStatusValue[]
  const areaId = query.areaId
  const warrantyFilter = query.warrantyStatuses as CommodityWarrantyStatus[]
  const lentOutOnly = query.lentOut
  const unassignedOnly = query.unassigned
  const includeInactive = query.includeInactive
  const validSort = query.sort
  const sortDesc = query.sortDesc
  // One string per distinct query, for effects that react to any change.
  const queryKey = writeCommodityQuery(new URLSearchParams(), query).toString()
  // Items opened from here carry this list in `?from=`, for Back and for
  // Previous / Next on the item page.
  const listContext = listContextFromLocation(listLocation)
  const urlView = searchParams.get("view")
  // Local override remembers a toolbar flip per-device. `null` means
  // "this device has never explicitly chosen" — in that case we fall
  // back to the user's Preferences default
  // (`appearance.default_items_view`) once it loads. Setting via the
  // toolbar writes to localStorage but NOT to user_preferences, so a
  // toolbar flip stays scoped to one device. Changing the setting in
  // Settings drops the flip on the device it is changed on.
  const [storedView, setStoredView] = useState<ViewMode | null>(() => {
    if (typeof window === "undefined") return null
    const raw = localStorage.getItem(COMMODITIES_VIEW_MODE_KEY)
    return isViewMode(raw) ? raw : null
  })
  const settingsQuery = useUserSettings()
  // Validate the server value the same way we validate `urlView` and
  // localStorage — `models.SettingsObject.appearanceDefaultItemsView`
  // is typed as `string | undefined` on the wire, so a misbehaving BE
  // (or stale row from a future enum) won't end up as `viewMode` and
  // break the toolbar/list rendering downstream.
  const rawPreferencesDefault = settingsQuery.data?.appearanceDefaultItemsView
  const preferencesDefault: ViewMode | null = isViewMode(rawPreferencesDefault)
    ? rawPreferencesDefault
    : null
  const viewMode: ViewMode = isViewMode(urlView)
    ? urlView
    : (storedView ?? preferencesDefault ?? "grid")

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
  // eslint-disable-next-line react-hooks/refs -- assigning the latest setter is the well-known "always-current ref" pattern; the ref isn't read during render below.
  setSearchParamsRef.current = setSearchParams
  // The size remembered on this device applies at page 1 of a URL without
  // one (features/commodities/listContext). Writing it into the URL keeps the
  // page numbers, and the list context of an item opened from here, the
  // same on any device and in any tab.
  const urlPageSize = parsePageSize(searchParams.get(COMMODITIES_PAGE_SIZE_PARAM))
  useEffect(() => {
    if (urlPageSize !== undefined || perPage === COMMODITIES_PAGE_SIZE) return
    // As the backdrop of an open item the list must not navigate.
    if (!slug || window.location.pathname !== commoditiesListPath(slug)) return
    setSearchParamsRef.current(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set(COMMODITIES_PAGE_SIZE_PARAM, String(perPage))
        return next
      },
      { replace: true }
    )
  }, [urlPageSize, perPage, slug])
  // Re-seed input when URL search param changes (back/forward).
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setSearchInput(search), [search])
  useEffect(() => {
    const trimmed = searchInput.trim()
    // Until the user types, the input holds the list's own query and there
    // is nothing to write. Writing it anyway breaks an item opened within
    // the 300ms: by then window.location is the item's URL, so the write
    // navigates back to the list and closes the item's panel.
    if (trimmed === search) return
    const handle = window.setTimeout(() => {
      const live = new URLSearchParams(window.location.search)
      const liveQuery = parseCommodityQuery(live)
      if (liveQuery.text === trimmed) return
      const next = writeCommodityQuery(live, { ...liveQuery, text: trimmed })
      next.delete("page")
      setSearchParamsRef.current(next, { replace: true })
    }, 300)
    return () => window.clearTimeout(handle)
  }, [searchInput, search])

  // ---- Data --------------------------------------------------------------
  // Every filter is applied by the server, so the page, the total and
  // the pager describe the same set (#2128). includeInactive is always
  // sent, so the server applies the active-only default.
  const listOptions = commoditiesListOptions(listParams)
  // A list opened from a saved view whose stored query cannot run does not
  // run (see useOpenedSavedView); until the views load, or while they fail
  // to, it cannot tell.
  const openedView = useOpenedSavedView()
  const listEnabled = enabled && !openedView.pending && !openedView.unknown && !openedView.broken
  const list = useCommodities(listOptions, { enabled: listEnabled })
  // Tag counts load when the Tags menu opens, for the rest of the query.
  const [tagMenuOpen, setTagMenuOpen] = useState(false)
  const tagCounts = useCommodityTagCounts(listOptions, { enabled: listEnabled && tagMenuOpen })
  const areas = useAreas({ enabled })
  const sellers = useSellers()
  // Locations populate the create-dialog's Location → Area paired
  // selects. The schema only carries area_id; location_id is UI-only.
  const locationsQuery = useLocations({ enabled })

  const create = useSubmitNewItem()
  const toast = useAppToast()

  // ---- Batch intake (#2810) ----------------------------------------------
  // Files dropped on the page, or picked with "Add from files", go to the
  // intake dialog, which uploads them and queues them for the AI scan.
  const role = useMyGroupRole()
  const canWrite = role === "user" || role === "admin" || role === "owner"
  const intakeLimits = useIntakeLimits(enabled && canWrite)
  const intakeEnabled = canWrite && !migrationLock.locked && !!intakeLimits.data?.enabled
  const [intakeOpen, setIntakeOpen] = useState(false)
  const [intakeFiles, setIntakeFiles] = useState<File[]>([])
  const drop = useFileDrop(intakeEnabled && !intakeOpen, (files) => {
    setIntakeFiles(files)
    setIntakeOpen(true)
  })
  function openIntake() {
    setIntakeFiles([])
    setIntakeOpen(true)
  }
  function handleIntakeQueued(session: IntakeSessionDetail) {
    setIntakeOpen(false)
    toast.success(t("commodities:intake.upload.queued", { count: session.summary.sources }))
    if (slug) {
      navigate(`/g/${encodeURIComponent(slug)}/intake/${encodeURIComponent(session.id)}`)
    }
  }

  // ---- Selection ---------------------------------------------------------
  // Picked rows, or every item the query matches ("Select all N matching").
  // A new query clears it, and so does a new page or page size unless every
  // matching item is selected; see features/commodities/selection.
  const selection = useCommoditySelection(queryKey, `${page}/${perPage}`)

  // ---- Dialogs -----------------------------------------------------------
  const [createOpen, setCreateOpen] = useState(false)
  // /g/:slug/commodities/new mounts this same list page and is
  // expected to auto-open the create dialog (router comment, AppSidebar
  // Add-item entry, Dashboard mobile CTA all point here). Detect the
  // pathname suffix and flip the dialog open. The matching close
  // handler (`handleCreateOpenChange` below) navigates back to
  // /commodities so the URL doesn't stay pinned to /new after the
  // dialog dismisses without a successful submit.
  const isCreateRoute = listLocation.pathname.endsWith("/commodities/new")
  // /commodities/new?draft=<id> opens a server draft to finish it (#2812).
  const resumeDraftId = isCreateRoute ? new URLSearchParams(listLocation.search).get("draft") : null
  useEffect(() => {
    // Mirror the disabled state of the toolbar's Add button: while a
    // currency migration is running on this group, commodity writes
    // are off-limits, so the URL-driven auto-open path also bows out
    // and bounces back to /commodities. Without this gate the locked
    // user could still reach the create dialog by typing /new in the
    // address bar — the toolbar Add is disabled, but the URL side-
    // effect is the same write affordance.
    if (isCreateRoute && migrationLock.locked && slug) {
      // Close the dialog BEFORE navigating away so the redirect doesn't
      // leave `createOpen=true` while the URL is back at /commodities.
      // Without this, a user who deep-linked to /new after a migration
      // started would see the dialog flash open on the redirected URL.
      // eslint-disable-next-line react-hooks/set-state-in-effect -- mirroring the open-side branch below: the URL is the authority, and we have to flip local state to match.
      setCreateOpen(false)
      navigate(`/g/${encodeURIComponent(slug)}/commodities`, { replace: true })
      return
    }
    // Two-way URL→dialog sync. The open branch handles direct hits to
    // /commodities/new and entry from the modal-overlay route. The
    // close branch covers the reverse: user hits browser Back from
    // /new → /commodities, the route changes away but `createOpen`
    // would otherwise stay true (the dialog only flips itself off via
    // its own onOpenChange). Closing here keeps history navigation
    // in lockstep with the dialog's visible state. The rule's
    // pure-derived-state heuristic accepts this shape (set-from-prop
    // sync), so no disable directive needed.
    setCreateOpen(isCreateRoute)
  }, [isCreateRoute, migrationLock.locked, slug, navigate])
  // ---- Row → Sheet overlay -------------------------------------------
  // #1546 modal-routes pattern. A bare row click navigates to
  // /commodities/:id and stamps the current list URL onto
  // `state.background`. The router (see app/router.tsx) reads that
  // state and renders TWO trees: this list as the backdrop + the
  // CommodityDetailSheet on top. The URL update is what makes back /
  // forward / `?tab=` deep-links work for the in-session drill-in.
  // Cmd/Ctrl/Shift-click + middle-button still fall through to the
  // underlying `<Link>` so opening in a new tab gets the full page
  // (the new tab carries no `state.background`, so the modal tree
  // stays unmounted there).
  function openCommodityInSheet(id: string) {
    if (!slug || !id) return
    navigate(commodityHref(slug, id, listContext), { state: { background: listLocation } })
  }

  // ---- URL helpers -------------------------------------------------------
  function updateParams(patch: (params: URLSearchParams) => void, opts?: { keepPage?: boolean }) {
    const next = new URLSearchParams(searchParams)
    patch(next)
    if (!opts?.keepPage) next.delete("page")
    setSearchParams(next, { replace: true })
  }

  // Changes part of the query; any change of filter or sort goes back to
  // the first page. Parameters the query does not own stay as they are.
  function updateQuery(patch: Partial<CommodityQuery>) {
    const next = writeCommodityQuery(searchParams, { ...query, ...patch })
    next.delete("page")
    setSearchParams(next, { replace: true })
  }

  function toggled<T>(values: T[], value: T): T[] {
    return values.includes(value) ? values.filter((v) => v !== value) : [...values, value]
  }

  function toggleType(type: string) {
    updateQuery({ types: toggled(query.types, type) })
  }
  function toggleStatus(status: CommodityStatusValue) {
    updateQuery({ statuses: toggled(query.statuses, status) })
  }
  function setAreaFilter(id: string) {
    updateQuery({ areaId: id })
  }
  function toggleSeller(id: string) {
    updateQuery({ sellerIds: toggled(query.sellerIds, id) })
  }
  function toggleWarranty(status: CommodityWarrantyStatus) {
    updateQuery({ warrantyStatuses: toggled(query.warrantyStatuses, status) })
  }
  function setSort(field: CommoditySortOption) {
    // Toggle direction when the same field is selected; otherwise
    // start the new field in its natural direction (ascending for
    // name, descending for dates / price — matching what users
    // typically expect).
    const isDateOrPrice = field !== "name" && field !== "count"
    const desc = query.sort === field ? !query.sortDesc : isDateOrPrice
    updateQuery({ sort: field, sortDesc: desc })
  }
  function clearFilters() {
    // The sort is not a filter and survives.
    updateQuery({ ...DEFAULT_COMMODITY_QUERY, sort: query.sort, sortDesc: query.sortDesc })
    setSearchInput("")
  }
  function toggleInactive() {
    updateQuery({ includeInactive: !includeInactive })
  }
  function toggleLentOut() {
    updateQuery({ lentOut: !lentOutOnly })
  }
  function toggleMissing(key: MissingFilter) {
    updateQuery({ [key]: !query[key] })
  }
  function toggleUnassigned() {
    updateQuery({ unassigned: !unassignedOnly })
  }
  function setViewMode(mode: ViewMode) {
    setStoredView(mode)
    if (typeof window !== "undefined") localStorage.setItem(COMMODITIES_VIEW_MODE_KEY, mode)
    updateParams((p) => p.set("view", mode), { keepPage: true })
  }
  function goToPage(p: number) {
    updateParams((params) => params.set("page", String(p)), { keepPage: true })
  }
  // A new page size keeps the first item of the current page in view.
  function setPageSize(size: CommoditiesPageSize) {
    storeCommoditiesPageSize(size)
    const firstPage = Math.floor(((page - 1) * perPage) / size) + 1
    updateParams(
      (params) => {
        params.set(COMMODITIES_PAGE_SIZE_PARAM, String(size))
        if (firstPage > 1) params.set("page", String(firstPage))
        else params.delete("page")
      },
      { keepPage: true }
    )
  }

  // ---- Derived -----------------------------------------------------------
  const allRows = list.data?.commodities ?? []
  // Warranty status reads the BE-computed `warranty_expires_at`
  // (#1367). The warranty filter is applied by the BE, so the page, the
  // total and the pager all describe the same filtered set (#2128).
  const rows = allRows
  // Open-loan counts for the visible page only (#1452). The hook
  // skips the request when commodityIDsForCounts is empty (e.g. while
  // the list is still loading), so the badge column simply doesn't
  // render until the first row resolves. Using `rows` (post-warranty
  // filter) instead of `allRows` keeps the count batch sized to what's
  // actually on screen.
  const commodityIDsForCounts = rows.map((r) => r.id ?? "").filter(Boolean)
  const loanCountsQuery = useLoanCounts(commodityIDsForCounts)
  const loanCounts = loanCountsQuery.data ?? {}
  const serviceCountsQuery = useServiceCounts(commodityIDsForCounts)
  const serviceCounts = serviceCountsQuery.data ?? {}
  const total = list.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / perPage))
  // The rows of this page the selection covers, for the row checkboxes.
  const pageIds = rows.map((r) => r.id ?? "").filter(Boolean)
  const checked = selection.checkedOn(pageIds)
  const isLoading = list.isLoading
  const isError = list.isError
  const isEmpty = !isLoading && !isError && rows.length === 0
  const hasFilters = hasCommodityFilters(query)
  // A link can carry a filter the server refuses (a hand-edited URL, a
  // value from a newer version). The 400 names the parameter; the page
  // says so instead of showing a generic load error.
  const invalidQuery =
    isError && getServerErrorCode(list.error) === "query.invalid_parameter"
      ? (getServerErrorMeta(list.error)?.parameter ?? "")
      : null
  const matches = list.data?.matches ?? {}

  const areaName = useMemo(() => {
    const map = new Map<string, string>()
    for (const a of areas.data ?? []) {
      if (a.id) map.set(a.id, a.name ?? "")
    }
    return (id?: string) => (id ? (map.get(id) ?? "") : "")
  }, [areas.data])
  const itemTypes = useItemTypeLookup()
  const typeIcon = useCallback((type?: string) => itemTypes.get(type)?.Icon, [itemTypes])
  const typeOptions = useItemTypeFilterOptions(types)
  const tagIndex = useTagIndex("commodity")
  const resolveTag = tagIndex.resolve
  const tagLabel = useCallback((slug: string) => resolveTag(slug).label, [resolveTag])
  // A card's tag links to this list narrowed to that tag as well; the other
  // filters stay and the list starts again at the first page.
  const tagHref = useCallback(
    (tagSlug: string) => {
      if (!slug) return undefined
      const next = writeCommodityQuery(searchParams, withIncludedTag(query, tagSlug))
      next.delete("page")
      return `/g/${encodeURIComponent(slug)}/commodities?${next.toString()}`
    },
    [slug, searchParams, query]
  )

  // ---- Handlers ----------------------------------------------------------
  // After a successful create, drop the user on the new commodity's
  // detail page. The Vue app did the same, and the design mock keeps
  // the just-created item front-and-centre so the user can immediately
  // attach files / fix typos. The list page is one back-button away.
  async function handleCreate(values: CreateCommodityRequest, draft?: DraftRef) {
    const created = await create.submit(values, draft)
    toast.success(t("commodities:toast.created"))
    setCreateOpen(false)
    if (slug && created?.id) {
      navigate(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(created.id)}`)
    }
    // Return so the dialog can use the new id to upload + link any
    // attachments collected on the Files step.
    return created
  }

  // Wrap setCreateOpen so dismissing the dialog after entering via
  // /commodities/new lands back on /commodities — otherwise the URL
  // stays on /new and any future navigation away + back would re-open
  // the dialog from the URL effect above.
  function handleCreateOpenChange(open: boolean) {
    setCreateOpen(open)
    if (!open && isCreateRoute && slug) {
      navigate(`/g/${encodeURIComponent(slug)}/commodities`, { replace: true })
    }
  }

  return (
    <>
      <RouteTitle title={t("commodities:list.documentTitle")} />
      <Page width="wide" data-testid="page-commodities" {...drop.handlers}>
        {drop.dragging ? (
          <div
            className="pointer-events-none fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-6"
            data-testid="intake-drop-overlay"
          >
            <div className="flex flex-col items-center gap-2 rounded-xl border-2 border-dashed border-primary bg-card px-10 py-8 text-center">
              <FileUp className="size-8 text-primary" aria-hidden="true" />
              <p className="text-base font-medium">{t("commodities:intake.drop.title")}</p>
              <p className="text-sm text-muted-foreground">
                {t("commodities:intake.drop.description")}
              </p>
            </div>
          </div>
        ) : null}
        <PageHeader
          title={t("commodities:list.heading")}
          subtitle={t("commodities:list.subtitle", { count: total })}
          subtitleClassName="leading-7"
          actions={
            <div className="flex flex-wrap items-center gap-2">
              {intakeEnabled ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={openIntake}
                  data-testid="commodities-add-from-files"
                  className="gap-2"
                >
                  <FileUp className="size-4" aria-hidden="true" />
                  {t("commodities:intake.addFromFiles")}
                </Button>
              ) : null}
              <Button
                type="button"
                onClick={() => setCreateOpen(true)}
                data-testid="commodities-add-button"
                className="gap-2"
                disabled={migrationLock.locked}
                title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
                aria-disabled={migrationLock.locked || undefined}
              >
                <Plus className="size-4" aria-hidden="true" />
                {t("commodities:list.addItem")}
              </Button>
            </div>
          }
        />

        <IntakeCard />

        <InboxNotice />

        <Toolbar
          searchInput={searchInput}
          onSearchInput={setSearchInput}
          types={types}
          typeOptions={typeOptions}
          statuses={statuses}
          areaId={areaId}
          warrantyFilter={warrantyFilter}
          lentOutOnly={lentOutOnly}
          unassignedOnly={unassignedOnly}
          includeInactive={includeInactive}
          sort={validSort}
          sortDesc={sortDesc}
          viewMode={viewMode}
          areas={areas.data ?? []}
          sellerIds={query.sellerIds}
          sellers={sellers.data ?? []}
          hasFilters={hasFilters}
          onToggleType={toggleType}
          onToggleStatus={toggleStatus}
          onSetArea={setAreaFilter}
          onToggleSeller={toggleSeller}
          onToggleWarranty={toggleWarranty}
          onToggleLentOut={toggleLentOut}
          onToggleUnassigned={toggleUnassigned}
          missing={query}
          onToggleMissing={toggleMissing}
          tagFilter={query.tags}
          tagOptions={tagIndex.tags}
          tagCounts={tagCounts.data}
          tagFilterUnreadable={unreadableTagParamCount(query)}
          onSetTagFilter={(tags) => updateQuery(withTagFilter(query, tags))}
          onTagMenuOpenChange={setTagMenuOpen}
          onSetSort={setSort}
          onToggleInactive={toggleInactive}
          onClearFilters={clearFilters}
          onSetViewMode={setViewMode}
          savedViews={<SavedViewControl query={query} layout={viewMode} />}
        />

        <CommodityBulkActions
          selection={selection}
          rows={rows}
          total={total}
          // A placeholder is the previous query's page, whose digest names
          // other items; "Select all matching" waits for the real one.
          selectionDigest={list.isPlaceholderData ? "" : (list.data?.selectionDigest ?? "")}
          query={query}
          slug={slug}
          areas={areas.data ?? []}
          listContext={listContext}
          locked={migrationLock.locked}
        />

        {openedView.broken ? (
          <SavedViewQueryErrorAlert view={openedView.broken} />
        ) : openedView.unknown ? (
          <SavedViewUnavailableAlert onRetry={openedView.retry} />
        ) : openedView.pending ? (
          <ListLoading viewMode={viewMode} />
        ) : invalidQuery !== null ? (
          <Alert variant="destructive" data-testid="commodities-invalid-query">
            <AlertTitle>{t("commodities:list.invalidQueryTitle")}</AlertTitle>
            <AlertDescription className="flex flex-col items-start gap-2">
              <span>
                {t("commodities:list.invalidQueryDescription", { parameter: invalidQuery })}
              </span>
              <Button variant="outline" size="sm" onClick={clearFilters}>
                {t("commodities:filter.clear")}
              </Button>
            </AlertDescription>
          </Alert>
        ) : isError ? (
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
        ) : viewMode !== "list" ? (
          <CommoditiesGrid
            variant={viewMode === "grid" ? "photo" : "compact"}
            rows={rows}
            slug={slug}
            listContext={listContext}
            selected={checked}
            onToggleSelected={(id) => selection.toggle(id, pageIds)}
            onPreview={openCommodityInSheet}
            areaName={areaName}
            typeIcon={typeIcon}
            tagLabel={tagLabel}
            resolveTag={resolveTag}
            tagHref={tagHref}
            currency={currentGroup?.group_currency ?? "USD"}
            loanCounts={loanCounts}
            serviceCounts={serviceCounts}
            matches={matches}
          />
        ) : (
          <CommoditiesTable
            rows={rows}
            slug={slug}
            listContext={listContext}
            selected={checked}
            onToggleSelected={(id) => selection.toggle(id, pageIds)}
            onToggleSelectAll={() => selection.togglePage(pageIds)}
            onPreview={openCommodityInSheet}
            areaName={areaName}
            typeIcon={typeIcon}
            tagLabel={tagLabel}
            resolveTag={resolveTag}
            tagHref={tagHref}
            currency={currentGroup?.group_currency ?? "USD"}
            loanCounts={loanCounts}
            serviceCounts={serviceCounts}
            matches={matches}
          />
        )}

        {/* The page size choice shows once there is more than the smallest
            page to show. */}
        {!isError && total > COMMODITIES_PAGE_SIZES[0] ? (
          <div className="flex flex-col items-center gap-3 sm:flex-row sm:justify-between">
            <PageSizeSelect value={perPage} onChange={setPageSize} />
            {totalPages > 1 ? (
              <Pagination
                page={page}
                totalPages={totalPages}
                onChange={goToPage}
                testId="commodities-pagination"
              />
            ) : null}
          </div>
        ) : null}
      </Page>

      <CommodityFormDialog
        key={resumeDraftId ?? "new"}
        open={createOpen}
        onOpenChange={handleCreateOpenChange}
        mode="create"
        areas={areas.data ?? []}
        locations={locationsQuery.data ?? []}
        defaultCurrency={currentGroup?.group_currency ?? "USD"}
        onSubmit={handleCreate}
        isPending={create.isPending}
        draftKey={slug ? `commodity-draft:${slug}:create` : undefined}
        serverDrafts
        resumeDraftId={resumeDraftId}
      />

      {intakeLimits.data ? (
        <IntakeUploadDialog
          open={intakeOpen}
          onOpenChange={setIntakeOpen}
          limits={intakeLimits.data}
          initialFiles={intakeFiles}
          onQueued={handleIntakeQueued}
        />
      ) : null}

      {/*
        The dedicated preview Sheet that used to live here was
        replaced in #1546 by the modal-routes pattern: a row click
        navigates to /commodities/:id with `state.background` set,
        and the router renders <CommodityDetailSheet> on top of this
        list page (see app/router.tsx). The full detail surface — not
        the trimmed preview — is now what shows on a quick drill-in.
      */}
    </>
  )
}

// ---- Toolbar -------------------------------------------------------------

// The missing-data filters (#2794), in menu order.
const MISSING_FILTERS = ["noPhoto", "noPurchasePrice", "noPurchaseDate"] as const
type MissingFilter = (typeof MISSING_FILTERS)[number]

interface ToolbarProps {
  searchInput: string
  onSearchInput: (v: string) => void
  types: string[]
  // The types the filter offers, in the order shown.
  typeOptions: ItemTypeDisplay[]
  statuses: CommodityStatusValue[]
  areaId: string
  warrantyFilter: CommodityWarrantyStatus[]
  lentOutOnly: boolean
  unassignedOnly: boolean
  includeInactive: boolean
  sort: CommoditySortOption
  sortDesc: boolean
  viewMode: ViewMode
  areas: { id?: string; name?: string }[]
  sellerIds: string[]
  sellers: Seller[]
  hasFilters: boolean
  onToggleType: (t: string) => void
  onToggleStatus: (s: CommodityStatusValue) => void
  onSetArea: (id: string) => void
  onToggleSeller: (id: string) => void
  onToggleWarranty: (s: CommodityWarrantyStatus) => void
  onToggleLentOut: () => void
  onToggleUnassigned: () => void
  missing: Record<MissingFilter, boolean>
  onToggleMissing: (key: MissingFilter) => void
  tagFilter: CommodityTagFilter
  // Tag-filter parameters in the URL whose value cannot be read.
  tagFilterUnreadable: number
  tagOptions: ResolvedTag[]
  tagCounts: { tags: Record<string, number>; untagged: number } | undefined
  onSetTagFilter: (tags: CommodityTagFilter) => void
  onTagMenuOpenChange: (open: boolean) => void
  onSetSort: (f: CommoditySortOption) => void
  onToggleInactive: () => void
  onClearFilters: () => void
  onSetViewMode: (v: ViewMode) => void
  // The saved views control, shown before the layout switch.
  savedViews?: ReactNode
}

function missingFilterLabel(t: ReturnType<typeof useTranslation>["t"], key: MissingFilter) {
  switch (key) {
    case "noPhoto":
      return t("commodities:filter.missingOptions.noPhoto")
    case "noPurchasePrice":
      return t("commodities:filter.missingOptions.noPurchasePrice")
    case "noPurchaseDate":
      return t("commodities:filter.missingOptions.noPurchaseDate")
  }
}

function MissingFilterMenu({
  missing,
  onToggle,
}: {
  missing: Record<MissingFilter, boolean>
  onToggle: (key: MissingFilter) => void
}) {
  const { t } = useTranslation()
  const active = MISSING_FILTERS.filter((key) => missing[key]).length
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant={active > 0 ? "default" : "outline"}
          size="sm"
          className="gap-1.5"
          data-testid="commodities-filter-missing"
        >
          <ListFilter className="size-3.5" aria-hidden="true" />
          {t("commodities:filter.missing")}
          {active > 0 ? (
            <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
              {active}
            </Badge>
          ) : null}
          <ChevronDown className="size-3.5" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-52">
        <DropdownMenuLabel>{t("commodities:filter.missing")}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {MISSING_FILTERS.map((key) => (
          <DropdownMenuCheckboxItem
            key={key}
            checked={missing[key]}
            onCheckedChange={() => onToggle(key)}
            data-testid={`commodities-filter-missing-${key}`}
          >
            {missingFilterLabel(t, key)}
          </DropdownMenuCheckboxItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
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
          {props.typeOptions.map(({ slug, label, Icon }) => (
            <DropdownMenuCheckboxItem
              key={slug}
              checked={props.types.includes(slug)}
              onCheckedChange={() => props.onToggleType(slug)}
              data-testid={`commodities-filter-type-${slug}`}
            >
              <Icon aria-hidden="true" />
              {label}
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

      {/* Shown once the group has a seller, or while the URL filters by
          one (a deleted seller's link), so the filter can be cleared. */}
      {props.sellers.length > 0 || props.sellerIds.length > 0 ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant={props.sellerIds.length > 0 ? "default" : "outline"}
              size="sm"
              className="gap-1.5"
              data-testid="commodities-filter-seller"
            >
              <ListFilter className="size-3.5" aria-hidden="true" />
              {t("commodities:filter.seller")}
              {props.sellerIds.length > 0 ? (
                <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
                  {props.sellerIds.length}
                </Badge>
              ) : null}
              <ChevronDown className="size-3.5" aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="max-h-80 w-56 overflow-y-auto">
            <DropdownMenuLabel>{t("commodities:filter.seller")}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            {props.sellers.map((s) => (
              <DropdownMenuCheckboxItem
                key={s.id}
                checked={props.sellerIds.includes(s.id)}
                onCheckedChange={() => props.onToggleSeller(s.id)}
              >
                <span className="truncate">{s.name}</span>
              </DropdownMenuCheckboxItem>
            ))}
            {props.sellerIds
              .filter((id) => !props.sellers.some((s) => s.id === id))
              .map((id) => (
                <DropdownMenuCheckboxItem
                  key={id}
                  checked
                  onCheckedChange={() => props.onToggleSeller(id)}
                >
                  <span className="truncate italic text-muted-foreground">
                    {t("commodities:filter.sellerUnknown")}
                  </span>
                </DropdownMenuCheckboxItem>
              ))}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant={props.warrantyFilter.length > 0 ? "default" : "outline"}
            size="sm"
            className="gap-1.5"
            data-testid="commodities-filter-warranty"
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

      <TagFilterMenu
        label={t("commodities:filter.tags")}
        testIdPrefix="commodities-filter-tags"
        tags={props.tagOptions}
        value={props.tagFilter}
        unreadable={props.tagFilterUnreadable}
        onChange={props.onSetTagFilter}
        counts={props.tagCounts?.tags}
        untaggedCount={props.tagCounts?.untagged}
        allowExclude
        allowMode
        allowUntagged
        onOpenChange={props.onTagMenuOpenChange}
      />

      <Button
        type="button"
        variant={props.lentOutOnly ? "default" : "outline"}
        size="sm"
        className="gap-1.5"
        onClick={props.onToggleLentOut}
        aria-pressed={props.lentOutOnly}
        data-testid="commodities-filter-lent-out"
      >
        <ListFilter className="size-3.5" aria-hidden="true" />
        {t("commodities:filter.lentOut")}
      </Button>

      <Button
        type="button"
        variant={props.unassignedOnly ? "default" : "outline"}
        size="sm"
        className="gap-1.5"
        onClick={props.onToggleUnassigned}
        aria-pressed={props.unassignedOnly}
        data-testid="commodities-filter-unassigned"
      >
        <MapPinOff className="size-3.5" aria-hidden="true" />
        {t("commodities:filter.unassigned")}
      </Button>

      <MissingFilterMenu missing={props.missing} onToggle={props.onToggleMissing} />

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

      <div className="ml-auto flex items-center gap-1">
        {props.savedViews}
        <Button
          variant={props.viewMode === "grid" ? "secondary" : "ghost"}
          size="icon"
          className="size-8"
          onClick={() => props.onSetViewMode("grid")}
          aria-label={t("commodities:list.viewGrid")}
          aria-pressed={props.viewMode === "grid"}
          title={t("commodities:list.viewGrid")}
          data-testid="commodities-view-grid"
        >
          <LayoutGrid className="size-4" aria-hidden="true" />
        </Button>
        <Button
          variant={props.viewMode === "compact" ? "secondary" : "ghost"}
          size="icon"
          className="size-8"
          onClick={() => props.onSetViewMode("compact")}
          aria-label={t("commodities:list.viewCompact")}
          aria-pressed={props.viewMode === "compact"}
          title={t("commodities:list.viewCompact")}
          data-testid="commodities-view-compact"
        >
          <LayoutList className="size-4" aria-hidden="true" />
        </Button>
        <Button
          variant={props.viewMode === "list" ? "secondary" : "ghost"}
          size="icon"
          className="size-8"
          onClick={() => props.onSetViewMode("list")}
          aria-label={t("commodities:list.viewList")}
          aria-pressed={props.viewMode === "list"}
          title={t("commodities:list.viewList")}
          data-testid="commodities-view-list"
        >
          <List className="size-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
  )
}

// ---- Loading / Empty ----------------------------------------------------

function ListLoading({ viewMode }: { viewMode: ViewMode }) {
  if (viewMode === "grid") return <CommodityPhotoGridSkeleton />
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
  // The list the rows belong to; item links carry it in `?from=`.
  listContext: ListContext
  selected: ReadonlySet<string>
  onToggleSelected: (id: string) => void
  onPreview: (id: string) => void
  areaName: (id?: string) => string
  // The icon of an item type, by slug.
  typeIcon: (type?: string) => LucideIcon | undefined
  // Resolves a stored tag slug to the tag's label (slug while unknown).
  tagLabel: (slug: string) => string
  // Resolves a stored tag slug to its label and color.
  resolveTag: (slug: string) => ResolvedTag
  // Where a tag on a row links to.
  tagHref: (tagSlug: string) => string | undefined
  currency: string
  // Map of commodity_id → open-loan count (#1452). Rows with count > 0
  // render a "Lent out" pill. Empty / missing keys mean "not lent" so
  // the parent can pass a stale map without flicker while the loan
  // counts query refetches in the background.
  loanCounts?: Record<string, number>
  // Map of commodity_id → open-service count (#1508). Same flicker-free
  // semantics as loanCounts. Rendered as a separate "In service" pill —
  // an item can technically only be in one holding kind at a time
  // (cross-kind invariant), but we render whichever the BE reports.
  serviceCounts?: Record<string, number>
  // Per commodity id, why it matched the search text (meta.matches).
  // Empty without a search.
  matches?: Record<string, CommoditySearchMatch[]>
}

function CommoditiesGrid({
  variant,
  rows,
  slug,
  listContext,
  selected,
  onToggleSelected,
  onPreview,
  areaName,
  typeIcon,
  tagLabel,
  resolveTag,
  tagHref,
  currency,
  loanCounts,
  serviceCounts,
  matches,
}: CommoditiesGridProps & { variant: "photo" | "compact" }) {
  const cards = rows.map((row) => {
    const props: CommodityCardProps = {
      row,
      slug,
      listContext,
      selected: selected.has(row.id ?? ""),
      onToggleSelected,
      onPreview,
      areaName,
      typeIcon,
      tagLabel,
      resolveTag,
      tagHref,
      currency,
      lent: (loanCounts?.[row.id ?? ""] ?? 0) > 0,
      inService: (serviceCounts?.[row.id ?? ""] ?? 0) > 0,
      match: primarySearchMatch(matches?.[row.id ?? ""]),
    }
    return variant === "photo" ? (
      <CommodityPhotoCard key={row.id} {...props} />
    ) : (
      <CommodityGridCard key={row.id} {...props} />
    )
  })
  if (variant === "photo") {
    return (
      <CommodityPhotoGrid data-testid="commodities-grid" data-view="grid">
        {cards}
      </CommodityPhotoGrid>
    )
  }
  return (
    <div
      className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
      data-testid="commodities-grid"
      data-view="compact"
    >
      {cards}
    </div>
  )
}

function CommodityGridCard({
  row,
  slug,
  listContext,
  selected,
  onToggleSelected,
  onPreview,
  areaName,
  typeIcon,
  tagLabel,
  resolveTag,
  tagHref,
  currency,
  lent,
  inService,
  match,
}: CommodityCardProps) {
  const { t } = useTranslation()
  const id = row.id ?? ""
  const detailHref = slug && id ? commodityHref(slug, id, listContext) : "#"
  const status = row.status as CommodityStatusValue | undefined
  const tone = status ? COMMODITY_STATUS_TONES[status] : ""
  const draftLabel = t("commodities:list.draftBadge")
  const statusLabel = status ? t(`commodities:status.${status}`) : ""
  const purchaseDateShort = row.purchase_date
    ? formatDate(row.purchase_date, { style: "short" })
    : null
  const purchaseDateLabel = purchaseDateShort
    ? t("commodities:card.purchasedOn", { date: purchaseDateShort })
    : null
  // The mock's "brand · model" line; an item with neither shows its short
  // name there instead.
  const subtitle = brandModelText(row) || row.short_name
  // Bare click opens the Sheet preview; ctrl/cmd/shift-click and
  // middle-click fall through to the underlying Link so the user can
  // open the canonical URL in a new tab. shiftKey/aux-button check
  // mirrors react-router's own DOM-link guard.
  function handleCardClick(e: React.MouseEvent<HTMLAnchorElement>) {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return
    e.preventDefault()
    onPreview(id)
  }
  return (
    <Card
      className={cn(
        "relative cursor-pointer gap-3 transition-all hover:shadow-md hover:-translate-y-0.5",
        row.draft && "opacity-70 border-dashed"
      )}
      data-testid="commodity-card"
      data-commodity-id={id}
      data-variant="compact"
    >
      {/* Card-wide click target: an absolute overlay <Link>. Because it
          is a *positioned* sibling, it paints ON TOP of the (static)
          card content — so inert columns carry `pointer-events-none` to
          let clicks fall through to it. Interactive children (the
          Checkbox) must therefore BOTH re-enable pointer events
          (`pointer-events-auto`) AND lift themselves above the overlay
          (`relative z-10`); without the z-lift the link sits over the
          checkbox and steals its click, opening the detail sheet
          instead of toggling selection (#1965). Pattern: LocationsListPage / #1638. */}
      <Link
        to={detailHref}
        aria-label={row.name ?? ""}
        onClick={handleCardClick}
        className="absolute inset-0 rounded-xl focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring/50"
        data-testid="commodity-card-link"
      />
      <CardHeader className="pb-2">
        <div className="pointer-events-none flex items-start justify-between gap-2">
          <div className="flex items-center gap-2">
            <Checkbox
              checked={selected}
              onCheckedChange={() => onToggleSelected(id)}
              aria-label={t("commodities:list.selectItem", { name: row.name ?? "" })}
              data-testid="commodity-select"
              className="relative z-10 pointer-events-auto"
            />
            <CommodityThumb
              cover={row.cover}
              type={row.type}
              icon={typeIcon(row.type)}
              name={row.name}
              size={36}
              testId="commodity-card-thumb"
            />
          </div>
          <div className="flex items-center gap-1.5 flex-wrap justify-end">
            {row.draft ? (
              <Badge variant="outline" className="text-[10px] h-4 px-1 border-dashed">
                {draftLabel}
              </Badge>
            ) : null}
            {lent ? (
              <Badge
                variant="secondary"
                className="text-[10px] h-4 px-1"
                data-testid="commodity-lent-badge"
              >
                {t("loans:badge.lentOut")}
              </Badge>
            ) : null}
            {inService ? (
              <Badge
                variant="secondary"
                className="text-[10px] h-4 px-1"
                data-testid="commodity-in-service-badge"
              >
                {t("services:badge.inService")}
              </Badge>
            ) : null}
            {status && status !== "in_use" ? (
              <span
                className={cn("text-[10px] font-medium px-1.5 py-0.5 rounded-full border", tone)}
              >
                {statusLabel}
              </span>
            ) : null}
            <WarrantyBadge
              source={{ warranty_expires_at: row.warranty_expires_at }}
              showIcon={false}
              className="h-4 px-1.5 text-[10px]"
              data-testid="commodity-card-warranty"
            />
          </div>
        </div>
        <CardTitle
          className="pointer-events-none mt-2 text-sm font-semibold leading-tight"
          data-testid="commodity-card-title"
        >
          {row.name}
        </CardTitle>
        <CardDescription
          className="pointer-events-none truncate text-xs"
          data-testid="commodity-card-subtitle"
        >
          {subtitle}
        </CardDescription>
        <SearchMatchHint match={match} tagLabel={tagLabel} className="pointer-events-none" />
      </CardHeader>
      <CardContent className="pointer-events-none">
        <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
          <div className="flex min-w-0 items-center gap-2">
            <span className="truncate" data-testid="commodity-card-area">
              {areaName(row.area_id)}
            </span>
            {purchaseDateShort ? (
              <span
                className="inline-flex shrink-0 items-center gap-1"
                aria-label={purchaseDateLabel ?? undefined}
                title={purchaseDateLabel ?? undefined}
                data-testid="commodity-card-purchase-date"
              >
                <Calendar className="size-3" aria-hidden="true" />
                {purchaseDateShort}
              </span>
            ) : null}
          </div>
          <span className="shrink-0 font-medium text-foreground">
            {formatPrice(row.current_price, currency, "—", formatCurrency)}
          </span>
        </div>
        {row.tags && row.tags.length > 0 ? (
          <div className="mt-2 flex flex-wrap gap-1">
            {row.tags.slice(0, 3).map((tagSlug) => {
              const tag = resolveTag(tagSlug)
              return (
                // Above the card-wide link, like the checkbox.
                <TagPill
                  key={tagSlug}
                  label={tag.label}
                  color={tag.color}
                  size="xs"
                  href={tagHref(tagSlug)}
                  className="pointer-events-auto relative z-10"
                  data-testid="commodity-card-tag"
                />
              )
            })}
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
  listContext,
  selected,
  onToggleSelected,
  onToggleSelectAll,
  onPreview,
  areaName,
  typeIcon,
  tagLabel,
  currency,
  loanCounts,
  serviceCounts,
  matches,
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
          const detailHref = slug && id ? commodityHref(slug, id, listContext) : "#"
          const status = row.status as CommodityStatusValue | undefined
          const tone = status ? COMMODITY_STATUS_TONES[status] : ""
          return (
            <li key={id} data-testid="commodity-row" data-commodity-id={id}>
              {i > 0 ? <Separator /> : null}
              <div
                className={cn(
                  "group relative flex items-center gap-3 px-5 py-3 transition-colors hover:bg-muted/50",
                  row.draft && "opacity-70"
                )}
              >
                {/* Whole-row click target — same overlay pattern as the
                    grid card: the absolute <Link> paints over the static
                    columns, so inert columns get `pointer-events-none`
                    and the Checkbox needs `relative z-10 pointer-events-auto`
                    to sit above the overlay (else the row link steals the
                    checkbox click and opens the sheet — #1965). */}
                <Link
                  to={detailHref}
                  aria-label={row.name ?? ""}
                  onClick={(e) => handleRowClick(id, e)}
                  className="absolute inset-0 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring/50"
                  data-testid="commodity-row-link"
                />
                <Checkbox
                  checked={selected.has(id)}
                  onCheckedChange={() => onToggleSelected(id)}
                  aria-label={t("commodities:list.selectItem", { name: row.name ?? "" })}
                  data-testid="commodity-row-select"
                  className="relative z-10 pointer-events-auto"
                />
                <div className="pointer-events-none">
                  <CommodityThumb
                    cover={row.cover}
                    type={row.type}
                    icon={typeIcon(row.type)}
                    name={row.name}
                    size={36}
                    testId="commodity-row-thumb"
                  />
                </div>
                <div className="pointer-events-none flex-1 min-w-0">
                  <div className="flex items-center gap-1.5">
                    <p className="truncate text-sm font-medium" data-testid="commodity-row-title">
                      {row.name}
                    </p>
                    {(loanCounts?.[id] ?? 0) > 0 ? (
                      <Badge
                        variant="secondary"
                        className="text-[10px] h-4 px-1"
                        data-testid="commodity-lent-badge"
                      >
                        {t("loans:badge.lentOut")}
                      </Badge>
                    ) : null}
                    {(serviceCounts?.[id] ?? 0) > 0 ? (
                      <Badge
                        variant="secondary"
                        className="text-[10px] h-4 px-1"
                        data-testid="commodity-in-service-badge"
                      >
                        {t("services:badge.inService")}
                      </Badge>
                    ) : null}
                  </div>
                  {brandModelText(row) || row.short_name ? (
                    <p
                      className="text-xs text-muted-foreground truncate"
                      data-testid="commodity-row-subtitle"
                    >
                      {brandModelText(row) || row.short_name}
                    </p>
                  ) : null}
                  <SearchMatchHint match={primarySearchMatch(matches?.[id])} tagLabel={tagLabel} />
                </div>
                <span className="pointer-events-none hidden sm:block w-32 truncate text-xs text-muted-foreground">
                  {areaName(row.area_id)}
                </span>
                <span className="pointer-events-none hidden sm:flex w-24 items-center">
                  {status && status !== "in_use" ? (
                    <span
                      className={cn(
                        "text-xs font-medium px-2 py-0.5 rounded-full border inline-block",
                        tone
                      )}
                    >
                      {t(`commodities:status.${status}`)}
                    </span>
                  ) : (
                    <WarrantyBadge
                      source={{ warranty_expires_at: row.warranty_expires_at }}
                      showIcon={false}
                      className="h-5 px-1.5 text-[10px]"
                      data-testid="commodity-row-warranty"
                    />
                  )}
                </span>
                <span className="pointer-events-none w-24 text-right text-sm font-medium">
                  {formatPrice(row.current_price, currency, "—", formatCurrency)}
                </span>
              </div>
            </li>
          )
        })}
      </ul>
    </Card>
  )
}

// ---- Page size -----------------------------------------------------------

function PageSizeSelect({
  value,
  onChange,
}: {
  value: CommoditiesPageSize
  onChange: (size: CommoditiesPageSize) => void
}) {
  const { t } = useTranslation()
  const labelId = useId()
  return (
    <div className="flex items-center gap-2 text-sm text-muted-foreground">
      <span id={labelId}>{t("commodities:list.pageSize")}</span>
      <Select
        value={String(value)}
        onValueChange={(v) => {
          const size = parsePageSize(v)
          if (size) onChange(size)
        }}
      >
        <SelectTrigger
          size="sm"
          className="w-20"
          aria-labelledby={labelId}
          data-testid="commodities-page-size"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {COMMODITIES_PAGE_SIZES.map((size) => (
            <SelectItem key={size} value={String(size)}>
              {size}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
