import { useEffect, useMemo, useState } from "react"
import { Link, useMatch, useNavigate } from "react-router-dom"
import { useTranslation } from "react-i18next"
import {
  ChevronRight,
  Layers,
  MapPin,
  MoreHorizontal,
  Package,
  Plus,
  Search,
  Trash2,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { LocationFormDialog } from "@/components/locations/LocationFormDialog"
import { AreaFormDialog } from "@/components/locations/AreaFormDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAreas, useCreateArea } from "@/features/areas/hooks"
import { useCommodities } from "@/features/commodities/hooks"
import { useCreateLocation, useDeleteLocation, useLocations } from "@/features/locations/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { cn } from "@/lib/utils"
import type { Location } from "@/features/locations/api"

interface LocationsListPageProps {
  // When mounted at /locations/new the dialog opens on first paint.
  initialMode?: "create"
}

// Cap the single page-level commodities fetch used to derive per-location
// item-count chips. Large enough to cover typical groups; partial counts
// past this are surfaced as "{N}+" so the chip stays useful instead of
// silently undercounting.
const ITEM_COUNT_FETCH_CAP = 500

// /locations — list of every location in the active group. Each card is
// a click-through tile (MapPin avatar + name + address + areas/items
// stat chips + chevron) routed to the location detail. Per
// `design-mocks/src/views/LocationPickerView.tsx` Level 1; the previous
// inline area list was dropped because the mock surfaces areas only on
// the detail page. Add buttons open the matching dialog
// (`LocationFormDialog` / `AreaFormDialog`); /locations/new mounts this
// same page with `initialMode="create"` so a deep link can open the
// create modal on top of the list.
export function LocationsListPage({ initialMode }: LocationsListPageProps = {}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const enabled = !!currentGroup
  const slug = currentGroup?.slug

  const locations = useLocations({ enabled })
  const areas = useAreas({ enabled })
  // Active-only, page-level fetch used purely for per-area count
  // aggregation behind the stat chips. Capped at ITEM_COUNT_FETCH_CAP —
  // a partial sample on bigger groups is still useful (chip becomes
  // "{N}+") and the cap avoids paying full inventory-scan cost on a
  // route that historically didn't read commodities at all.
  const itemsForCounts = useCommodities(
    { perPage: ITEM_COUNT_FETCH_CAP, includeInactive: false },
    { enabled }
  )
  const createLocation = useCreateLocation()
  const deleteLocation = useDeleteLocation()
  const createArea = useCreateArea()

  const toast = useAppToast()
  const confirm = useConfirm()

  // Search filters locations + their areas client-side. The list is
  // typically small (single-digit) so paging / server-side search
  // would be over-engineered.
  const [query, setQuery] = useState("")

  // Dialog state. We track both kinds in one state slot so the page
  // never tries to mount two modals at once.
  type DialogState =
    | { kind: "none" }
    | { kind: "create-location" }
    | { kind: "create-area"; locationId?: string }
  const [dialog, setDialog] = useState<DialogState>(() =>
    initialMode === "create" ? { kind: "create-location" } : { kind: "none" }
  )

  // Deep link: /locations/new auto-opens the create dialog. Closing
  // the dialog navigates back to /locations so the URL doesn't keep
  // re-opening the dialog on a re-render.
  const newMatch = useMatch({ path: "/g/:groupSlug/locations/new", end: true })
  useEffect(() => {
    if (newMatch && dialog.kind === "none") {
      // Deep-link sync from URL → local dialog state. The cascading
      // render is intentional and bounded (one extra render per nav).
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setDialog({ kind: "create-location" })
    }
  }, [newMatch, dialog.kind])

  function closeDialog() {
    setDialog({ kind: "none" })
    if (newMatch && slug) {
      navigate(`/g/${encodeURIComponent(slug)}/locations`, { replace: true })
    }
  }

  // Per-area item counts derived once from the page-level commodities
  // fetch. The map is empty while loading or on error; LocationCard
  // handles both branches by rendering "—" instead of the count digit.
  const areaItemCounts = useMemo(() => {
    const map = new Map<string, number>()
    for (const c of itemsForCounts.data?.commodities ?? []) {
      if (c.area_id) map.set(c.area_id, (map.get(c.area_id) ?? 0) + 1)
    }
    return map
  }, [itemsForCounts.data])
  // Network/API failure → counts are unknown, not zero. Funnel `isError`
  // into the same "loading" branch the chip already renders as "—" so
  // the UI doesn't claim an exact 0 when the request bombed out. Match
  // by truncation only when the data actually exists.
  const itemsCountIsUnknown = itemsForCounts.isLoading || itemsForCounts.isError
  const itemsCountTruncated =
    !!itemsForCounts.data &&
    (itemsForCounts.data.total ?? 0) > (itemsForCounts.data.commodities.length ?? 0)

  const filteredLocations = useMemo(() => {
    const list = locations.data ?? []
    if (!query.trim()) return list
    const q = query.trim().toLowerCase()
    const areaList = areas.data ?? []
    return list.filter((l) => {
      if ((l.name ?? "").toLowerCase().includes(q)) return true
      if ((l.address ?? "").toLowerCase().includes(q)) return true
      // A location matches if any of its areas match the query — keeps
      // search useful when the user remembers the room name but not
      // the building it's in.
      return areaList.some(
        (a) => a.location_id === l.id && (a.name ?? "").toLowerCase().includes(q)
      )
    })
  }, [locations.data, areas.data, query])

  async function handleCreateLocation(values: {
    name: string
    address: string
    icon: string
    description: string
  }) {
    await createLocation.mutateAsync({
      name: values.name,
      address: values.address,
      icon: values.icon,
      description: values.description,
    })
    toast.success(t("locations:toast.locationCreated"))
  }

  async function handleCreateArea(values: { name: string; location_id: string; icon: string }) {
    await createArea.mutateAsync(values)
    toast.success(t("locations:toast.areaCreated"))
  }

  async function handleDeleteLocation(loc: Location) {
    if (!loc.id) return
    const areaCount = (areas.data ?? []).filter((a) => a.location_id === loc.id).length
    const ok = await confirm({
      title: t("locations:delete.locationTitle", { name: loc.name ?? "" }),
      // Surface the orphan count so the user can't blow away a populated
      // location by accident. Areas belonging to the location go with
      // it server-side; the legacy frontend renders this same warning.
      description:
        areaCount > 0
          ? t("locations:delete.locationDescriptionWithAreas", { count: areaCount })
          : t("locations:delete.locationDescription"),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (!ok) return
    try {
      await deleteLocation.mutateAsync(loc.id)
      toast.success(t("locations:toast.locationDeleted"))
    } catch {
      toast.error(t("locations:toast.locationDeleteError"))
    }
  }

  const isLoading = locations.isLoading || areas.isLoading
  const isError = locations.isError || areas.isError
  const isEmpty = !isLoading && !isError && (locations.data ?? []).length === 0

  return (
    <>
      <RouteTitle title={t("locations:list.documentTitle")} />
      <div
        className="flex flex-col gap-6 p-6 max-w-5xl mx-auto w-full"
        data-testid="page-locations"
      >
        <header className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
              {t("locations:list.heading")}
            </h1>
            <p className="mt-1 text-muted-foreground leading-7">{t("locations:list.subtitle")}</p>
          </div>
          <Button
            type="button"
            onClick={() => setDialog({ kind: "create-location" })}
            data-testid="locations-add-button"
            className="gap-2"
          >
            <Plus className="size-4" aria-hidden="true" />
            {t("locations:list.addLocation")}
          </Button>
        </header>

        {!isEmpty && !isError ? (
          <div className="relative">
            <Search
              className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              type="search"
              placeholder={t("locations:list.searchPlaceholder")}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="pl-9"
              data-testid="locations-search"
            />
          </div>
        ) : null}

        {isError ? (
          <Alert variant="destructive" data-testid="locations-error">
            <AlertTitle>{t("locations:list.errorTitle")}</AlertTitle>
            <AlertDescription>{t("locations:list.errorDescription")}</AlertDescription>
          </Alert>
        ) : isLoading ? (
          <div className="space-y-3" data-testid="locations-loading">
            {Array.from({ length: 2 }).map((_, i) => (
              <Card key={i}>
                <CardHeader>
                  <Skeleton className="h-5 w-40" />
                  <Skeleton className="h-3 w-64" />
                </CardHeader>
              </Card>
            ))}
          </div>
        ) : isEmpty ? (
          <Card data-testid="locations-empty">
            <CardHeader>
              <CardTitle>{t("locations:list.emptyTitle")}</CardTitle>
              <CardDescription>{t("locations:list.emptyDescription")}</CardDescription>
            </CardHeader>
            <CardContent>
              <Button
                type="button"
                onClick={() => setDialog({ kind: "create-location" })}
                data-testid="locations-empty-cta"
                className="gap-2"
              >
                <Plus className="size-4" aria-hidden="true" />
                {t("locations:list.addLocation")}
              </Button>
            </CardContent>
          </Card>
        ) : (
          <ul className="space-y-3">
            {filteredLocations.map((loc) => {
              const locAreas = (areas.data ?? []).filter((a) => a.location_id === loc.id)
              const itemCount = locAreas.reduce(
                (sum, a) => sum + (a.id ? (areaItemCounts.get(a.id) ?? 0) : 0),
                0
              )
              return (
                <li key={loc.id}>
                  <LocationCard
                    location={loc}
                    areaCount={locAreas.length}
                    itemCount={itemCount}
                    itemCountLoading={itemsCountIsUnknown}
                    itemCountTruncated={itemsCountTruncated}
                    onAddArea={() => setDialog({ kind: "create-area", locationId: loc.id })}
                    onDeleteLocation={() => handleDeleteLocation(loc)}
                  />
                </li>
              )
            })}
            {filteredLocations.length === 0 ? (
              <li className="text-sm text-muted-foreground" data-testid="locations-search-empty">
                {t("locations:list.searchEmpty")}
              </li>
            ) : null}
          </ul>
        )}
      </div>

      <LocationFormDialog
        open={dialog.kind === "create-location"}
        onOpenChange={(open) => (open ? null : closeDialog())}
        onSubmit={handleCreateLocation}
        isPending={createLocation.isPending}
      />
      <AreaFormDialog
        open={dialog.kind === "create-area"}
        onOpenChange={(open) => (open ? null : setDialog({ kind: "none" }))}
        locations={locations.data ?? []}
        defaultLocationId={dialog.kind === "create-area" ? dialog.locationId : undefined}
        onSubmit={handleCreateArea}
        isPending={createArea.isPending}
      />
    </>
  )
}

interface LocationCardProps {
  location: Location
  areaCount: number
  itemCount: number
  itemCountLoading: boolean
  itemCountTruncated: boolean
  onAddArea: () => void
  onDeleteLocation: () => void
}

// LocationCard renders one location as a click-through tile per the
// Level 1 mock (`design-mocks/src/views/LocationPickerView.tsx`
// L546-L600). The icon avatar shows `location.icon` (emoji) when set,
// otherwise falls back to the generic MapPin glyph. The subtitle
// prefers `location.description` (the mock's muted one-liner) and
// falls back to `location.address` for rows created before the
// description field existed.
function LocationCard({
  location,
  areaCount,
  itemCount,
  itemCountLoading,
  itemCountTruncated,
  onAddArea,
  onDeleteLocation,
}: LocationCardProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug
  const detailHref =
    slug && location.id
      ? `/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(location.id)}`
      : "#"
  // Tile is link-only when the slug + id route exists; otherwise render
  // as a plain card to keep the page usable while the GroupContext
  // resolves (the hooks gate on !!currentGroup, so this is rare but
  // possible during first paint).
  const interactive = detailHref !== "#"
  // Truncation states: we sampled a capped page of commodities, so any
  // location whose items happen to live past the cap can sample to 0
  // even when the real count is non-zero. Surface that ambiguity
  // explicitly instead of rendering "0" (which would imply emptiness).
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
        "group relative flex items-center gap-4 rounded-2xl border border-border bg-card p-5 transition-all",
        interactive && "hover:-translate-y-0.5 hover:border-primary/20 hover:shadow-sm"
      )}
      data-testid="location-card"
      data-location-id={location.id}
    >
      {interactive ? (
        // Card-wide click target. Positioned-absolute fill so the
        // dropdown trigger + items below it can sit on top with their
        // own pointer events. aria-label gives screen readers the same
        // affordance as the visible title without duplicating it.
        <Link
          to={detailHref}
          aria-label={location.name ?? ""}
          className="absolute inset-0 rounded-2xl focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring/50"
          data-testid="location-card-link"
        />
      ) : null}
      {/* Inert decorative + text columns — `pointer-events-none` lets
          the overlay <Link> above receive clicks anywhere on the card.
          The actions column re-enables pointer events on its
          interactive children (dropdown trigger) only. */}
      <div
        className="pointer-events-none flex size-14 shrink-0 items-center justify-center rounded-xl bg-muted text-3xl text-muted-foreground"
        data-testid="location-card-icon"
      >
        {location.icon ? (
          <span aria-hidden="true">{location.icon}</span>
        ) : (
          <MapPin className="size-6" aria-hidden="true" />
        )}
      </div>
      <div className="pointer-events-none flex min-w-0 flex-1 flex-col gap-1">
        <p className="truncate text-base font-semibold">{location.name}</p>
        {location.description ? (
          <p
            className="truncate text-sm text-muted-foreground"
            data-testid="location-card-description"
          >
            {location.description}
          </p>
        ) : location.address ? (
          <p className="truncate text-sm text-muted-foreground">{location.address}</p>
        ) : null}
        <div className="mt-1 flex flex-wrap items-center gap-3">
          <span
            className="inline-flex items-center gap-1 text-xs text-muted-foreground"
            data-testid="location-card-stat-areas"
          >
            <Layers className="size-3.5" aria-hidden="true" />
            {t("locations:list.statsAreas", { count: areaCount })}
          </span>
          <span
            className="inline-flex items-center gap-1 text-xs text-muted-foreground"
            data-testid="location-card-stat-items"
          >
            <Package className="size-3.5" aria-hidden="true" />
            {t("locations:list.statsItems", { count: itemCount, formatted: itemCountLabel })}
          </span>
        </div>
      </div>
      {/* `relative z-10` is required even though the absolute overlay
          `<Link>` sits earlier in the DOM. CSS painting order puts
          positioned descendants (the Link, at z-auto = 0) above
          in-flow non-positioned descendants (this actions cluster),
          so without a stacking-context bump the trigger Button
          loses every click to the Link even though it has
          `pointer-events-auto`. Issue #1654. */}
      <div className="pointer-events-none relative z-10 flex shrink-0 items-center gap-1">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              // `cursor-pointer` is intentional: Tailwind v4 preflight
              // strips `cursor: pointer` from native <button>; same
              // workaround pattern as AppSidebar.tsx (sidebar Add-item).
              className="pointer-events-auto size-8 cursor-pointer opacity-0 transition-opacity focus-visible:opacity-100 group-hover:opacity-100 data-[state=open]:opacity-100"
              aria-label={t("locations:list.actionsLabel", { name: location.name ?? "" })}
              data-testid="location-card-menu"
            >
              <MoreHorizontal className="size-4" aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={onAddArea} data-testid="location-card-add-area">
              <Plus className="mr-2 size-4" aria-hidden="true" />
              {t("locations:list.addArea")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={onDeleteLocation}
              className="text-destructive focus:text-destructive"
              data-testid="location-card-delete"
            >
              <Trash2 className="mr-2 size-4" aria-hidden="true" />
              {t("common:actions.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <ChevronRight
          className="size-4 text-muted-foreground transition-colors group-hover:text-foreground"
          aria-hidden="true"
        />
      </div>
    </div>
  )
}
