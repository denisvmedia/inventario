import { useEffect, useState } from "react"
import { Link, useMatch, useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { ArrowLeft, ChevronRight, MapPin, Pencil, Plus, Trash2 } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { EntityFilesPanel } from "@/components/files/EntityFilesPanel"
import { LocationFormDialog } from "@/components/locations/LocationFormDialog"
import { AreaFormDialog } from "@/components/locations/AreaFormDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAreas, useCreateArea, useDeleteArea } from "@/features/areas/hooks"
import {
  useDeleteLocation,
  useLocation,
  useLocations,
  useUpdateLocation,
} from "@/features/locations/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import type { Area } from "@/features/areas/api"

interface LocationDetailPageProps {
  initialMode?: "edit"
}

// /locations/:id — single-location detail. Renders metadata + the
// location's areas, with edit / add-area / delete actions. The
// /locations/:id/edit deep link mounts this same component with
// `initialMode="edit"`; both routes auto-open the matching dialog.
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

  const editMatch = useMatch({ path: "/g/:groupSlug/locations/:id/edit", end: true })
  useEffect(() => {
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

  async function handleEditLocation(values: { name: string; address: string }) {
    await updateLocation.mutateAsync({ name: values.name, address: values.address })
    toast.success(t("locations:toast.locationUpdated"))
  }

  async function handleCreateArea(values: { name: string; location_id: string }) {
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

  if (location.isError) {
    return (
      <div className="flex flex-col gap-6 p-6 max-w-3xl mx-auto w-full">
        <RouteTitle title={t("locations:detail.errorTitle")} />
        <Alert variant="destructive" data-testid="location-detail-error">
          <AlertTitle>{t("locations:detail.errorTitle")}</AlertTitle>
          <AlertDescription>{t("locations:detail.errorDescription")}</AlertDescription>
        </Alert>
      </div>
    )
  }

  const myAreas = (allAreas.data ?? []).filter((a) => a.location_id === id)
  const backHref = slug ? `/g/${encodeURIComponent(slug)}/locations` : "#"

  return (
    <>
      <RouteTitle title={location.data?.name ?? t("locations:detail.fallbackTitle")} />
      <div
        className="flex flex-col gap-6 p-6 max-w-3xl mx-auto w-full"
        data-testid="page-location-detail"
      >
        <Link
          to={backHref}
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
          {t("locations:detail.back")}
        </Link>

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
                  <MapPin className="size-5 text-muted-foreground" aria-hidden="true" />
                  <span className="truncate">{location.data.name}</span>
                </h1>
                {location.data.address ? (
                  <p className="mt-1 text-muted-foreground">{location.data.address}</p>
                ) : null}
              </div>
              <div className="flex items-center gap-2 shrink-0">
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

            <Card>
              <CardHeader>
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <CardTitle className="text-base">{t("locations:detail.areasTitle")}</CardTitle>
                    <CardDescription>
                      {t("locations:detail.areasDescription", { count: myAreas.length })}
                    </CardDescription>
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
              </CardHeader>
              <CardContent>
                {myAreas.length === 0 ? (
                  <p
                    className="text-sm text-muted-foreground"
                    data-testid="location-detail-areas-empty"
                  >
                    {t("locations:detail.areasEmpty")}
                  </p>
                ) : (
                  <ul className="divide-y divide-border rounded-md border border-border">
                    {myAreas.map((area) => {
                      const areaHref =
                        slug && area.id
                          ? `/g/${encodeURIComponent(slug)}/areas/${encodeURIComponent(area.id)}`
                          : "#"
                      return (
                        <li
                          key={area.id}
                          className="flex items-center justify-between px-3 py-2"
                          data-testid="location-detail-area"
                        >
                          <Link
                            to={areaHref}
                            className="flex items-center gap-2 text-sm hover:underline min-w-0"
                          >
                            <ChevronRight
                              className="size-3.5 text-muted-foreground"
                              aria-hidden="true"
                            />
                            <span className="truncate">{area.name}</span>
                          </Link>
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => handleDeleteArea(area)}
                            aria-label={t("locations:list.deleteArea", { name: area.name ?? "" })}
                          >
                            <Trash2 className="size-3.5 text-destructive" aria-hidden="true" />
                          </Button>
                        </li>
                      )
                    })}
                  </ul>
                )}
              </CardContent>
            </Card>

            <EntityFilesPanel linkedEntityType="location" linkedEntityId={id} />
          </>
        ) : null}
      </div>

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
    </>
  )
}
