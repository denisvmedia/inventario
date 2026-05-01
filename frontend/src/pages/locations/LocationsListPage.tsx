import { useEffect, useMemo, useState } from "react"
import { Link, useMatch, useNavigate } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { ChevronRight, MapPin, Plus, Search, Trash2 } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { LocationFormDialog } from "@/components/locations/LocationFormDialog"
import { AreaFormDialog } from "@/components/locations/AreaFormDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAreas, useCreateArea, useDeleteArea } from "@/features/areas/hooks"
import { useCreateLocation, useDeleteLocation, useLocations } from "@/features/locations/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import type { Area } from "@/features/areas/api"
import type { Location } from "@/features/locations/api"

interface LocationsListPageProps {
  // When mounted at /locations/new the dialog opens on first paint.
  initialMode?: "create"
}

// /locations — list of every location in the active group, with the
// nested areas surfaced inline. Add buttons open the matching dialog
// (`LocationFormDialog` / `AreaFormDialog`); both dialogs are also
// reachable via /locations/new — that route mounts this same page
// with `initialMode="create"` so a deep link can open the create
// modal on top of the list.
export function LocationsListPage({ initialMode }: LocationsListPageProps = {}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const enabled = !!currentGroup
  const slug = currentGroup?.slug

  const locations = useLocations({ enabled })
  const areas = useAreas({ enabled })
  const createLocation = useCreateLocation()
  const deleteLocation = useDeleteLocation()
  const createArea = useCreateArea()
  const deleteArea = useDeleteArea()

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
      setDialog({ kind: "create-location" })
    }
  }, [newMatch, dialog.kind])

  function closeDialog() {
    setDialog({ kind: "none" })
    if (newMatch && slug) {
      navigate(`/g/${encodeURIComponent(slug)}/locations`, { replace: true })
    }
  }

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

  async function handleCreateLocation(values: { name: string; address: string }) {
    await createLocation.mutateAsync({ name: values.name, address: values.address })
    toast.success(t("locations:toast.locationCreated"))
  }

  async function handleCreateArea(values: { name: string; location_id: string }) {
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
              return (
                <li key={loc.id}>
                  <LocationCard
                    location={loc}
                    areas={locAreas}
                    onAddArea={() => setDialog({ kind: "create-area", locationId: loc.id })}
                    onDeleteLocation={() => handleDeleteLocation(loc)}
                    onDeleteArea={(a) => handleDeleteArea(a)}
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
  areas: Area[]
  onAddArea: () => void
  onDeleteLocation: () => void
  onDeleteArea: (area: Area) => void
}

// LocationCard owns one location + its inline area list. Extracted as
// a separate component so the list page stays readable; nothing here
// needs hooks on its own (parent owns state + mutations).
function LocationCard({
  location,
  areas,
  onAddArea,
  onDeleteLocation,
  onDeleteArea,
}: LocationCardProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug
  const detailHref =
    slug && location.id
      ? `/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(location.id)}`
      : "#"

  return (
    <Card data-testid="location-card" data-location-id={location.id}>
      <CardHeader>
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <CardTitle className="flex items-center gap-2 text-base">
              <MapPin className="size-4 text-muted-foreground" aria-hidden="true" />
              <Link to={detailHref} className="hover:underline truncate">
                {location.name}
              </Link>
            </CardTitle>
            {location.address ? (
              <CardDescription className="truncate">{location.address}</CardDescription>
            ) : null}
          </div>
          <div className="flex items-center gap-1.5 shrink-0">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={onAddArea}
              data-testid="location-card-add-area"
              className="gap-1.5"
            >
              <Plus className="size-3.5" aria-hidden="true" />
              {t("locations:list.addArea")}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={onDeleteLocation}
              data-testid="location-card-delete"
              aria-label={t("locations:list.deleteLocation")}
            >
              <Trash2 className="size-4 text-destructive" aria-hidden="true" />
            </Button>
          </div>
        </div>
      </CardHeader>
      {areas.length > 0 ? (
        <CardContent className="pt-0">
          <ul className="divide-y divide-border rounded-md border border-border">
            {areas.map((area) => {
              const areaHref =
                slug && area.id
                  ? `/g/${encodeURIComponent(slug)}/areas/${encodeURIComponent(area.id)}`
                  : "#"
              return (
                <li
                  key={area.id}
                  className="flex items-center justify-between px-3 py-2"
                  data-testid="location-card-area"
                >
                  <Link
                    to={areaHref}
                    className="flex items-center gap-2 text-sm hover:underline min-w-0"
                  >
                    <ChevronRight className="size-3.5 text-muted-foreground" aria-hidden="true" />
                    <span className="truncate">{area.name}</span>
                  </Link>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => onDeleteArea(area)}
                    aria-label={t("locations:list.deleteArea", { name: area.name ?? "" })}
                  >
                    <Trash2 className="size-3.5 text-destructive" aria-hidden="true" />
                  </Button>
                </li>
              )
            })}
          </ul>
        </CardContent>
      ) : null}
    </Card>
  )
}
