import { useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import {
  Link,
  useLocation,
  useMatch,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom"
import { useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  ArrowLeft,
  BarChart3,
  Calendar,
  CircleDot,
  DollarSign,
  ExternalLink,
  FileInput,
  FileText,
  Hash,
  ListPlus,
  MapPin,
  Maximize2,
  Package,
  Paperclip,
  Pencil,
  Printer,
  QrCode,
  Store,
  Tag,
  Trash2,
  TriangleAlert,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Page } from "@/components/ui/page"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { CommodityFilesTab } from "@/components/files/CommodityFilesTab"
import { DropOverlay } from "@/components/files/DropOverlay"
import { UploadFilesDialog } from "@/components/files/UploadFilesDialog"
import { useFileDropZone } from "@/components/files/useFileDropZone"
import { AssignLocationBanner } from "@/components/items/AssignLocationBanner"
import { CommodityCoverPanel } from "@/components/items/CommodityCoverPanel"
import { CommodityEditForm } from "@/components/items/CommodityEditForm"
import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { DocumentSource } from "@/components/items/DocumentSource"
import { ListPager } from "@/components/ListPager"
import {
  LocationAreaFields,
  type AreaOption,
  type LocationOption,
} from "@/components/locations/LocationAreaFields"
import { TagPalette } from "@/components/tags/TagPalette"
import { Textarea } from "@/components/ui/textarea"
import {
  StatusTransitionDialog,
  type StatusTransitionPayload,
} from "@/components/items/StatusTransitionDialog"
import { LendTab } from "@/components/loans/LendTab"
import { MaintenanceTab } from "@/components/maintenance/MaintenanceTab"
import { CommodityRemindersSection } from "@/components/reminders/CommodityRemindersSection"
import { ServiceTab } from "@/components/services/ServiceTab"
import { SuppliesTab } from "@/components/supplies/SuppliesTab"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAreas } from "@/features/areas/hooks"
import { useFiles } from "@/features/files/hooks"
import { useLocations } from "@/features/locations/hooks"
import { formatAssetId } from "@/features/commodities/asset-id"
import { brandModelText } from "@/features/commodities/brand"
import { CommodityHistoryTimeline } from "@/features/commodities/CommodityHistoryTimeline"
import { CommodityThumb } from "@/features/commodities/CommodityThumb"
import { formatPrice } from "@/features/commodities/price"
import {
  useCommodity,
  useDeleteCommodity,
  useSetCommodityCover,
  useUpdateCommodity,
} from "@/features/commodities/hooks"
import {
  COMMODITY_STATUS_TONES,
  TERMINAL_COMMODITY_STATUSES,
  warrantyStatus,
  type CommodityStatusValue,
  type CommodityWarrantyStatus,
  type TerminalCommodityStatusValue,
} from "@/features/commodities/constants"

// CHANGE-STATUS bar: terminal status transitions surfaced as quick
// buttons on the detail surface (mock: "Change Status" section).
// Every status but `in_use`, which is what the item transitions *from*.
// The mock's row stops at Written Off; Donated and Stolen follow it.
const TERMINAL_STATUSES = TERMINAL_COMMODITY_STATUSES

// Mock parity colour-only mapping for the CHANGE STATUS quick
// buttons. Lifted from `inventario-design`'s
// `COMMODITY_STATUS_CONFIG.color` field — text-only, no bg/border
// tint, so the buttons are plain outline pills with coloured
// labels (the mock's `cn("gap-1.5 text-xs h-7", c.color)` pattern).
// Distinct from the project's `COMMODITY_STATUS_TONES` (which adds
// bg + border for the inline status pills elsewhere).
const STATUS_TRANSITION_TEXT_TONES: Record<(typeof TERMINAL_STATUSES)[number], string> = {
  sold: "text-chart-2",
  lost: "text-status-expiring",
  disposed: "text-muted-foreground",
  written_off: "text-status-expired",
  donated: "text-chart-3",
  stolen: "text-chart-4",
}
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import { WARRANTY_STATUS_CONFIG } from "@/components/warranty/config"
import {
  isCommodityRevisionConflict,
  type Commodity,
  type LinkedItemSummary,
  type UpdateCommodityRequest,
} from "@/features/commodities/api"
import { CustomFieldValue } from "@/components/customFields/CustomFieldValue"
import type { CustomFieldValues } from "@/features/customFields/api"
import { useCustomFields } from "@/features/customFields/hooks"
import { hasValue } from "@/features/customFields/values"
import { useGroupMigrationLock } from "@/features/currency-migration/lock"
import { labelsPageHref } from "@/features/labels/links"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useSectionVisible } from "@/features/group/sections"
import { useLoansForCommodity } from "@/features/loans/hooks"
import { useMaintenanceForCommodity } from "@/features/maintenance/hooks"
import { useServicesForCommodity } from "@/features/services/hooks"
import { useItemTypeLookup } from "@/features/itemTypes/hooks"
import { useUndoItemDelete } from "@/features/trash/useUndoItemDelete"
import {
  commodityHref,
  commoditiesListSource,
  isCommoditiesListContext,
} from "@/features/commodities/listContext"
import { commodityKeys } from "@/features/commodities/keys"
import { useMyGroupRole } from "@/features/group/roles"
import { setCommodityDraft } from "@/features/inbox/api"
import { useSetItemDraft } from "@/features/inbox/hooks"
import { inboxReviewSource, isInboxReviewContext } from "@/features/inbox/listContext"
import { reviewRefusal } from "@/features/inbox/reviewRefusal"
import { InboxReviewBar } from "@/components/items/InboxReviewBar"
import {
  commodityListHref,
  commoditySellerHref,
  commodityTagHref,
} from "@/features/commodities/query"
import { useItemSource } from "@/features/intake/hooks"
import { useSellers } from "@/features/sellers/hooks"
import { useTagIndex } from "@/features/tags/hooks"
import { TagPill } from "@/components/tags/TagPill"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { fetchRecordAfter, useListNavigation, type ListNeighbor } from "@/hooks/useListNavigation"
import { useLocationGuard } from "@/hooks/useLocationGuard"
import { formatCurrency, formatDate } from "@/lib/intl"
import {
  listContextHref,
  readListContext,
  withListContextPage,
  type ListContext,
} from "@/lib/list-context"
import { parseServerError } from "@/lib/server-error"
import { toastMutationError } from "@/lib/mutation-error-toast"
import { cn } from "@/lib/utils"

type TabKey = "details" | "warranty" | "files" | "lend" | "service" | "supplies" | "maintenance"

const TAB_KEYS = [
  "details",
  "warranty",
  "files",
  "lend",
  "service",
  "supplies",
  "maintenance",
] as const

function parseTab(raw: string | null): TabKey {
  return (TAB_KEYS as readonly string[]).includes(raw ?? "") ? (raw as TabKey) : "details"
}

// CommodityDetailContent is the rendered body of the item detail surface
// — header, action buttons, tab strip, and the per-tab bodies. The same
// component is rendered in two visual frames:
//
//   - `variant="page"` (default): standard full-page wrapper at
//     `/g/:slug/commodities/:id`. Stable URL, deep-link friendly,
//     refresh-survivable. This is what the catch-all route and
//     direct-link arrivals see (#1546 AC2).
//   - `variant="sheet"`: rendered inside a right-side `<Sheet>` overlay
//     on top of the items list. Used when the user drills in from the
//     list — the URL still updates to the same `/commodities/:id` so
//     back/forward and `?tab=` deep-links work, but the list page
//     stays mounted behind the sheet (#1546 AC1).
//
// Tabs: Details, Warranty, Files, Lend, Service. The same hooks /
// query-string mirror / edit dialog / drop zone all run in both
// variants — the only difference is the outer chrome and a "Back to
// list" affordance vs. the Sheet's built-in close button.
// PageFrame swaps between a plain sheet body and the canonical Page wrapper.
// Keeps every render site (loading / error / not-found / main) routed through
// the same width token without duplicating the branching. Hoisted to module
// scope to satisfy react-hooks/static-components.
function PageFrame({
  isSheet,
  className,
  children,
  ...rest
}: React.ComponentProps<"div"> & { isSheet: boolean }) {
  if (isSheet) {
    return (
      <div className={className} {...rest}>
        {children}
      </div>
    )
  }
  return (
    <Page width="wide" className={className} {...rest}>
      {children}
    </Page>
  )
}

export interface CommodityDetailContentProps {
  /**
   * The commodity id to load. Pre-extracted from the URL by the page
   * or sheet wrapper so this component stays unaware of the route shape.
   */
  id: string
  /**
   * Outer-frame variant. Default `"page"`. `"sheet"` drops the
   * page-level max-width wrapper and the "Back to list" link (the
   * Sheet provides its own X close button), but otherwise renders
   * the same content with the same hooks and behaviour.
   */
  variant?: "page" | "sheet"
}

export function CommodityDetailContent({ id, variant = "page" }: CommodityDetailContentProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  // Outer-frame classes match the design mock 1:1
  // (`denisvmedia/inventario-design/src/components/ItemDetail.tsx`):
  // sheet mode uses `flex flex-col gap-0 px-5 pb-5` with each section
  // owning its own vertical padding (the mock's `<SheetHeader pt-6
  // pb-4>` + `flex gap-2 pb-4` + `mb-4 rounded-xl …` pattern). Page
  // mode keeps the wider 4xl centered-column layout that direct
  // landings expect. Scroll is owned by `<SheetContent
  // overflow-y-auto>` — leaving it off the inner wrapper avoids the
  // double-scrollbar bug copilot flagged.
  const isSheet = variant === "sheet"
  // Page-mode width comes from the canonical wide token via <Page width="wide">
  // (`PageFrame` below). The literal class strings here stay scoped to the sheet
  // variant so the page-layout convention guard's `max-w-*` check never trips
  // on a top-level page wrapper.
  const errorWrapperClass = isSheet ? "px-5 py-6" : undefined
  const mainWrapperClass = isSheet ? "relative flex flex-col gap-0 px-5 pb-5" : "relative"
  const { currentGroup } = useCurrentGroup()
  const migrationLock = useGroupMigrationLock()
  const slug = currentGroup?.slug
  const enabled = !!currentGroup
  const detail = useCommodity(id, { enabled })
  const areas = useAreas({ enabled })
  const locations = useLocations({ enabled })
  const itemTypes = useItemTypeLookup()
  const update = useUpdateCommodity()
  const remove = useDeleteCommodity()
  const undoDelete = useUndoItemDelete()
  const setCover = useSetCommodityCover()
  const toast = useAppToast()
  const queryClient = useQueryClient()
  const askConfirm = useConfirm()
  // The Files tab label gets a count badge driven by this query.
  // perPage=1 keeps the round-trip cheap — we only need `meta.total`.
  // Gated on `enabled && id` so it doesn't fire before the slug
  // resolves, and the cache key matches the "all files attached to
  // this commodity" view the Files tab reuses.
  const filesCount = useFiles(
    { linkedEntityType: "commodity", linkedEntityId: id, perPage: 1 },
    { enabled: enabled && !!id }
  )
  const fileCount = filesCount.data?.total ?? 0

  // Tab selection is mirrored in the `?tab=` query string so deep
  // links from the warranties list / dashboard expiring panel land on
  // the right tab. `?tab=details` is the default — we strip the param
  // when switching back so the URL stays clean.
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedTab = parseTab(searchParams.get("tab"))
  function setTab(next: TabKey) {
    const params = new URLSearchParams(searchParams)
    if (next === "details") params.delete("tab")
    else params.set("tab", next)
    setSearchParams(params, { replace: true })
  }
  // /commodities/:id/edit is the item's edit form, in place of the read
  // view (the mock's ItemEditForm). Being a URL, it survives a reload and
  // "Save and next" can land on the next item's form.
  const editMatch = useMatch({ path: "/g/:groupSlug/commodities/:id/edit", end: true })
  const editing = !!editMatch
  // Each dialog of the page keeps the item it was opened on and writes to
  // that item alone. Moving to another item, even one already loaded,
  // closes it, so what was entered never lands on the wrong item.
  //
  // The step-by-step dialog, opened from the edit form's "All fields" with
  // the values entered so far. Null while closed.
  const [allFieldsOn, setAllFieldsOn] = useState<{ id: string; values: Commodity } | null>(null)
  const dialogDraft = allFieldsOn?.id === id ? allFieldsOn.values : null
  // The item whose edit form was opened before the item last changed: a
  // save from the dialog that form opened was refused (the form tracks its
  // own refused saves). Reload reads the item again and opens a fresh form
  // (a new formLoad gives it a new key).
  const [staleEditOn, setStaleEditOn] = useState<string | null>(null)
  const [formLoad, setFormLoad] = useState(0)
  const [reloadingForm, setReloadingForm] = useState(false)
  // The refusal belongs to the edit form it happened in. Leaving that form
  // by any way (Cancel, Back, Save and next) forgets it: the refused save
  // read the item again, so the next form opens on the item as it is.
  useEffect(() => {
    if (!editing) return
    return () => setStaleEditOn(null)
  }, [editing, id])
  // The Details tab field being edited in place (#2795), if any.
  const [quickEditOn, setQuickEditOn] = useState<{ id: string; field: QuickEditField } | null>(null)
  const quickEdit = quickEditOn?.id === id ? quickEditOn.field : null
  const setQuickEdit = (field: QuickEditField | null) =>
    setQuickEditOn(field ? { id, field } : null)
  // #1448 quick-attach state. `pendingDropFiles` holds files seeded by
  // the page-level drop catcher; the upload dialog reads them on open
  // and queues them in the select step. The button-triggered open
  // path leaves it empty so the user picks files inside the dialog.
  const [uploadFor, setUploadFor] = useState<string | null>(null)
  const uploadOpen = uploadFor === id
  const [pendingDropFiles, setPendingDropFiles] = useState<File[]>([])
  // #1611: forward-transition target gates the StatusTransitionDialog.
  // Null = dialog closed. Revert keeps the lighter useConfirm path
  // (no metadata to capture).
  const [transitionOn, setTransitionOn] = useState<{
    id: string
    target: TerminalCommodityStatusValue
  } | null>(null)
  const transitionTarget = transitionOn?.id === id ? transitionOn.target : null
  // An open dialog or confirm holds the review walk on the item: what it
  // saves belongs to this item. Read after an await, where the render's
  // own values are stale.
  const dialogOpen = useRef(false)
  const confirmOpen = useRef(false)
  useEffect(() => {
    dialogOpen.current =
      quickEdit !== null || transitionTarget !== null || uploadOpen || dialogDraft !== null
  }, [quickEdit, transitionTarget, uploadOpen, dialogDraft])
  const holdsItem = () => dialogOpen.current || confirmOpen.current
  async function confirm(options: Parameters<typeof askConfirm>[0]) {
    confirmOpen.current = true
    try {
      return await askConfirm(options)
    } finally {
      confirmOpen.current = false
    }
  }
  const dropZone = useFileDropZone({
    onFiles: (files) => {
      setPendingDropFiles(files)
      setUploadFor(id)
    },
    // Don't catch new drags while the dialog is already open — its
    // own dropzone takes over so a second drop doesn't bounce through
    // the page wrapper.
    disabled: uploadOpen,
  })

  // Opened from the Items list, the item carries that list in `?from=`
  // (lib/list-context.ts): Back returns to it, and Previous / Next / Save
  // and next walk it in its filters and order (hooks/useListNavigation.ts).
  // The list queries wait for the group, as the list page does, so both
  // build the same query keys and share the cached pages. Opened from the
  // inbox, it walks the items to review the same way, and the review bar
  // marks each one as reviewed (features/inbox/listContext.ts).
  const location = useLocation()
  const groupReady = !!currentGroup?.slug
  const from = useMemo(
    () => (slug ? readListContext(searchParams, `/g/${encodeURIComponent(slug)}`) : null),
    [searchParams, slug]
  )
  const itemsList = from && slug && isCommoditiesListContext(from, slug) ? from : null
  const reviewList = from && slug && isInboxReviewContext(from, slug) ? from : null
  const walked = itemsList ?? reviewList
  const source = useMemo(() => {
    if (!groupReady || !slug) return null
    if (itemsList) return commoditiesListSource(itemsList, slug)
    if (reviewList) return inboxReviewSource(reviewList, slug)
    return null
  }, [itemsList, reviewList, groupReady, slug])
  const nav = useListNavigation({ source, currentId: id || undefined })
  // The list context to carry: on the list page that holds the item.
  const currentContext = walked ? withListContextPage(walked, nav.page) : from
  const contextAt = (page: number) => (walked ? withListContextPage(walked, page) : currentContext)
  const role = useMyGroupRole()
  const canWrite = role === "user" || role === "admin" || role === "owner"
  const setDraft = useSetItemDraft()
  // A review and the item's own saves are not started together: a quick
  // save sends the item as the page last read it, draft mark included, and
  // would put the mark back if it landed after the review.
  const itemSaving = update.isPending || setDraft.isPending
  const guardLocation = useLocationGuard()

  // Navigates without leaving the sheet: the sheet's background (the list
  // behind it) moves along to the list page the context now points at.
  // `editEntered` marks an edit form opened with Edit from this item's read
  // view, the entry just before it in the history.
  function go(
    to: string,
    ctx: ListContext | null,
    opts: { replace?: boolean; editEntered?: boolean } = {}
  ) {
    const state: Record<string, unknown> = { ...(location.state as object | null) }
    delete state.editEntered
    if (state.background && ctx) {
      state.background = {
        pathname: ctx.pathname,
        search: ctx.search ? `?${ctx.search}` : "",
        hash: "",
      }
    }
    if (opts.editEntered) state.editEntered = true
    navigate(to, {
      replace: opts.replace,
      state: Object.keys(state).length > 0 ? state : undefined,
    })
  }
  function itemHref(targetId: string, ctx: ListContext | null, edit = false) {
    return slug ? commodityHref(slug, targetId, ctx, { edit }) : "#"
  }
  function startEditing() {
    setQuickEdit(null)
    go(itemHref(id, currentContext, true), currentContext, { editEntered: true })
  }
  function stopEditing() {
    // Back to the read view the Edit button was pressed on, so the history
    // holds the item once. A form reached by a link, a reload of a typed
    // URL, or "Save and next" swaps itself for the item's read view.
    if ((location.state as { editEntered?: boolean } | null)?.editEntered) {
      navigate(-1)
      return
    }
    go(itemHref(id, currentContext), currentContext, { replace: true })
  }
  function goToNeighbor(target: ListNeighbor, edit: boolean) {
    const ctx = contextAt(target.page)
    go(itemHref(target.id, ctx, edit), ctx)
  }
  // Clears the draft mark of an item opened from the inbox, then moves to
  // the next item to review, or back to the inbox after the last one. The
  // next item is read from the list after the save: the one that followed
  // this item while the list still holds it, else the one now in this
  // item's place (hooks/useListNavigation.ts, fetchRecordAfter). A user who
  // moved on while the save ran stays where they went, one who opened a
  // dialog or a confirm stays on the item, and a list that could not be
  // read is not taken for one that ended: the page stays on the item.
  async function markReviewedAndNext(title: string) {
    const walk = source
    const offset = nav.position === null ? null : nav.position - 1
    const followed = nav.next?.id ?? null
    const backToList = currentContext
    const stillHere = guardLocation()
    try {
      await setDraft.mutateAsync({ id, draft: false })
    } catch (err) {
      const problems = reviewRefusal(t, err)
      if (problems) toast.error(t("inbox:reviewFailed"), { description: problems })
      else toast.error(parseServerError(err, t("inbox:reviewFailed")))
      return
    }
    toast.success(t("inbox:reviewed", { title }), {
      action: {
        label: t("inbox:undo"),
        onClick: () => {
          setCommodityDraft(id, true).then(
            () => void queryClient.invalidateQueries({ queryKey: commodityKeys.group(slug ?? "") }),
            (err: unknown) => toast.error(parseServerError(err, t("inbox:undoFailed")))
          )
        },
      },
    })
    if (!walk || offset === null) return
    let next: ListNeighbor | null
    try {
      next = await fetchRecordAfter(queryClient, walk, offset, { skipId: id, expected: followed })
    } catch {
      return
    }
    if (!stillHere() || holdsItem()) return
    if (next) goToNeighbor(next, false)
    else if (backToList) navigate(listContextHref(backToList))
  }

  const commodity = detail.data?.commodity

  // A section the group hides (#1607) drops its tab, unless the item holds
  // data for it: an item with a warranty, a loan, a service record or a
  // maintenance schedule keeps the tab and shows that data, without the
  // action that starts a new one. The lists are read only while the
  // section is hidden; the tabs read them anyway once opened. A tab goes
  // only once its data has loaded and holds nothing, so a link to the tab
  // does not fall back to Details while the data loads, and a failed read
  // does not hide data the item may have.
  const showWarranties = useSectionVisible("warranties")
  const showLoans = useSectionVisible("loans")
  const showMaintenance = useSectionVisible("maintenance")
  const heldLoans = useLoansForCommodity(id, { enabled: enabled && !showLoans })
  const heldServices = useServicesForCommodity(id, { enabled: enabled && !showMaintenance })
  const heldSchedules = useMaintenanceForCommodity(id, {
    enabled: enabled && !showMaintenance,
  })
  const hiddenTabs = new Set<TabKey>()
  if (!showWarranties && commodity && !commodity.warranty_expires_at && !commodity.warranty_notes) {
    hiddenTabs.add("warranty")
  }
  if (!showLoans && heldLoans.isSuccess && heldLoans.data.loans.length === 0) {
    hiddenTabs.add("lend")
  }
  if (!showMaintenance && heldServices.isSuccess && heldServices.data.services.length === 0) {
    hiddenTabs.add("service")
  }
  if (!showMaintenance && heldSchedules.isSuccess && heldSchedules.data.schedules.length === 0) {
    hiddenTabs.add("maintenance")
  }
  const tab: TabKey = hiddenTabs.has(requestedTab) ? "details" : requestedTab

  const areaName = useMemo(() => {
    const map = new Map<string, string>()
    for (const a of areas.data ?? []) {
      if (a.id) map.set(a.id, a.name ?? "")
    }
    return (areaId?: string) => (areaId ? (map.get(areaId) ?? "") : "")
  }, [areas.data])

  // areaLabel mirrors the design mock's `areaLabel(areaId)` —
  // "{Location.name} · {Area.name}" when both exist, fall back to
  // just the area name (mock fallback), then empty string. Drives
  // the Location row in DetailsTab so the user sees both
  // breadcrumb levels at a glance instead of a single area name.
  const areaLabel = useMemo(() => {
    const areaById = new Map<string, { name: string; locationId?: string }>()
    for (const a of areas.data ?? []) {
      if (a.id) areaById.set(a.id, { name: a.name ?? "", locationId: a.location_id })
    }
    const locationById = new Map<string, string>()
    for (const l of locations.data ?? []) {
      if (l.id) locationById.set(l.id, l.name ?? "")
    }
    return (areaId?: string) => {
      if (!areaId) return ""
      const area = areaById.get(areaId)
      if (!area) return ""
      const locationName = area.locationId ? (locationById.get(area.locationId) ?? "") : ""
      return locationName ? `${locationName} · ${area.name}` : area.name
    }
  }, [areas.data, locations.data])

  // The detail page heading mirrors the commodity name; once it's
  // loaded we update the document title via RouteTitle so browser
  // tabs are useful in long sessions. Sheet mode is gated out — the
  // backdrop list page already owns the tab title, and stomping it
  // when the user pops the sheet would leak the item name onto the
  // list URL too (copilot review).
  useEffect(() => {
    if (isSheet) return
    if (!commodity?.id) return
    if (typeof document !== "undefined") {
      document.title = commodity.name
        ? `${commodity.name} — Inventario`
        : t("commodities:detail.documentTitle")
    }
  }, [isSheet, commodity?.id, commodity?.name, t])

  if (detail.isLoading) {
    return (
      <PageFrame
        isSheet={isSheet}
        className={errorWrapperClass}
        data-testid="commodity-detail-loading"
      >
        <Skeleton className="h-8 w-64" />
        <Skeleton className="mt-2 h-4 w-32" />
        <div className="mt-6 grid grid-cols-2 gap-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-12 rounded-md" />
          ))}
        </div>
      </PageFrame>
    )
  }
  if (detail.isError) {
    return (
      <PageFrame isSheet={isSheet} className={errorWrapperClass}>
        <Alert variant="destructive" data-testid="commodity-detail-error">
          <AlertTitle>{t("commodities:detail.errorTitle")}</AlertTitle>
          <AlertDescription>{t("commodities:detail.errorDescription")}</AlertDescription>
        </Alert>
        <Button variant="ghost" className="mt-4 gap-1" onClick={() => navigate(-1)}>
          <ArrowLeft className="size-4" aria-hidden="true" />
          {t("commodities:detail.backToList")}
        </Button>
      </PageFrame>
    )
  }
  if (!commodity) {
    return (
      <PageFrame isSheet={isSheet} className={errorWrapperClass}>
        <Card data-testid="commodity-detail-not-found">
          <CardHeader>
            <CardTitle>{t("commodities:detail.notFoundTitle")}</CardTitle>
            <CardDescription>{t("commodities:detail.notFoundDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={() => navigate(-1)}>{t("commodities:detail.backToList")}</Button>
          </CardContent>
        </Card>
      </PageFrame>
    )
  }

  const status = commodity.status as CommodityStatusValue | undefined
  const tone = status ? COMMODITY_STATUS_TONES[status] : ""
  const type = commodity.type
  const typeLabel = itemTypes.get(type)?.label ?? ""
  // Per the BE, `original_price` is denominated in the purchase
  // currency (`original_price_currency`); `converted_original_price`
  // and `current_price` are denominated in the group currency.
  // Pass both down so DetailsTab can format each row correctly rather
  // than mixing the symbols.
  const groupCurrency = currentGroup?.group_currency ?? "USD"
  const purchaseCurrency = commodity.original_price_currency ?? groupCurrency
  const currency = groupCurrency // for the (deprecated) edit-dialog default
  const listHref = slug ? `/g/${encodeURIComponent(slug)}/commodities` : "#"
  // Back goes to the list the item was opened from, as the user left it.
  const backHref = currentContext ? listContextHref(currentContext) : listHref
  // Bare full-page detail href (no `state.background`). The sheet's
  // "View full details" CTA navigates here so the router mounts the
  // full-page <CommodityDetailPage> (all 7 tabs) rather than re-opening
  // the slim sheet overlay — see app/router.tsx. `null` until we have a
  // real id so the CTA hides on the (unreachable) sheet-without-id case.
  const detailHref = slug && commodity.id ? itemHref(commodity.id, currentContext) : null
  const printHref =
    slug && commodity.id
      ? `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodity.id)}/print`
      : "#"
  const labelHref = slug && commodity.id ? labelsPageHref(slug, { items: [commodity.id] }) : "#"
  const assetId = formatAssetId(commodity.asset_number)

  // Save from the step-by-step dialog: closes it and the edit form under it.
  async function handleDialogSave(values: UpdateCommodityRequest) {
    if (!allFieldsOn) return
    const savingOn = allFieldsOn.id
    try {
      await update.mutateAsync({ id: savingOn, values })
    } catch (err) {
      // The dialog says nothing was saved; the edit form under it offers
      // the reload.
      if (isCommodityRevisionConflict(err)) setStaleEditOn(savingOn)
      throw err
    }
    toast.success(t("commodities:toast.updated"))
    setAllFieldsOn(null)
    stopEditing()
  }

  // After a refused save from the edit form: read the item again and open
  // the form afresh on it. A failed read keeps the form and its alert; the
  // global error toast reports the failure.
  async function reloadEditForm() {
    const reloadingOn = id
    setReloadingForm(true)
    try {
      const result = await detail.refetch()
      if (result.isError) return
    } finally {
      setReloadingForm(false)
    }
    setStaleEditOn((on) => (on === reloadingOn ? null : on))
    setFormLoad((n) => n + 1)
  }

  // A save built from the item as the page read it is refused when the item
  // changed since. useUpdateCommodity reads the item again, so trying again
  // saves on the item as it is now.
  function toastSaveError(err: unknown, fallback: string) {
    if (isCommodityRevisionConflict(err)) toast.error(t("commodities:toast.changedMeanwhile"))
    else toastMutationError(err, fallback)
  }

  // A quick edit on the Details tab saves one field and keeps the rest of
  // the record as it is. Resolves false when the save failed. On success it
  // closes the editor that saved, and leaves alone one opened meanwhile.
  async function handleQuickSave(
    field: QuickEditField,
    patch: Partial<Commodity>
  ): Promise<boolean> {
    if (!commodity) return false
    const savedOn = id
    try {
      await update.mutateAsync({ id: savedOn, values: { ...commodity, ...patch } })
      toast.success(t("commodities:toast.updated"))
      setQuickEditOn((open) => (open?.id === savedOn && open.field === field ? null : open))
      return true
    } catch (err) {
      toastSaveError(err, t("commodities:quickEdit.error"))
      return false
    }
  }

  async function handleDelete() {
    if (!commodity?.id) return
    const deleting = commodity.id
    const ok = await confirm({
      title: t("commodities:delete.title", { name: commodity.name ?? "" }),
      description: t("commodities:delete.description"),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (!ok) return
    try {
      await remove.mutateAsync(deleting)
      toast.success(t("commodities:toast.deleted"), { action: undoDelete([deleting]) })
      navigate(listHref)
    } catch (err) {
      toastMutationError(err, t("commodities:toast.deleteError"))
    }
  }

  // CHANGE STATUS quick action: drives the row's status transitions.
  // Forward transitions (in_use → any terminal status) open
  // the StatusTransitionDialog (#1611) so the user can record the
  // status_date / status_note / sale_price triple the BE persists.
  // Revert (terminal → in_use) keeps the lighter useConfirm path and
  // explicitly clears the three metadata fields so the BE's model
  // validation invariant (status_date NULL when status==in_use,
  // sale_price NULL when status!=sold) holds on the next write.
  async function handleStatusTransition(next: CommodityStatusValue) {
    if (!commodity) return
    if (next !== "in_use") {
      setTransitionOn({ id, target: next })
      return
    }
    const reverting = id
    const ok = await confirm({
      title: t("commodities:detail.terminalStatus.revertConfirmTitle"),
      description: t("commodities:detail.terminalStatus.revertConfirmDescription"),
      confirmLabel: t("common:actions.confirm"),
      destructive: false,
    })
    if (!ok) return
    try {
      await update.mutateAsync({
        id: reverting,
        values: {
          ...commodity,
          status: next,
          status_date: undefined,
          status_note: "",
          sale_price: undefined,
        },
      })
      toast.success(t("commodities:toast.statusUpdated"))
    } catch (err) {
      toastSaveError(err, t("commodities:toast.statusUpdateError"))
    }
  }

  // handleTransitionConfirm bridges the StatusTransitionDialog's
  // captured payload to the PATCH wiring. Threads the three new
  // metadata columns alongside the status flip. `sale_price` is
  // included by the dialog only for `sold`; for other targets it
  // stays absent so the BE's per-row invariant holds.
  async function handleTransitionConfirm(payload: StatusTransitionPayload) {
    // The dialog is open only on the item it was opened on, so the item
    // shown is the one it writes.
    if (!commodity || !transitionOn || transitionOn.id !== id) return
    try {
      await update.mutateAsync({
        id: transitionOn.id,
        values: {
          ...commodity,
          status: transitionOn.target,
          status_date: payload.status_date,
          status_note: payload.status_note,
          sale_price: payload.sale_price,
        },
      })
      toast.success(t("commodities:toast.statusUpdated"))
      setTransitionOn(null)
    } catch (err) {
      toastSaveError(err, t("commodities:toast.statusUpdateError"))
      // Re-throw so the dialog can map field-level 422 errors onto its
      // inputs (it stays open). The toast above is the summary.
      throw err
    }
  }

  const pagerLabels = {
    pager: t("commodities:listNav.label"),
    previous: t("commodities:listNav.previous"),
    next: t("commodities:listNav.next"),
  }

  const allFieldsDialog = (
    <CommodityFormDialog
      open={dialogDraft !== null}
      onOpenChange={(open) => {
        if (!open) setAllFieldsOn(null)
      }}
      mode="edit"
      initialValues={dialogDraft ?? commodity}
      linkedItems={detail.data?.linkedItems}
      areas={areas.data ?? []}
      locations={locations.data ?? []}
      defaultCurrency={currency}
      onSubmit={handleDialogSave}
      isPending={itemSaving}
    />
  )

  if (editing && !migrationLock.locked) {
    return (
      <>
        {isSheet ? null : <RouteTitle title={t("commodities:detail.documentTitle")} />}
        <PageFrame
          isSheet={isSheet}
          className={mainWrapperClass}
          data-testid={isSheet ? "sheet-commodity-detail" : "page-commodity-detail"}
        >
          <CommodityEditForm
            // A fresh form per item: "Save and next" lands here with the
            // next item's values. A reload after a refused save opens a
            // fresh one too.
            key={`${commodity.id}:${formLoad}`}
            commodity={commodity}
            areas={areas.data ?? []}
            locations={locations.data ?? []}
            groupCurrency={groupCurrency}
            onSubmit={(req) => update.mutateAsync({ id, values: req })}
            isPending={itemSaving}
            stale={staleEditOn === id}
            onReload={() => void reloadEditForm()}
            reloading={reloadingForm}
            nav={nav}
            hrefFor={(target) => itemHref(target.id, contextAt(target.page), true)}
            onExit={stopEditing}
            onGoTo={(target) => goToNeighbor(target, true)}
            onAllFields={(values) => setAllFieldsOn({ id, values })}
            linkedItems={detail.data?.linkedItems}
            // The mock's form is sized for the side panel; on the full page
            // it keeps a readable column.
            className={isSheet ? undefined : "mx-auto w-full max-w-2xl"}
            // The sheet's close button sits over the header's right end.
            headerClassName={isSheet ? "pr-8" : undefined}
          />
        </PageFrame>
        {allFieldsDialog}
      </>
    )
  }

  return (
    <>
      {/* Document title is owned by the full-page variant only — when
          the sheet is open over the list, the list page already set
          the title and the user expects it to stay. */}
      {isSheet ? null : <RouteTitle title={t("commodities:detail.documentTitle")} />}
      <PageFrame
        isSheet={isSheet}
        className={mainWrapperClass}
        data-testid={isSheet ? "sheet-commodity-detail" : "page-commodity-detail"}
        {...dropZone.bindProps}
      >
        {dropZone.isDragging ? (
          <DropOverlay
            label={t("files:entityPanel.dropOverlay_commodity")}
            hint={t("files:entityPanel.dropHint")}
          />
        ) : null}
        {/* Non-blocking nudge to file an unassigned item under a
            location (#1987). Shows only when the item has no area and
            the group actually has locations to choose from. Hidden while
            a currency migration holds the group lock — its CTA opens the
            same edit dialog the main Edit button gates on the lock, so
            hiding it keeps the banner from being a lock-bypass path. */}
        <AssignLocationBanner
          // Key by commodity id so the banner remounts when the user
          // navigates between items on the same CommodityDetailContent
          // instance — its in-session dismiss is per-item ("returns on the
          // next mount"), not sticky across commodities.
          key={id}
          show={!commodity.area_id && (locations.data?.length ?? 0) > 0 && !migrationLock.locked}
          onAssign={() => {
            if (!isSheet) setTab("details")
            setQuickEdit("location")
          }}
        />
        {/* The Sheet variant renders its own X close button (top-right
            of the panel), so the explicit "Back to list" Link is
            page-only — clicking the X dismisses the overlay and lands
            the user back on the list URL the sheet opened over. */}
        {isSheet ? (
          nav.active ? (
            // The sheet's own close button sits top-right; Previous / Next
            // go on the left of the same strip.
            <div className="flex items-center gap-2 pt-4 pr-8">
              <ListPager
                nav={nav}
                hrefFor={(target) => itemHref(target.id, contextAt(target.page))}
                onNavigate={(target) => goToNeighbor(target, false)}
                labels={pagerLabels}
                testIdPrefix="commodity-detail"
              />
            </div>
          ) : null
        ) : (
          <div className="flex flex-wrap items-center justify-between gap-2">
            <Link
              to={backHref}
              className="text-sm text-muted-foreground hover:underline inline-flex items-center gap-1"
              data-testid="commodity-detail-back"
            >
              <ArrowLeft className="size-3.5" aria-hidden="true" />
              {reviewList ? t("inbox:walk.back") : t("commodities:detail.backToList")}
            </Link>
            {nav.active ? (
              <ListPager
                nav={nav}
                hrefFor={(target) => itemHref(target.id, contextAt(target.page))}
                labels={pagerLabels}
                testIdPrefix="commodity-detail"
              />
            ) : null}
          </div>
        )}
        {nav.active && !nav.loading && !nav.inList ? (
          <p className="text-sm text-muted-foreground" data-testid="commodity-detail-not-in-list">
            {t("commodities:listNav.notInList")}
          </p>
        ) : null}
        {reviewList && canWrite && commodity.draft ? (
          <InboxReviewBar
            position={nav.position}
            total={nav.total}
            hasNext={!!nav.next}
            pending={itemSaving || nav.loading}
            disabled={migrationLock.locked || quickEdit !== null}
            disabledReason={
              migrationLock.locked ? t("errors:lockedDuringMigration") : t("inbox:walk.finishEdit")
            }
            onSkip={() => {
              if (nav.next) goToNeighbor(nav.next, false)
            }}
            onReview={() => void markReviewedAndNext(commodity.name ?? "")}
            className={isSheet ? "mt-3" : undefined}
          />
        ) : null}

        {/* Header (mock parity: `<SheetHeader pt-6 pb-4 px-0>`). The
            title block + status pills row both live inside the
            header in the mock. We do the same shape: avatar +
            title + short_name + type subtitle, then status pills
            below. Page mode keeps `gap-6` flow above so the
            existing visual rhythm is unchanged.
            An item with a cover photo shows it larger than the mock's
            48px tile (#2797, see design-deviations): beside the title
            on the page, from `sm` up, above it on a phone; at 96px in
            the sheet. */}
        <header
          className={
            isSheet
              ? "pt-6 pb-4"
              : commodity.cover
                ? "flex flex-col gap-4 sm:flex-row sm:items-start sm:gap-6"
                : ""
          }
        >
          {!isSheet && commodity.cover ? (
            <CommodityCoverPanel
              commodity={commodity}
              className="w-full shrink-0 sm:w-48 lg:w-56"
            />
          ) : null}
          <div className="min-w-0 flex-1">
            <div className="flex items-start gap-3">
              {isSheet || !commodity.cover ? (
                <CommodityThumb
                  cover={commodity.cover}
                  type={type}
                  icon={itemTypes.get(type)?.Icon}
                  name={commodity.name}
                  size={isSheet && commodity.cover ? 96 : 48}
                  testId="commodity-detail-thumb"
                />
              ) : null}
              <div className="min-w-0 flex-1 pr-6">
                <h1
                  className={cn(
                    "font-semibold leading-tight tracking-tight",
                    isSheet ? "text-lg" : "text-2xl"
                  )}
                  data-testid="commodity-detail-name"
                >
                  {commodity.name}
                </h1>
                {commodity.short_name && commodity.short_name !== commodity.name ? (
                  <p
                    className="text-xs font-mono text-muted-foreground mt-0.5"
                    data-testid="commodity-detail-short-name"
                  >
                    {commodity.short_name}
                  </p>
                ) : null}
                {brandModelText(commodity) ? (
                  <p
                    className="text-sm text-muted-foreground mt-0.5"
                    data-testid="commodity-detail-brand-model"
                  >
                    {brandModelText(commodity)}
                  </p>
                ) : null}
                {type ? (
                  <p className="text-sm text-muted-foreground mt-0.5">
                    {slug ? (
                      // The items list with every item of this type, inactive
                      // ones included, since this item may be one of them.
                      <Link
                        to={commodityListHref(slug, { types: [type], includeInactive: true })}
                        className="hover:text-foreground hover:underline"
                        data-testid="commodity-detail-type-link"
                      >
                        {typeLabel}
                      </Link>
                    ) : (
                      typeLabel
                    )}
                  </p>
                ) : null}
              </div>
            </div>

            {/* Status pills row — commodity status + warranty + days
              remaining. Mock places it inside SheetHeader directly
              under the title block (`pt-1`). */}
            <div
              className="flex flex-wrap items-center gap-2 pt-1"
              data-testid="commodity-detail-pills"
            >
              {/* Asset ID (#2807): the number printed on the item's label.
                Not in the mock; see design-deviations. */}
              {assetId ? (
                <Badge
                  variant="outline"
                  className="font-mono text-xs"
                  title={t("commodities:detail.assetId")}
                  data-testid="commodity-detail-asset-id"
                >
                  <span className="sr-only">{t("commodities:detail.assetId")} </span>
                  {assetId}
                </Badge>
              ) : null}
              {status ? (
                <span
                  className={cn(
                    "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium border",
                    tone || "border-border text-foreground"
                  )}
                  data-testid="commodity-detail-status-pill"
                >
                  <CircleDot className="size-3" aria-hidden="true" />
                  {t(`commodities:status.${status}`)}
                </span>
              ) : null}
              {commodity.draft ? (
                <Badge variant="outline" className="border-dashed text-xs">
                  {t("commodities:list.draftBadge")}
                </Badge>
              ) : null}
              {/* #1554: bundles don't carry a warranty — hide the pill
                + days-remaining row entirely so the header doesn't
                advertise an attribute the row can't have. The
                Warranty tab body shows the explanatory empty state. */}
              {(commodity.count ?? 0) > 1 ? null : (
                <>
                  <WarrantyBadge
                    source={{ warranty_expires_at: commodity.warranty_expires_at }}
                    data-testid="commodity-detail-warranty-pill"
                  />
                  {(() => {
                    const days = warrantyDaysRemaining(commodity.warranty_expires_at)
                    return days !== null && days > 0 ? (
                      <span className="text-xs text-muted-foreground">
                        {t("commodities:detail.warranty.daysRemaining", { count: days })}
                      </span>
                    ) : null
                  })()}
                </>
              )}
            </div>
          </div>
        </header>

        {/* Action row — `flex gap-2 pb-4 flex-wrap` per the mock.
            Edit (flex-1) + Insurance Report + icon-only destructive
            Delete. Print is page-only (the mock has no Print
            button); direct landings still have it on the page
            chrome where it doesn't crowd the action row. */}
        <div className={cn("flex flex-wrap items-center gap-2", isSheet && "pb-4")}>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={startEditing}
            data-testid="commodity-detail-edit"
            className="flex-1 gap-1.5"
            disabled={migrationLock.locked || setDraft.isPending}
            title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
            aria-disabled={migrationLock.locked || undefined}
          >
            <Pencil className="size-3.5" aria-hidden="true" />
            {t("commodities:detail.edit")}
          </Button>
          {slug && commodity.id ? (
            <Button
              asChild
              type="button"
              variant="outline"
              size="sm"
              className="gap-1.5"
              data-testid="commodity-detail-insurance"
            >
              <Link
                to={`/g/${encodeURIComponent(slug)}/reports/insurance?mode=item&item=${encodeURIComponent(commodity.id)}`}
              >
                <BarChart3 className="size-3.5" aria-hidden="true" />
                {t("commodities:detail.insuranceReport")}
              </Link>
            </Button>
          ) : null}
          {isSheet ? null : (
            <Button
              asChild
              type="button"
              variant="outline"
              size="sm"
              title={t("commodities:detail.print")}
              aria-label={t("commodities:detail.print")}
            >
              <Link to={printHref} data-testid="commodity-detail-print">
                <Printer className="size-3.5" aria-hidden="true" />
              </Link>
            </Button>
          )}
          {/* Print label (#2807) — page-only like Print above. */}
          {isSheet ? null : (
            <Button
              asChild
              type="button"
              variant="outline"
              size="sm"
              title={t("commodities:detail.printLabel")}
              aria-label={t("commodities:detail.printLabel")}
            >
              <Link to={labelHref} data-testid="commodity-detail-print-label">
                <QrCode className="size-3.5" aria-hidden="true" />
              </Link>
            </Button>
          )}
          {/* Delete uses `size="sm"` (h-8 px-3 — slightly wider
              than `size="icon"`'s 8x8 square) to match the mock's
              same-row Edit / Insurance Report buttons. The icon
              child stays the same; the destructive tone comes from
              the className. */}
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="text-destructive hover:bg-destructive/10"
            onClick={handleDelete}
            data-testid="commodity-detail-delete"
            title={
              migrationLock.locked
                ? t("errors:lockedDuringMigration")
                : t("commodities:detail.delete")
            }
            aria-label={t("commodities:detail.delete")}
            disabled={migrationLock.locked}
            aria-disabled={migrationLock.locked || undefined}
          >
            <Trash2 className="size-3.5" aria-hidden="true" />
          </Button>
        </div>

        {/* CHANGE STATUS bar — `mb-4 rounded-xl border border-border
            bg-muted/30 p-3 space-y-2` lifted from the mock. Only
            meaningful while the item is still `in_use`. */}
        {status === "in_use" ? (
          <div
            className={cn(
              "rounded-xl border border-border bg-muted/30 p-3 space-y-2",
              isSheet && "mb-4"
            )}
            data-testid="commodity-detail-change-status"
          >
            <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
              {t("commodities:detail.statusTransition.heading")}
            </p>
            <div className="flex flex-wrap gap-1.5">
              {TERMINAL_STATUSES.map((s) => (
                <Button
                  key={s}
                  type="button"
                  variant="outline"
                  size="sm"
                  className={cn("gap-1.5 text-xs h-7", STATUS_TRANSITION_TEXT_TONES[s])}
                  onClick={() => handleStatusTransition(s)}
                  data-testid={`commodity-detail-transition-${s}`}
                  disabled={itemSaving || migrationLock.locked}
                  title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
                  aria-disabled={migrationLock.locked || undefined}
                >
                  {t(`commodities:status.${s}`)}
                </Button>
              ))}
            </div>
          </div>
        ) : null}

        {/* Terminal-status info card (#1530 item 1 + #1611) — surfaces
            the current status, the metadata captured during the
            transition (date / note / sale_price), and the explicit
            "Revert to In Use" CTA the mock requires (lines 736-762).
            Each metadata row gates on its column being set, so rows
            that pre-date #1611 (NULL metadata) collapse to just the
            label + Revert pair — the prior shipped behaviour. */}
        {status && status !== "in_use" ? (
          <div
            className={cn(
              "rounded-xl border p-3 space-y-1",
              tone || "border-border bg-muted text-foreground",
              isSheet && "mb-4"
            )}
            role="status"
            data-testid="commodity-detail-terminal-status"
          >
            <div className="flex items-center gap-1.5">
              <TriangleAlert className="size-3.5 shrink-0" aria-hidden="true" />
              <p
                className="text-xs font-semibold"
                data-testid="commodity-detail-terminal-status-label"
              >
                {t(`commodities:status.${status}`)}
              </p>
            </div>
            {commodity.status_date ? (
              <p
                className="text-xs text-muted-foreground"
                data-testid="commodity-detail-terminal-status-date"
              >
                {t("commodities:detail.terminalStatus.statusDate", {
                  date: formatDate(commodity.status_date),
                })}
              </p>
            ) : null}
            {commodity.status_note ? (
              <p
                className="text-xs text-muted-foreground"
                data-testid="commodity-detail-terminal-status-note"
              >
                {commodity.status_note}
              </p>
            ) : null}
            {commodity.sale_price != null ? (
              <p
                className="text-xs text-muted-foreground"
                data-testid="commodity-detail-terminal-status-sale-price"
              >
                {t("commodities:detail.terminalStatus.salePrice", {
                  value: formatCurrency(Number(commodity.sale_price), purchaseCurrency),
                })}
              </p>
            ) : null}
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="-mb-1 mt-1 h-6 px-1 text-xs text-foreground"
              onClick={() => handleStatusTransition("in_use")}
              disabled={itemSaving || migrationLock.locked}
              title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
              aria-disabled={migrationLock.locked || undefined}
              data-testid="commodity-detail-revert-status"
            >
              {t("commodities:detail.terminalStatus.revert")}
            </Button>
          </div>
        ) : null}

        {/* The 7-tab strip overflows the narrow `sm:max-w-lg` Sheet,
            so in sheet mode it's dropped entirely (#1965). The mock
            only ever had 4 tabs; the app grew to 7 (Details / Warranty
            / Files / Lend / Service / Supplies / Maintenance) and the
            right-hand tabs clip past the panel edge. Page mode keeps
            the full strip untouched. */}
        {isSheet ? null : (
          <Tabs
            value={tab}
            onChange={setTab}
            fileCount={fileCount}
            variant={variant}
            hidden={hiddenTabs}
          />
        )}

        {/* Tab content gets `mt-4` in sheet mode to mirror the
            mock's `<TabsContent className="mt-4 space-y-0">`. Page
            mode keeps its parent `gap-6` flow. */}
        <div className={isSheet ? "mt-4" : ""}>
          {isSheet ? (
            // Sheet = slim quick-peek: just the Details fields plus a
            // CTA into the full tabbed page. We deliberately skip the
            // `CommodityHistoryTimeline` here (page-only) so the peek
            // stays short. The "View full details" button navigates to
            // the BARE detail href with NO `state.background` so the
            // router mounts the full-page <CommodityDetailPage> (all 7
            // tabs) instead of re-opening this sheet — see app/router.tsx.
            <>
              <DetailsTab
                key={commodity.id}
                commodity={commodity}
                groupCurrency={groupCurrency}
                purchaseCurrency={purchaseCurrency}
                areaName={areaName(commodity.area_id)}
                areaLabel={areaLabel(commodity.area_id)}
                variant={variant}
                quickEdit={migrationLock.locked ? null : quickEdit}
                onQuickEdit={migrationLock.locked ? undefined : setQuickEdit}
                onQuickSave={handleQuickSave}
                saving={itemSaving}
                areas={areas.data ?? []}
                locations={locations.data ?? []}
                linkedItems={detail.data?.linkedItems}
              />
              {detailHref ? (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="mt-4 w-full gap-1.5"
                  data-testid="commodity-detail-view-full"
                  onClick={() => navigate(detailHref)}
                >
                  <Maximize2 className="size-3.5" aria-hidden="true" />
                  {t("commodities:detail.viewFullDetails")}
                </Button>
              ) : null}
            </>
          ) : tab === "details" ? (
            <>
              <DetailsTab
                key={commodity.id}
                commodity={commodity}
                groupCurrency={groupCurrency}
                purchaseCurrency={purchaseCurrency}
                areaName={areaName(commodity.area_id)}
                areaLabel={areaLabel(commodity.area_id)}
                variant={variant}
                quickEdit={migrationLock.locked ? null : quickEdit}
                onQuickEdit={migrationLock.locked ? undefined : setQuickEdit}
                onQuickSave={handleQuickSave}
                saving={itemSaving}
                areas={areas.data ?? []}
                locations={locations.data ?? []}
                linkedItems={detail.data?.linkedItems}
              />
              {commodity.id ? (
                // One instance per item, as the tabs below; the key differs
                // from the Details tab's beside it.
                <CommodityRemindersSection
                  key={`reminders-${id}`}
                  commodityId={commodity.id}
                  className="my-6"
                />
              ) : null}
              {commodity.id ? <CommodityHistoryTimeline commodityId={commodity.id} /> : null}
            </>
          ) : tab === "warranty" ? (
            <WarrantyTab
              commodity={commodity}
              onUploadReceipt={() => {
                // #2012: open the upload dialog directly (commodity
                // preselected) — the same dialog the Files-tab dropzone
                // opens — instead of just switching tabs and leaving the
                // user to hunt for the dropzone. Switch the underlying
                // tab to Files too so the newly uploaded receipt is
                // visible once the dialog closes.
                setTab("files")
                setPendingDropFiles([])
                setUploadFor(id)
              }}
            />
          ) : tab === "lend" ? (
            // The tabs write to the item from their own dialogs. One
            // instance per item closes an open dialog when the page moves
            // to another item, so it never writes to that one.
            <LendTab
              key={id}
              commodityId={commodity?.id ?? id}
              commodityCount={commodity?.count ?? undefined}
              canStart={showLoans}
            />
          ) : tab === "service" ? (
            <ServiceTab
              key={id}
              commodityId={commodity?.id ?? id}
              commodityCount={commodity?.count ?? undefined}
              canStart={showMaintenance}
            />
          ) : tab === "supplies" ? (
            <SuppliesTab key={id} commodityId={commodity?.id ?? id} />
          ) : tab === "maintenance" ? (
            <MaintenanceTab
              key={id}
              commodityId={commodity?.id ?? id}
              commodityCount={commodity?.count ?? undefined}
              canStart={showMaintenance}
            />
          ) : (
            <FilesTab
              key={id}
              commodityId={commodity?.id ?? id}
              commodityName={commodity?.name}
              coverState={{
                // Explicit override (issue #1451 option B) takes precedence;
                // first_photo is the auto-pick when no override is set.
                current: commodity?.cover_file_id ?? undefined,
                auto:
                  commodity?.cover && commodity.cover.source === "first_photo"
                    ? commodity.cover.fileId
                    : undefined,
              }}
              onSetCover={(fileId) => {
                setCover.mutate(
                  { id, fileId },
                  {
                    onSuccess: () => {
                      toast.success(
                        fileId
                          ? t("commodities:cover.setSuccess", {
                              defaultValue: "Cover photo updated.",
                            })
                          : t("commodities:cover.clearSuccess", {
                              defaultValue: "Cover photo cleared.",
                            })
                      )
                    },
                    onError: () =>
                      toast.error(
                        t("commodities:cover.error", {
                          defaultValue: "Couldn't update the cover photo.",
                        })
                      ),
                  }
                )
              }}
              coverBusy={setCover.isPending}
              onAttachClick={() => {
                setPendingDropFiles([])
                setUploadFor(id)
              }}
            />
          )}
        </div>
      </PageFrame>

      {allFieldsDialog}

      <StatusTransitionDialog
        open={transitionTarget !== null}
        onOpenChange={(o) => {
          if (!o) setTransitionOn(null)
        }}
        targetStatus={transitionTarget}
        purchaseCurrency={purchaseCurrency}
        onSubmit={handleTransitionConfirm}
        isPending={itemSaving}
      />

      <UploadFilesDialog
        open={uploadOpen}
        onOpenChange={(open) => {
          setUploadFor(open ? id : null)
          if (!open) setPendingDropFiles([])
        }}
        linkedEntity={{
          type: "commodity",
          id: uploadFor ?? id,
          name: commodity?.name,
        }}
        initialFiles={pendingDropFiles}
        // Issue #1451 option C — promote the first uploaded photo to
        // the cover when the commodity has no explicit cover yet. The
        // checkbox in the metadata step defaults to ON for that case.
        // "Has a cover" means EITHER an explicit `cover_file_id` set on
        // the row (option B) OR an auto-picked first-photo cover (option
        // A). Only `cover_file_id` would silently re-promote new uploads
        // on commodities that already have a perfectly fine first-photo
        // cover (Copilot review on PR #1504). `commodity.cover` is the
        // resolved cover from `meta.cover`, set whenever either path
        // produced a usable image.
        commodityHasCover={!!commodity?.cover}
        onSetCover={(fileId) => {
          setCover.mutate(
            { id: uploadFor ?? id, fileId },
            {
              onError: () =>
                toast.error(
                  t("commodities:cover.error", {
                    defaultValue: "Couldn't update the cover photo.",
                  })
                ),
            }
          )
        }}
      />
    </>
  )
}

interface TabsProps {
  value: TabKey
  onChange: (v: TabKey) => void
  // File count surfaced as a badge on the Files tab — mirrors the
  // mock's `<TabsTrigger value="files">Files {n > 0 && <Badge>}`.
  // Zero is hidden so the tab strip doesn't carry visual debt for
  // every empty commodity.
  fileCount?: number
  // `"sheet"` flips the tab strip to `w-full` so it spans the
  // narrower panel; `"page"` keeps the existing left-clumped
  // layout that fits the wider canvas.
  variant?: "page" | "sheet"
  // Tabs of sections the group hides, for an item without data in them.
  hidden?: ReadonlySet<TabKey>
}

function Tabs({ value, onChange, fileCount = 0, variant = "page", hidden }: TabsProps) {
  const { t } = useTranslation()
  const isSheet = variant === "sheet"
  // Files tab gets a leading icon — mock parity (`<Paperclip
  // size-3.5 />` inside `<TabsTrigger>`). Other tabs stay
  // text-only; the strip would feel cluttered if every label
  // sprouted an icon.
  const tabs: {
    key: TabKey
    label: string
    count?: number
    icon?: typeof Paperclip
  }[] = [
    { key: "details", label: t("commodities:detail.tabs.details") },
    { key: "warranty", label: t("commodities:detail.tabs.warranty") },
    {
      key: "files",
      label: t("commodities:detail.tabs.files"),
      count: fileCount,
      icon: Paperclip,
    },
    { key: "lend", label: t("commodities:detail.tabs.lend") },
    { key: "service", label: t("commodities:detail.tabs.service") },
    {
      key: "supplies",
      label: t("commodities:detail.tabs.supplies", { defaultValue: "Supplies" }),
    },
    {
      key: "maintenance",
      label: t("commodities:detail.tabs.maintenance", { defaultValue: "Maintenance" }),
    },
  ]
  return (
    <div
      role="tablist"
      className={cn(
        "flex",
        // Sheet variant: no full-width bottom border on the
        // container — the mock's `<TabsList variant="line">` only
        // shows the underline under the *active* trigger. Page
        // mode keeps the strip-wide divider for the wider chrome.
        // `flex-1` triggers below spread the labels evenly across
        // the panel instead of clumping left.
        isSheet ? "w-full gap-1" : "gap-1 border-b border-border"
      )}
      data-testid="commodity-detail-tabs"
    >
      {tabs
        .filter((tb) => !hidden?.has(tb.key))
        .map((tb) => {
          const Icon = tb.icon
          return (
            <button
              key={tb.key}
              role="tab"
              type="button"
              aria-selected={value === tb.key}
              onClick={() => onChange(tb.key)}
              className={cn(
                "inline-flex items-center gap-1.5 py-2 text-sm border-b-2 -mb-px transition-colors",
                // Sheet triggers grow to equal width + center their
                // labels so the tab strip mirrors the mock's
                // `flex-1 justify-center` `<TabsTrigger>` shape.
                isSheet ? "flex-1 justify-center px-2" : "px-3",
                value === tb.key
                  ? "border-primary text-foreground font-semibold"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              )}
              data-testid={`commodity-detail-tab-${tb.key}`}
            >
              {Icon ? <Icon className="size-3.5" aria-hidden="true" /> : null}
              {tb.label}
              {tb.count && tb.count > 0 ? (
                <span
                  className="flex size-4 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-foreground"
                  data-testid={`commodity-detail-tab-${tb.key}-count`}
                >
                  {tb.count}
                </span>
              ) : null}
            </button>
          )
        })}
    </div>
  )
}

interface DetailsTabProps {
  commodity: import("@/features/commodities/api").Commodity
  // Currency the BE stores `original_price` in (taken from
  // `original_price_currency`).
  purchaseCurrency: string
  // Currency the BE stores `converted_original_price` and
  // `current_price` in — always the active group currency.
  groupCurrency: string
  areaName: string
  // "{Location.name} · {Area.name}" breadcrumb for the Location
  // row (mock parity). Falls back to the area name alone when the
  // location can't be resolved; empty string when the area itself
  // is missing. Sheet mode prefers this richer label since the
  // panel is narrow but the user still wants the full context.
  areaLabel: string
  // Variant matches the parent's: `"sheet"` swaps the 2-col grid
  // for a vertical icon|label-value list (matching the mock and
  // staying readable inside the narrower Sheet panel); `"page"`
  // keeps the existing 2-col layout that fills the wider canvas.
  variant?: "page" | "sheet"
  // Quick edits (#2795): the field being edited in place, a setter that
  // opens or closes one (absent = quick edits are off, e.g. during a
  // currency migration), and the save that patches one field.
  quickEdit?: QuickEditField | null
  onQuickEdit?: (field: QuickEditField | null) => void
  onQuickSave?: (field: QuickEditField, patch: Partial<Commodity>) => Promise<boolean>
  saving?: boolean
  areas?: AreaOption[]
  locations?: LocationOption[]
  // Names of the items the item's link fields point at (meta.linked_items).
  linkedItems?: Record<string, LinkedItemSummary>
}

type QuickEditField = "location" | "tags" | "notes"

function DetailsTab({
  commodity,
  purchaseCurrency,
  groupCurrency,
  areaName,
  areaLabel,
  variant = "page",
  quickEdit = null,
  onQuickEdit,
  onQuickSave,
  saving = false,
  areas = [],
  locations = [],
  linkedItems,
}: DetailsTabProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const groupSlug = currentGroup?.slug ?? ""
  const customFields = useCustomFields().data
  const customValues = commodity.custom_fields as CustomFieldValues | undefined
  const shownCustomFields = (customFields ?? []).filter((f) => hasValue(customValues, f.id))
  const tagIndex = useTagIndex("commodity")
  const sellers = useSellers()
  // The other system the item's document came from (#2809), if any.
  const source = useItemSource(commodity.id).data ?? null
  const noValue = t("commodities:detail.noValue")
  const seller = commodity.seller_id
    ? sellers.data?.find((s) => s.id === commodity.seller_id)
    : undefined
  const areaText = (isSheetVariant: boolean) => (isSheetVariant ? areaLabel : areaName) || noValue
  // Tags, the area and the type link to the items list filtered by them
  // alone, inactive items included.
  const tagPills = (commodity.tags ?? []).map((slug) => {
    const tag = tagIndex.resolve(slug)
    return (
      <TagPill
        key={slug}
        label={tag.label}
        color={tag.color}
        size="xs"
        href={groupSlug ? commodityTagHref(groupSlug, slug) : undefined}
        data-testid="commodity-detail-tag"
      />
    )
  })
  // Sheet mode shows the breadcrumb-style "Location · Area"
  // value with a "Location" label (mock parity); page mode keeps
  // the bare "Area" / area-name pair to match the existing column
  // layout where adjacent rows already carry their own context.
  const isSheetVariant = variant === "sheet"
  const locationLabel = isSheetVariant
    ? t("commodities:detail.fields.location")
    : t("commodities:detail.fields.area")
  const tagsLabel = t("commodities:detail.fields.tags")
  const notesLabel = t("commodities:detail.fields.comments")
  const cancelQuickEdit = () => onQuickEdit?.(null)
  const saveField = (field: QuickEditField) => (patch: Partial<Commodity>) =>
    onQuickSave?.(field, patch) ?? Promise.resolve(false)
  // The pencil next to a field's label; hidden while that field is open.
  const editButton = (field: QuickEditField, label: string) =>
    onQuickEdit && quickEdit !== field ? (
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        className="text-muted-foreground"
        onClick={() => onQuickEdit(field)}
        aria-label={t("commodities:quickEdit.edit", { field: label })}
        data-testid={`commodity-quick-edit-${field}`}
      >
        <Pencil aria-hidden="true" />
      </Button>
    ) : null
  const rows: { icon: typeof MapPin; label: string; value: React.ReactNode; testId?: string }[] = [
    {
      icon: Package,
      label: t("commodities:detail.fields.count"),
      value: commodity.count ?? noValue,
    },
    {
      icon: Calendar,
      label: t("commodities:detail.fields.purchaseDate"),
      testId: "commodity-detail-purchase-date",
      value: commodity.purchase_date
        ? formatDate(commodity.purchase_date as string, { style: "short" })
        : noValue,
    },
    {
      icon: Store,
      label: t("commodities:detail.fields.seller"),
      testId: "commodity-detail-seller",
      // The seller links to the items list of everything bought from it.
      value: seller ? (
        groupSlug ? (
          <Link
            to={commoditySellerHref(groupSlug, seller.id)}
            className="hover:underline"
            data-testid="commodity-detail-seller-link"
          >
            {seller.name}
          </Link>
        ) : (
          seller.name
        )
      ) : commodity.seller_id && sellers.isLoading ? (
        "…"
      ) : (
        noValue
      ),
    },
    ...(source
      ? [
          {
            icon: FileInput,
            label: t("commodities:detail.fields.source"),
            testId: "commodity-detail-source",
            value: <DocumentSource source={source} variant="value" />,
          },
        ]
      : []),
    {
      icon: DollarSign,
      label: t("commodities:detail.fields.originalPrice"),
      testId: "commodity-detail-original-price",
      // After a currency migration the BE freezes the per-row
      // "as purchased" amount in `acquisition_price` /
      // `acquisition_currency` (issue #202 §2 Case A). Surface it
      // as a subdued line under the live OriginalPrice so users can
      // still see the original purchase amount in the original
      // currency. Hidden when the BE didn't freeze a value (fresh
      // commodity → live OriginalPrice already IS the purchase
      // value, so no second line is needed).
      value: (
        <span className="flex flex-col gap-0.5">
          <span>
            {formatPrice(commodity.original_price, purchaseCurrency, noValue, formatCurrency)}
          </span>
          {commodity.acquisition_price != null && commodity.acquisition_currency ? (
            <span
              className="text-xs text-muted-foreground"
              data-testid="commodity-detail-acquisition"
            >
              {t("commodities:detail.acquisitionPrice", {
                // formatCurrency injects the locale-correct symbol /
                // code on its own; the i18n string interpolates the
                // already-formatted value. Translations that want a
                // bare numeric + uppercase code (e.g. "1,234.56 USD")
                // can switch to a number-only formatter and append
                // {{currency}} themselves once we surface that knob.
                price: formatCurrency(
                  Number(commodity.acquisition_price),
                  commodity.acquisition_currency
                ),
              })}
            </span>
          ) : null}
        </span>
      ),
    },
    {
      icon: DollarSign,
      label: t("commodities:detail.fields.convertedOriginalPrice"),
      testId: "commodity-detail-converted-price",
      value: formatPrice(
        commodity.converted_original_price,
        groupCurrency,
        noValue,
        formatCurrency
      ),
    },
    {
      icon: DollarSign,
      label: t("commodities:detail.fields.currentPrice"),
      testId: "commodity-detail-current-price",
      value: formatPrice(commodity.current_price, groupCurrency, noValue, formatCurrency),
    },
    {
      icon: Hash,
      label: t("commodities:detail.fields.serialNumber"),
      value: commodity.serial_number || noValue,
    },
  ]
  // Sheet variant renders the rows directly on the SheetContent
  // surface (NO outer Card / no nested rounded border) — the mock's
  // `<TabsContent className="mt-4 space-y-0">` pattern. Each row +
  // Separator pair is the entire visual rhythm; the panel chrome
  // is everything around it. Page variant keeps the wider 2-col
  // grid wrapped in the existing Card.
  const isSheet = variant === "sheet"
  const containerClass = isSheet ? "flex flex-col" : "grid grid-cols-1 sm:grid-cols-2 gap-4 py-6"
  const fullWidthClass = isSheet ? "" : "sm:col-span-2"
  const separatorClass = isSheet ? "" : "sm:col-span-2"
  const Outer: React.ElementType = isSheet ? "div" : Card
  const Inner: React.ElementType = isSheet ? "div" : CardContent
  return (
    <Outer data-testid="commodity-detail-details">
      <Inner className={containerClass}>
        <DetailRow
          icon={MapPin}
          label={locationLabel}
          action={editButton("location", locationLabel)}
          value={
            quickEdit === "location" ? (
              <LocationQuickEditor
                commodity={commodity}
                areas={areas}
                locations={locations}
                saving={saving}
                onSave={saveField("location")}
                onCancel={cancelQuickEdit}
              />
            ) : groupSlug && commodity.area_id ? (
              <Link
                to={commodityListHref(groupSlug, {
                  areaId: commodity.area_id,
                  includeInactive: true,
                })}
                className="hover:underline"
                data-testid="commodity-detail-area-link"
              >
                {areaText(isSheetVariant)}
              </Link>
            ) : (
              areaText(isSheetVariant)
            )
          }
          variant={variant}
          wide={quickEdit === "location"}
          testId="commodity-detail-location"
        />
        {rows.map((r) => (
          <DetailRow
            key={r.label}
            icon={r.icon}
            label={r.label}
            value={r.value}
            variant={variant}
            withDivider={isSheet}
            testId={r.testId}
          />
        ))}
        {/* Auxiliary fields (tags / urls / notes / extra serials /
            part numbers). Sheet mode threads them through the same
            DetailRow shape as the main rows so the labels stay
            "Tags" / "Notes" (not "TAGS" / "NOTES" via DetailLabel's
            uppercase tracking) and the icon-on-left layout is
            uniform. Page mode keeps the existing 2-col grid block
            with DetailLabel on top. */}
        <FieldBlock
          icon={Tag}
          label={tagsLabel}
          action={editButton("tags", tagsLabel)}
          variant={variant}
          className={fullWidthClass}
          testId="commodity-detail-tags"
        >
          {quickEdit === "tags" ? (
            <TagsQuickEditor
              commodity={commodity}
              saving={saving}
              onSave={saveField("tags")}
              onCancel={cancelQuickEdit}
            />
          ) : commodity.tags && commodity.tags.length > 0 ? (
            <div className={cn("flex flex-wrap gap-1", isSheet && "mt-0.5")}>{tagPills}</div>
          ) : (
            noValue
          )}
        </FieldBlock>
        {Array.isArray(commodity.urls) && commodity.urls.length > 0 ? (
          isSheet ? (
            <DetailRow
              icon={ExternalLink}
              label={t("commodities:detail.fields.urls")}
              value={
                <ul className="text-sm">
                  {(commodity.urls as unknown as string[]).map((u, i) => (
                    <li key={i}>
                      <a
                        href={u}
                        target="_blank"
                        rel="noreferrer noopener"
                        className="text-primary hover:underline"
                      >
                        {u}
                      </a>
                    </li>
                  ))}
                </ul>
              }
              variant={variant}
              withDivider
            />
          ) : (
            <div className={cn("flex flex-col gap-1.5", fullWidthClass)}>
              <DetailLabel icon={ExternalLink} label={t("commodities:detail.fields.urls")} />
              <ul className="text-sm">
                {(commodity.urls as unknown as string[]).map((u, i) => (
                  <li key={i}>
                    <a
                      href={u}
                      target="_blank"
                      rel="noreferrer noopener"
                      className="text-primary hover:underline"
                    >
                      {u}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          )
        ) : null}
        <FieldBlock
          icon={isSheet ? FileText : Hash}
          label={notesLabel}
          action={editButton("notes", notesLabel)}
          variant={variant}
          className={fullWidthClass}
          testId="commodity-detail-notes"
        >
          {quickEdit === "notes" ? (
            <NotesQuickEditor
              commodity={commodity}
              label={notesLabel}
              saving={saving}
              onSave={saveField("notes")}
              onCancel={cancelQuickEdit}
            />
          ) : commodity.comments ? (
            <p className={cn("text-sm whitespace-pre-wrap", isSheet && "font-normal")}>
              {commodity.comments}
            </p>
          ) : (
            noValue
          )}
        </FieldBlock>
        {commodity.extra_serial_numbers && commodity.extra_serial_numbers.length > 0 ? (
          isSheet ? (
            <DetailRow
              icon={Hash}
              label={t("commodities:detail.fields.extraSerialNumbers")}
              value={
                <div className="flex flex-wrap gap-1.5 mt-0.5">
                  {commodity.extra_serial_numbers.map((s) => (
                    <Badge key={s} variant="outline">
                      {s}
                    </Badge>
                  ))}
                </div>
              }
              variant={variant}
              withDivider
            />
          ) : (
            <div className={cn("flex flex-col gap-1.5", fullWidthClass)}>
              <DetailLabel icon={Hash} label={t("commodities:detail.fields.extraSerialNumbers")} />
              <div className="flex flex-wrap gap-1.5">
                {commodity.extra_serial_numbers.map((s) => (
                  <Badge key={s} variant="outline">
                    {s}
                  </Badge>
                ))}
              </div>
            </div>
          )
        ) : null}
        {commodity.part_numbers && commodity.part_numbers.length > 0 ? (
          isSheet ? (
            <DetailRow
              icon={Package}
              label={t("commodities:detail.fields.partNumbers")}
              value={
                <div className="flex flex-wrap gap-1.5 mt-0.5">
                  {commodity.part_numbers.map((p) => (
                    <Badge key={p} variant="outline">
                      {p}
                    </Badge>
                  ))}
                </div>
              }
              variant={variant}
              withDivider
            />
          ) : (
            <div className={cn("flex flex-col gap-1.5", fullWidthClass)}>
              <DetailLabel icon={Hash} label={t("commodities:detail.fields.partNumbers")} />
              <div className="flex flex-wrap gap-1.5">
                {commodity.part_numbers.map((p) => (
                  <Badge key={p} variant="outline">
                    {p}
                  </Badge>
                ))}
              </div>
            </div>
          )
        ) : null}
        {/* Custom fields (#2803): one row per field the item has a value
            for, in the group's order. The mock has no custom fields; they
            take the rows' own shape. */}
        {shownCustomFields.map((field) => (
          <DetailRow
            key={field.id}
            icon={ListPlus}
            label={field.name}
            value={
              <CustomFieldValue
                field={field}
                raw={customValues?.[field.id]}
                linkedItems={linkedItems}
                groupSlug={groupSlug}
              />
            }
            variant={variant}
            withDivider={isSheet}
            testId={`commodity-detail-custom-field-${field.id}`}
          />
        ))}
        {isSheet ? null : <Separator className={separatorClass} />}
        <DetailRow
          icon={Calendar}
          label={t("commodities:detail.fields.registeredDate")}
          value={
            commodity.registered_date
              ? formatDate(commodity.registered_date as string, { style: "short" })
              : noValue
          }
          variant={variant}
          withDivider={isSheet}
        />
        <DetailRow
          icon={Calendar}
          label={t("commodities:detail.fields.lastModifiedDate")}
          value={
            commodity.last_modified_date
              ? formatDate(commodity.last_modified_date as string, { style: "short" })
              : noValue
          }
          variant={variant}
          withDivider={isSheet}
        />
      </Inner>
    </Outer>
  )
}

interface DetailRowProps {
  icon: typeof MapPin
  label: string
  value: React.ReactNode
  // `"sheet"` switches to the mock's `[icon] [label / value]`
  // horizontal arrangement separated by Separator; `"page"` keeps
  // the existing label-stacked-above-value layout.
  variant?: "page" | "sheet"
  // Sheet rows draw a top Separator unless they're first in the
  // list; the parent passes `false` for the leading row.
  withDivider?: boolean
  // A control next to the label, such as the quick-edit pencil.
  action?: ReactNode
  // Spans both columns of the page grid (an open quick editor needs the room).
  wide?: boolean
  testId?: string
}

function DetailRow({
  icon: Icon,
  label,
  value,
  variant = "page",
  withDivider,
  action,
  wide,
  testId,
}: DetailRowProps) {
  if (variant === "sheet") {
    return (
      <>
        {withDivider ? <Separator /> : null}
        <div className="flex items-start gap-3 py-2.5" data-testid={testId}>
          <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <div className="flex-1 min-w-0">
            <div className="mb-0.5 flex min-h-4 items-center justify-between gap-2">
              <p className="text-xs text-muted-foreground">{label}</p>
              {action}
            </div>
            <div className="text-sm font-medium">{value}</div>
          </div>
        </div>
      </>
    )
  }
  return (
    <div className={cn("flex flex-col gap-1", wide && "sm:col-span-2")} data-testid={testId}>
      <div className="flex min-h-4 items-center gap-1">
        <DetailLabel icon={Icon} label={label} />
        {action}
      </div>
      <div className="text-sm">{value}</div>
    </div>
  )
}

// A full-width field of the Details tab (tags, notes): a DetailRow in the
// sheet, a labelled block across both columns on the page.
function FieldBlock({
  icon,
  label,
  action,
  variant,
  className,
  testId,
  children,
}: {
  icon: typeof MapPin
  label: string
  action?: ReactNode
  variant: "page" | "sheet"
  className?: string
  testId?: string
  children: ReactNode
}) {
  if (variant === "sheet") {
    return (
      <DetailRow
        icon={icon}
        label={label}
        action={action}
        value={children}
        variant={variant}
        withDivider
        testId={testId}
      />
    )
  }
  return (
    <div className={cn("flex flex-col gap-1.5", className)} data-testid={testId}>
      <div className="flex min-h-4 items-center gap-1">
        <DetailLabel icon={icon} label={label} />
        {action}
      </div>
      <div className="text-sm">{children}</div>
    </div>
  )
}

interface QuickEditorProps {
  commodity: Commodity
  saving: boolean
  onSave: (patch: Partial<Commodity>) => Promise<boolean>
  onCancel: () => void
}

function QuickEditActions({
  field,
  saving,
  dirty,
  onSave,
  onCancel,
}: {
  field: QuickEditField
  saving: boolean
  dirty: boolean
  onSave: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  return (
    <div className="flex gap-2">
      <Button
        type="button"
        size="sm"
        onClick={onSave}
        disabled={saving || !dirty}
        data-testid={`commodity-quick-${field}-save`}
      >
        {t("common:actions.save")}
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        onClick={onCancel}
        disabled={saving}
        data-testid={`commodity-quick-${field}-cancel`}
      >
        {t("common:actions.cancel")}
      </Button>
    </div>
  )
}

function LocationQuickEditor({
  commodity,
  areas,
  locations,
  saving,
  onSave,
  onCancel,
}: QuickEditorProps & { areas: AreaOption[]; locations: LocationOption[] }) {
  const initial = commodity.area_id ?? ""
  const [areaId, setAreaId] = useState(initial)
  return (
    <div className="flex flex-col gap-3 pt-1 font-normal" data-testid="commodity-quick-location">
      <LocationAreaFields
        areas={areas}
        locations={locations}
        areaId={areaId}
        onAreaChange={setAreaId}
        idPrefix="quick"
      />
      <QuickEditActions
        field="location"
        saving={saving}
        dirty={areaId !== initial}
        // An empty area leaves the item unassigned.
        onSave={() => void onSave({ area_id: areaId || undefined })}
        onCancel={onCancel}
      />
    </div>
  )
}

function TagsQuickEditor({ commodity, saving, onSave, onCancel }: QuickEditorProps) {
  const initial = commodity.tags ?? []
  const [tags, setTags] = useState(initial)
  const dirty = tags.length !== initial.length || tags.some((tag) => !initial.includes(tag))
  return (
    <div className="flex flex-col gap-3 pt-1 font-normal" data-testid="commodity-quick-tags">
      <TagPalette
        kind="commodity"
        values={tags}
        onChange={setTags}
        inputId="quick-tags-input"
        testId="commodity-quick-tags-palette"
      />
      <QuickEditActions
        field="tags"
        saving={saving}
        dirty={dirty}
        onSave={() => void onSave({ tags })}
        onCancel={onCancel}
      />
    </div>
  )
}

function NotesQuickEditor({
  commodity,
  label,
  saving,
  onSave,
  onCancel,
}: QuickEditorProps & { label: string }) {
  const initial = commodity.comments ?? ""
  const [notes, setNotes] = useState(initial)
  return (
    <div className="flex flex-col gap-3 pt-1 font-normal" data-testid="commodity-quick-notes">
      <Textarea
        rows={3}
        className="resize-none"
        value={notes}
        onChange={(e) => setNotes(e.target.value)}
        aria-label={label}
        data-testid="commodity-quick-notes-input"
      />
      <QuickEditActions
        field="notes"
        saving={saving}
        dirty={notes !== initial}
        onSave={() => void onSave({ comments: notes })}
        onCancel={onCancel}
      />
    </div>
  )
}

function DetailLabel({ icon: Icon, label }: { icon: typeof MapPin; label: string }) {
  return (
    <span className="flex items-center gap-1.5 text-xs uppercase tracking-wide text-muted-foreground">
      <Icon className="size-3.5" aria-hidden="true" />
      {label}
    </span>
  )
}

interface FilesTabProps {
  commodityId: string
  commodityName?: string
  onAttachClick: () => void
  coverState?: { current?: string; auto?: string }
  onSetCover?: (fileId: string | null) => void
  coverBusy?: boolean
}

function FilesTab({
  commodityId,
  commodityName,
  onAttachClick,
  coverState,
  onSetCover,
  coverBusy,
}: FilesTabProps) {
  // Commodity-detail Files tab — mock-parity surface introduced for
  // #1530 item 3: segmented chip-bar (All / Photos / Invoices /
  // Documents) + contextual upload zone + 3-col aspect-square photo
  // grid + non-photo list. Sits on top of the same unified `/files`
  // surface the global Files page uses (#1411 AC #4) but with a
  // commodity-specific layout. The cover-photo wiring (#1451 option
  // B) and the page-level upload dialog (#1448) are passed through.
  //
  // `EntityFilesPanel` stays as-is for `LocationDetailPage`; only
  // the commodity surface adopts the chip-bar treatment because the
  // mock contract is commodity-specific.
  return (
    <CommodityFilesTab
      commodityId={commodityId}
      commodityName={commodityName}
      onAttachClick={onAttachClick}
      coverState={coverState}
      onSetCover={onSetCover}
      coverBusy={coverBusy}
    />
  )
}

interface WarrantyTabProps {
  commodity?: Commodity
  // Optional. When provided, renders an "Upload Receipt" CTA that opens
  // the page-level UploadFilesDialog with this commodity preselected —
  // the same dialog the Files-tab dropzone opens (#2012) — instead of
  // inventing a per-commodity upload widget. The page wires this to
  // open the dialog and switch the Files tab underneath.
  onUploadReceipt?: () => void
}

// WarrantyTab renders the first-class warranty surface (#1367):
// status-coloured card + expiry-with-days-remaining line + notes block
// + an "Upload Receipt" CTA that opens the upload dialog (#2012).
// Layout mirrors the design mock
// (`inventario-design/src/components/ItemDetail.tsx`,
// `<TabsContent value="warranty">`); the design mock CLAUDE.md
// requires that warranty status colours go through the canonical
// WarrantyBadge / `WARRANTY_STATUS_CONFIG` so all four surfaces
// (badge, list rows, dashboard panel, this tab) read the same tokens.
//
// The form inputs themselves live in the commodity edit dialog's
// Warranty step — this tab stays read-only.
function WarrantyTab({ commodity, onUploadReceipt }: WarrantyTabProps) {
  const { t } = useTranslation()
  // #1554: bundle commodities don't carry a warranty — render the
  // "split into separate items" hint instead of the live pill / notes
  // block. Doing this here (rather than inside the existing layout)
  // avoids leaking the upload-receipt CTA + status pill into a row
  // that can never have either.
  const isBundle = (commodity?.count ?? 0) > 1
  if (isBundle) {
    return (
      <Card data-testid="commodity-detail-warranty">
        <CardHeader>
          <CardTitle>{t("commodities:detail.warranty.title")}</CardTitle>
          <CardDescription>{t("commodities:detail.warranty.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <p
            className="text-sm text-muted-foreground"
            data-testid="commodity-detail-warranty-bundle-empty-state"
          >
            {t("commodities:trackingRestrictions.warrantyDisabled")}
          </p>
        </CardContent>
      </Card>
    )
  }
  const status: CommodityWarrantyStatus = commodity
    ? warrantyStatus({ warranty_expires_at: commodity.warranty_expires_at })
    : "none"
  const daysRemaining = warrantyDaysRemaining(commodity?.warranty_expires_at)
  const visual = WARRANTY_STATUS_CONFIG[status]
  const StatusIcon = visual.icon
  return (
    <Card data-testid="commodity-detail-warranty">
      <CardHeader>
        <CardTitle>{t("commodities:detail.warranty.title")}</CardTitle>
        <CardDescription>{t("commodities:detail.warranty.description")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className={cn("flex flex-col gap-2 rounded-lg border p-4", visual.bg, visual.border)}>
          <div className="flex items-center gap-2">
            <StatusIcon className={cn("size-4", visual.text)} aria-hidden="true" />
            <WarrantyBadge
              status={status}
              showIcon={false}
              data-testid="commodity-detail-warranty-status"
            />
          </div>
          {commodity?.warranty_expires_at && daysRemaining !== null ? (
            <p className={cn("text-sm", visual.text)}>
              {daysRemaining > 0
                ? t("commodities:detail.warranty.expiresOnIn", {
                    date: formatDate(commodity.warranty_expires_at),
                    count: daysRemaining,
                  })
                : daysRemaining === 0
                  ? t("commodities:detail.warranty.expiresToday")
                  : t("commodities:detail.warranty.expiredOnAgo", {
                      date: formatDate(commodity.warranty_expires_at),
                      count: -daysRemaining,
                    })}
            </p>
          ) : status === "none" ? (
            <p className="text-sm text-muted-foreground">
              {t("commodities:detail.warranty.noneInline")}
            </p>
          ) : null}
        </div>
        {commodity?.warranty_notes ? (
          <div
            className="rounded-lg border border-border bg-muted/30 p-3"
            data-testid="commodity-detail-warranty-notes"
          >
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {t("commodities:detail.warranty.notesLabel")}
            </p>
            <p className="mt-1 text-sm text-foreground whitespace-pre-wrap">
              {commodity.warranty_notes}
            </p>
          </div>
        ) : null}
        {status === "none" ? (
          <p className="text-sm text-muted-foreground">
            {t("commodities:detail.warranty.emptyState")}
          </p>
        ) : null}
        {onUploadReceipt ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="w-fit gap-1.5"
            onClick={onUploadReceipt}
            data-testid="commodity-detail-warranty-upload-receipt"
          >
            <FileText className="size-3.5" aria-hidden="true" />
            {t("commodities:detail.warranty.uploadReceipt")}
          </Button>
        ) : null}
      </CardContent>
    </Card>
  )
}

// warrantyDaysRemaining returns days between today and the expiry
// date. Negative values mean the warranty already expired. Returns
// null when the date is missing or unparseable.
function warrantyDaysRemaining(date: string | undefined): number | null {
  if (!date) return null
  const t = Date.parse(`${date}T00:00:00Z`)
  if (Number.isNaN(t)) return null
  const today = new Date()
  const todayUTC = Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), today.getUTCDate())
  return Math.round((t - todayUTC) / (1000 * 60 * 60 * 24))
}

// CommodityDetailPage is the canonical full-page entry mounted at
// `/g/:slug/commodities/:id` (and `/edit`). Direct landings, refresh,
// "open in new tab", and shared links all hit this page — its URL is
// stable and the layout is the standard centered-column view.
//
// The actual rendering lives in `<CommodityDetailContent>` so the
// sheet variant can reuse the same hooks, dialogs, drag-drop, and
// tab logic without duplication.
export function CommodityDetailPage() {
  const { id = "" } = useParams<{ id: string }>()
  return <CommodityDetailContent id={id} variant="page" />
}

// CommodityDetailSheet renders the same content as the full page,
// but inside a right-side `<Sheet>` overlay. Used by the router when
// the navigation that landed us here carried `state.background` (i.e.
// the user drilled in from the items list). The list page stays
// mounted underneath so closing the sheet just dismisses the overlay
// without a route transition.
//
// Closing the sheet (X / Esc / outside click) navigates to the
// `state.background` URL — that drops the `:id` from the path and
// returns the user to the list they came from. If we can't resolve a
// background (e.g. the user reloaded with a stale state), we fall
// back to going back in history; if even that fails, the route's
// catch-all `<NotFoundPage>` would handle it after the page reloads.
export function CommodityDetailSheet() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const background = (
    location.state as { background?: { pathname: string; search?: string; hash?: string } } | null
  )?.background

  function handleClose() {
    if (background) {
      navigate(`${background.pathname}${background.search ?? ""}${background.hash ?? ""}`, {
        replace: true,
      })
    } else {
      navigate(-1)
    }
  }

  return (
    <Sheet open onOpenChange={(open) => !open && handleClose()}>
      <SheetContent
        side="right"
        // Width + flex shape lifted verbatim from the design mock
        // (`denisvmedia/inventario-design/src/components/ItemDetail.tsx`).
        // `flex flex-col gap-0` overrides the SheetContent default
        // `gap-4` so children layout flush against the header — each
        // section owns its own padding (header `pt-6 pb-4`, action
        // row `pb-4`, status card `mb-4`, tab body `mt-4`). `p-0`
        // lets the inner wrapper supply `px-5 pb-5`. Below `sm` the
        // Sheet primitive falls back to `w-3/4` so the panel takes
        // the whole viewport on narrow screens.
        className="w-full sm:max-w-lg flex flex-col gap-0 overflow-y-auto p-0"
        data-testid="commodity-detail-sheet"
      >
        {/* Radix Dialog (which Sheet wraps) emits a console warning
            unless a DialogTitle/SheetTitle is mounted inside
            DialogContent. The visible commodity name lives inside
            CommodityDetailContent's own header (`<h1
            data-testid="commodity-detail-name">`) which Radix can't
            see, so we mirror it sr-only here. The fallback copy
            covers the brief window before `useCommodity` resolves. */}
        <SheetHeader className="sr-only">
          <SheetTitle>{t("commodities:detail.sheetTitleFallback")}</SheetTitle>
          <SheetDescription>{t("commodities:detail.sheetDescriptionFallback")}</SheetDescription>
        </SheetHeader>
        <CommodityDetailContent id={id} variant="sheet" />
      </SheetContent>
    </Sheet>
  )
}
