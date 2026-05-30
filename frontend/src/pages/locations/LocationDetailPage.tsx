import { useEffect, useMemo, useState } from "react"
import { Link, useMatch, useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import {
  ChevronRight,
  MapPin,
  MoreHorizontal,
  Package,
  Pencil,
  Plus,
  Shield,
  Trash2,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Page } from "@/components/ui/page"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { DropOverlay } from "@/components/files/DropOverlay"
import { EntityFilesPanel } from "@/components/files/EntityFilesPanel"
import { UploadFilesDialog } from "@/components/files/UploadFilesDialog"
import { useFileDropZone } from "@/components/files/useFileDropZone"
import { LocationsBreadcrumb } from "@/components/locations/LocationsBreadcrumb"
import { LocationFormDialog } from "@/components/locations/LocationFormDialog"
import { AreaFormDialog } from "@/components/locations/AreaFormDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAreas, useCreateArea, useDeleteArea } from "@/features/areas/hooks"
import { useCommodities } from "@/features/commodities/hooks"
import { warrantyStatus } from "@/features/commodities/constants"
import {
  useDeleteLocation,
  useLocation,
  useLocations,
  useUpdateLocation,
} from "@/features/locations/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { cn } from "@/lib/utils"
import type { Area } from "@/features/areas/api"

interface LocationDetailPageProps {
  initialMode?: "edit"
}

// Cap the single page-level commodities fetch behind area tiles —
// matches LocationsListPage's cap so a navigate from list → detail
// reuses the cached query. Partial counts beyond the cap surface as
// "{N}+" on tiles.
const ITEM_COUNT_FETCH_CAP = 500

// /locations/:id — single-location detail. Renders the multi-segment
// breadcrumb, metadata + edit/delete header, the responsive area tile
// grid (per `design-mocks/src/views/LocationPickerView.tsx` Level 2)
// and the Location Files panel underneath. The /locations/:id/edit
// deep link mounts this same component with `initialMode="edit"`;
// both routes auto-open the edit dialog.
export function LocationDetailPage({ initialMode }: LocationDetailPageProps = {}) {
  const { t } = useTranslation()
  const params = useParams<{ id: string }>()
  const id = params.id ?? ""
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug

  const location = useLocation(id, { enabled: !!currentGroup })
  const allLocations = useLocations({ enabled: !!currentGroup })
  const allAreas = useAreas({ enabled: !!currentGroup })
  const itemsForCounts = useCommodities(
    { perPage: ITEM_COUNT_FETCH_CAP, includeInactive: false },
    { enabled: !!currentGroup }
  )
  const updateLocation = useUpdateLocation(id)
  const deleteLocation = useDeleteLocation()
  const createArea = useCreateArea()
  const deleteArea = useDeleteArea()

  const toast = useAppToast()
  const confirm = useConfirm()

  type DialogState = { kind: "none" } | { kind: "edit" } | { kind: "create-area" }
  const [dialog, setDialog] = useState<DialogState>(() =>
    initialMode === "edit" ? { kind: "edit" } : { kind: "none" }
  )

  // #1448 quick-attach: same pattern as the commodity detail page.
  const [uploadOpen, setUploadOpen] = useState(false)
  const [pendingDropFiles, setPendingDropFiles] = useState<File[]>([])
  const dropZone = useFileDropZone({
    onFiles: (files) => {
      setPendingDropFiles(files)
      setUploadOpen(true)
    },
    disabled: uploadOpen,
  })

  const editMatch = useMatch({ path: "/g/:groupSlug/locations/:id/edit", end: true })
  useEffect(() => {
    // Deep-link sync from URL → local dialog state; one extra render is fine.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (editMatch && dialog.kind === "none") setDialog({ kind: "edit" })
  }, [editMatch, dialog.kind])

  function closeDialog() {
    setDialog({ kind: "none" })
    if (editMatch && slug && id) {
      navigate(`/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(id)}`, {
        replace: true,
      })
    }
  }

  async function handleEditLocation(values: {
    name: string
    address: string
    icon: string
    description: string
  }) {
    await updateLocation.mutateAsync({
      name: values.name,
      address: values.address,
      icon: values.icon,
      description: values.description,
    })
    toast.success(t("locations:toast.locationUpdated"))
  }

  async function handleCreateArea(values: { name: string; location_id: string; icon: string }) {
    await createArea.mutateAsync(values)
    toast.success(t("locations:toast.areaCreated"))
  }

  async function handleDelete() {
    if (!location.data?.id) return
    const areaCount = (allAreas.data ?? []).filter((a) => a.location_id === id).length
    const ok = await confirm({
      title: t("locations:delete.locationTitle", { name: location.data.name ?? "" }),
      description:
        areaCount > 0
          ? t("locations:delete.locationDescriptionWithAreas", { count: areaCount })
          : t("locations:delete.locationDescription"),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (!ok) return
    try {
      await deleteLocation.mutateAsync(location.data.id)
      toast.success(t("locations:toast.locationDeleted"))
      if (slug) navigate(`/g/${encodeURIComponent(slug)}/locations`, { replace: true })
    } catch {
      toast.error(t("locations:toast.locationDeleteError"))
    }
  }

  async function handleDeleteArea(area: Area) {
    if (!area.id) return
    const ok = await confirm({
      title: t("locations:delete.areaTitle", { name: area.name ?? "" }),
      description: t("locations:delete.areaDescription"),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (!ok) return
    try {
      await deleteArea.mutateAsync(area.id)
      toast.success(t("locations:toast.areaDeleted"))
    } catch {
      toast.error(t("locations:toast.areaDeleteError"))
    }
  }

  // Per-area item + expiring-warranty counts derived from the page's
  // single commodities fetch. Empty while loading or on error; tiles
  // render the count chip as "—" in those windows. Match-by-truncation
  // only when data actually exists (an error leaves data undefined, so
  // total/rows are both 0 and would otherwise look "not truncated").
  const { itemCounts, expiringCounts, isTruncated } = useMemo(() => {
    const itemMap = new Map<string, number>()
    const expiringMap = new Map<string, number>()
    const rows = itemsForCounts.data?.commodities ?? []
    for (const c of rows) {
      if (!c.area_id) continue
      itemMap.set(c.area_id, (itemMap.get(c.area_id) ?? 0) + 1)
      if (warrantyStatus({ warranty_expires_at: c.warranty_expires_at }) === "expiring") {
        expiringMap.set(c.area_id, (expiringMap.get(c.area_id) ?? 0) + 1)
      }
    }
    return {
      itemCounts: itemMap,
      expiringCounts: expiringMap,
      isTruncated: !!itemsForCounts.data && (itemsForCounts.data.total ?? 0) > rows.length,
    }
  }, [itemsForCounts.data])
  // Network/API failure → counts are unknown, not zero. Treat `isError`
  // as "still loading" so AreaTile keeps the "—" affordance instead of
  // implying empty.
  const itemsCountIsUnknown = itemsForCounts.isLoading || itemsForCounts.isError

  if (location.isError) {
    return (
      <Page width="wide">
        <RouteTitle title={t("locations:detail.errorTitle")} />
        <Alert variant="destructive" data-testid="location-detail-error">
          <AlertTitle>{t("locations:detail.errorTitle")}</AlertTitle>
          <AlertDescription>{t("locations:detail.errorDescription")}</AlertDescription>
        </Alert>
      </Page>
    )
  }

  const myAreas = (allAreas.data ?? []).filter((a) => a.location_id === id)
  const locationsHref = slug ? `/g/${encodeURIComponent(slug)}/locations` : "#"

  return (
    <>
      <RouteTitle title={location.data?.name ?? t("locations:detail.fallbackTitle")} />
      <Page
        width="wide"
        className="relative"
        data-testid="page-location-detail"
        {...dropZone.bindProps}
      >
        {dropZone.isDragging ? (
          <DropOverlay
            label={t("files:entityPanel.dropOverlay_location")}
            hint={t("files:entityPanel.dropHint")}
          />
        ) : null}
        <LocationsBreadcrumb
          backHref={locationsHref}
          backLabel={t("locations:detail.back")}
          navLabel={t("locations:breadcrumb.navLabel")}
          segments={[
            {
              label: t("locations:breadcrumb.locations"),
              to: locationsHref,
              testId: "breadcrumb-locations",
            },
            {
              label: location.data?.name ?? t("locations:detail.fallbackTitle"),
              testId: "breadcrumb-current",
            },
          ]}
          testId="location-detail-breadcrumb"
        />

        {location.isLoading ? (
          <div className="space-y-3" data-testid="location-detail-loading">
            <Skeleton className="h-8 w-64" />
            <Skeleton className="h-4 w-96" />
          </div>
        ) : location.data ? (
          <>
            <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <h1 className="flex items-center gap-2 text-2xl font-semibold tracking-tight">
                  {location.data.icon ? (
                    <span
                      className="text-2xl leading-none"
                      aria-hidden="true"
                      data-testid="location-detail-icon"
                    >
                      {location.data.icon}
                    </span>
                  ) : (
                    <MapPin className="size-5 text-muted-foreground" aria-hidden="true" />
                  )}
                  <span className="truncate">{location.data.name}</span>
                </h1>
                {location.data.description ? (
                  <p
                    className="mt-1 text-muted-foreground"
                    data-testid="location-detail-description"
                  >
                    {location.data.description}
                  </p>
                ) : null}
                {location.data.address ? (
                  <p className="mt-1 text-sm text-muted-foreground/80">{location.data.address}</p>
                ) : null}
              </div>
              <div className="flex items-center gap-2 shrink-0">
                {slug && location.data.id ? (
                  <Button
                    asChild
                    variant="outline"
                    className="gap-2"
                    data-testid="location-detail-insurance"
                  >
                    <Link
                      to={`/g/${encodeURIComponent(slug)}/reports/insurance?mode=location&location=${encodeURIComponent(location.data.id)}`}
                    >
                      <Shield className="size-4" aria-hidden="true" />
                      {t("locations:detail.insuranceReport")}
                    </Link>
                  </Button>
                ) : null}
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setDialog({ kind: "edit" })}
                  data-testid="location-detail-edit"
                  className="gap-2"
                >
                  <Pencil className="size-4" aria-hidden="true" />
                  {t("locations:detail.edit")}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleDelete}
                  data-testid="location-detail-delete"
                  className="gap-2"
                >
                  <Trash2 className="size-4 text-destructive" aria-hidden="true" />
                  {t("common:actions.delete")}
                </Button>
              </div>
            </header>

            <section className="flex flex-col gap-3" data-testid="location-detail-areas-section">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold">{t("locations:detail.areasTitle")}</h2>
                  <p className="text-sm text-muted-foreground">
                    {t("locations:detail.areasDescription", { count: myAreas.length })}
                  </p>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setDialog({ kind: "create-area" })}
                  data-testid="location-detail-add-area"
                  className="gap-2"
                >
                  <Plus className="size-3.5" aria-hidden="true" />
                  {t("locations:list.addArea")}
                </Button>
              </div>

              {myAreas.length === 0 ? (
                <Card data-testid="location-detail-areas-empty">
                  <CardHeader>
                    <CardTitle className="text-base">
                      {t("locations:detail.areasEmptyTitle")}
                    </CardTitle>
                    <CardDescription>{t("locations:detail.areasEmpty")}</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setDialog({ kind: "create-area" })}
                      className="gap-2"
                    >
                      <Plus className="size-3.5" aria-hidden="true" />
                      {t("locations:list.addArea")}
                    </Button>
                  </CardContent>
                </Card>
              ) : (
                <ul className="grid gap-3 sm:grid-cols-2" data-testid="location-detail-areas-grid">
                  {myAreas.map((area) => {
                    const count = area.id ? (itemCounts.get(area.id) ?? 0) : 0
                    const expiring = area.id ? (expiringCounts.get(area.id) ?? 0) : 0
                    return (
                      <li key={area.id}>
                        <AreaTile
                          area={area}
                          itemCount={count}
                          expiringCount={expiring}
                          itemCountLoading={itemsCountIsUnknown}
                          itemCountTruncated={isTruncated}
                          onDelete={() => handleDeleteArea(area)}
                        />
                      </li>
                    )
                  })}
                </ul>
              )}
            </section>

            <EntityFilesPanel
              linkedEntityType="location"
              linkedEntityId={id}
              onAttachClick={() => {
                setPendingDropFiles([])
                setUploadOpen(true)
              }}
            />
          </>
        ) : null}
      </Page>

      <LocationFormDialog
        open={dialog.kind === "edit"}
        onOpenChange={(open) => (open ? null : closeDialog())}
        location={location.data}
        onSubmit={handleEditLocation}
        isPending={updateLocation.isPending}
      />
      <AreaFormDialog
        open={dialog.kind === "create-area"}
        onOpenChange={(open) => (open ? null : setDialog({ kind: "none" }))}
        locations={allLocations.data ?? []}
        defaultLocationId={id}
        onSubmit={handleCreateArea}
        isPending={createArea.isPending}
      />

      <UploadFilesDialog
        open={uploadOpen}
        onOpenChange={(open) => {
          setUploadOpen(open)
          if (!open) setPendingDropFiles([])
        }}
        linkedEntity={{
          type: "location",
          id,
          name: location.data?.name,
        }}
        initialFiles={pendingDropFiles}
      />
    </>
  )
}

interface AreaTileProps {
  area: Area
  itemCount: number
  expiringCount: number
  itemCountLoading: boolean
  itemCountTruncated: boolean
  onDelete: () => void
}

// AreaTile is the Level-2 mock card per
// `design-mocks/src/views/LocationPickerView.tsx` L615-L668: rounded
// border, icon avatar, name, "{n} item(s)", optional expiring pill,
// dropdown menu (reveal on hover), chevron. The avatar shows
// `area.icon` (emoji) when set, otherwise falls back to the generic
// Package glyph.
function AreaTile({
  area,
  itemCount,
  expiringCount,
  itemCountLoading,
  itemCountTruncated,
  onDelete,
}: AreaTileProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug
  const detailHref =
    slug && area.id ? `/g/${encodeURIComponent(slug)}/areas/${encodeURIComponent(area.id)}` : "#"
  const editHref =
    slug && area.id
      ? `/g/${encodeURIComponent(slug)}/areas/${encodeURIComponent(area.id)}/edit`
      : "#"
  const interactive = detailHref !== "#"
  // Truncation states mirror LocationsListPage's chip: the page-level
  // commodities fetch is capped, so an area whose items happen to live
  // past the cap can sample to 0 even when the real count isn't. Show
  // "—" instead of "0" in that case so the tile doesn't imply emptiness.
  // - loading                              → "—"
  // - truncated AND ≥1 in the sample       → "{n}+" (at-least)
  // - truncated AND 0 in the sample        → "—" (true count unknown)
  // - not truncated                        → exact count
  const itemCountLabel = itemCountLoading
    ? "—"
    : itemCountTruncated
      ? itemCount >= 1
        ? `${itemCount}+`
        : "—"
      : String(itemCount)
  return (
    <div
      className={cn(
        "group relative flex items-start gap-3 rounded-xl border border-border bg-card p-4 transition-all",
        interactive && "hover:-translate-y-0.5 hover:border-primary/20 hover:shadow-sm"
      )}
      data-testid="location-detail-area"
      data-area-id={area.id}
    >
      {interactive ? (
        <Link
          to={detailHref}
          aria-label={area.name ?? ""}
          className="absolute inset-0 rounded-xl focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring/50"
          data-testid="location-detail-area-link"
        />
      ) : null}
      {/* Inert decorative + text columns — `pointer-events-none` lets
          the overlay <Link> above receive clicks anywhere on the tile.
          The actions column re-enables pointer events on its
          interactive children only. */}
      <div
        className="pointer-events-none flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted text-xl text-muted-foreground"
        data-testid="location-detail-area-icon"
      >
        {area.icon ? (
          <span aria-hidden="true">{area.icon}</span>
        ) : (
          <Package className="size-5" aria-hidden="true" />
        )}
      </div>
      <div className="pointer-events-none flex min-w-0 flex-1 flex-col gap-0.5">
        <p className="truncate text-sm font-semibold">{area.name}</p>
        <p className="text-xs text-muted-foreground">
          {t("locations:detail.areaItems", { count: itemCount, formatted: itemCountLabel })}
        </p>
        {expiringCount > 0 ? (
          <Badge
            variant="secondary"
            className="mt-1 h-5 self-start border-0 bg-status-expiring/10 px-1.5 text-[10px] text-status-expiring"
            data-testid="location-detail-area-expiring"
          >
            {t("locations:detail.areaExpiring", { count: expiringCount })}
          </Badge>
        ) : null}
      </div>
      <div className="pointer-events-none flex shrink-0 items-center gap-1">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="pointer-events-auto size-7 opacity-0 transition-opacity focus-visible:opacity-100 group-hover:opacity-100 data-[state=open]:opacity-100"
              aria-label={t("locations:list.actionsLabel", { name: area.name ?? "" })}
              data-testid="location-detail-area-menu"
            >
              <MoreHorizontal className="size-3.5" aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem asChild data-testid="location-detail-area-edit">
              <Link to={editHref}>
                <Pencil className="mr-2 size-4" aria-hidden="true" />
                {t("locations:detail.edit")}
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={onDelete}
              className="text-destructive focus:text-destructive"
              data-testid="location-detail-area-delete"
            >
              <Trash2 className="mr-2 size-4" aria-hidden="true" />
              {t("common:actions.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <ChevronRight
          className="size-3.5 text-muted-foreground transition-colors group-hover:text-foreground"
          aria-hidden="true"
        />
      </div>
    </div>
  )
}
