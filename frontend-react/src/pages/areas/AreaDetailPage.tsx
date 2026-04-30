import { useEffect, useState } from "react"
import { Link, useMatch, useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { ArrowLeft, MapPin, Package, Pencil, Trash2 } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { AreaFormDialog } from "@/components/locations/AreaFormDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useArea, useDeleteArea, useUpdateArea } from "@/features/areas/hooks"
import { useLocation, useLocations } from "@/features/locations/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"

interface AreaDetailPageProps {
  initialMode?: "edit"
}

// /areas/:id — single-area detail. Shows the area name and its parent
// location's breadcrumb. Edit + delete are the only actions today; the
// per-area commodity list lands once the items page (#1410) ships and
// can take an `?area=:id` filter.
export function AreaDetailPage({ initialMode }: AreaDetailPageProps = {}) {
  const { t } = useTranslation()
  const params = useParams<{ id: string }>()
  const id = params.id ?? ""
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug

  const area = useArea(id, { enabled: !!currentGroup })
  // Fetch the parent location for the breadcrumb. The detail endpoint
  // doesn't include the parent's name, so we hit /locations/:id once
  // the area resolves.
  const parent = useLocation(area.data?.location_id, {
    enabled: !!area.data?.location_id,
  })
  const allLocations = useLocations({ enabled: !!currentGroup })
  const updateArea = useUpdateArea(id)
  const deleteArea = useDeleteArea()

  const toast = useAppToast()
  const confirm = useConfirm()

  const [editOpen, setEditOpen] = useState(initialMode === "edit")

  const editMatch = useMatch({ path: "/g/:groupSlug/areas/:id/edit", end: true })
  useEffect(() => {
    if (editMatch && !editOpen) setEditOpen(true)
  }, [editMatch, editOpen])

  function closeDialog() {
    setEditOpen(false)
    if (editMatch && slug && id) {
      navigate(`/g/${encodeURIComponent(slug)}/areas/${encodeURIComponent(id)}`, {
        replace: true,
      })
    }
  }

  async function handleEdit(values: { name: string; location_id: string }) {
    await updateArea.mutateAsync(values)
    toast.success(t("locations:toast.areaUpdated"))
  }

  async function handleDelete() {
    if (!id) return
    const ok = await confirm({
      title: t("locations:delete.areaTitle", { name: area.data?.name ?? "" }),
      description: t("locations:delete.areaDescription"),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (!ok) return
    try {
      await deleteArea.mutateAsync(id)
      toast.success(t("locations:toast.areaDeleted"))
      if (slug && area.data?.location_id) {
        navigate(
          `/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(area.data.location_id)}`,
          { replace: true }
        )
      } else if (slug) {
        navigate(`/g/${encodeURIComponent(slug)}/locations`, { replace: true })
      }
    } catch {
      toast.error(t("locations:toast.areaDeleteError"))
    }
  }

  if (area.isError) {
    return (
      <div className="flex flex-col gap-6 p-6 max-w-3xl mx-auto w-full">
        <RouteTitle title={t("locations:areaDetail.errorTitle")} />
        <Alert variant="destructive" data-testid="area-detail-error">
          <AlertTitle>{t("locations:areaDetail.errorTitle")}</AlertTitle>
          <AlertDescription>{t("locations:areaDetail.errorDescription")}</AlertDescription>
        </Alert>
      </div>
    )
  }

  const backHref =
    slug && area.data?.location_id
      ? `/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(area.data.location_id)}`
      : slug
        ? `/g/${encodeURIComponent(slug)}/locations`
        : "#"

  return (
    <>
      <RouteTitle title={area.data?.name ?? t("locations:areaDetail.fallbackTitle")} />
      <div
        className="flex flex-col gap-6 p-6 max-w-3xl mx-auto w-full"
        data-testid="page-area-detail"
      >
        <Link
          to={backHref}
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
          {parent.data ? parent.data.name : t("locations:areaDetail.back")}
        </Link>

        {area.isLoading ? (
          <div className="space-y-3" data-testid="area-detail-loading">
            <Skeleton className="h-8 w-64" />
            <Skeleton className="h-4 w-96" />
          </div>
        ) : area.data ? (
          <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0">
              <h1 className="flex items-center gap-2 text-2xl font-semibold tracking-tight">
                <Package className="size-5 text-muted-foreground" aria-hidden="true" />
                <span className="truncate">{area.data.name}</span>
              </h1>
              {parent.data ? (
                <p className="mt-1 text-sm text-muted-foreground inline-flex items-center gap-1.5">
                  <MapPin className="size-3.5" aria-hidden="true" />
                  {parent.data.name}
                </p>
              ) : null}
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <Button
                type="button"
                variant="outline"
                onClick={() => setEditOpen(true)}
                data-testid="area-detail-edit"
                className="gap-2"
              >
                <Pencil className="size-4" aria-hidden="true" />
                {t("locations:detail.edit")}
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={handleDelete}
                data-testid="area-detail-delete"
                className="gap-2"
              >
                <Trash2 className="size-4 text-destructive" aria-hidden="true" />
                {t("common:actions.delete")}
              </Button>
            </div>
          </header>
        ) : null}

        <Alert data-testid="area-detail-items-soon">
          <AlertTitle>{t("locations:areaDetail.itemsSoonTitle")}</AlertTitle>
          <AlertDescription>{t("locations:areaDetail.itemsSoonDescription")}</AlertDescription>
        </Alert>
      </div>

      <AreaFormDialog
        open={editOpen}
        onOpenChange={(open) => (open ? null : closeDialog())}
        area={area.data}
        locations={allLocations.data ?? []}
        onSubmit={handleEdit}
        isPending={updateArea.isPending}
      />
    </>
  )
}
