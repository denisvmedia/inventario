import { useEffect, useState } from "react"
import { useMatch, useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { Package, Pencil, Trash2 } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Page } from "@/components/ui/page"
import { Skeleton } from "@/components/ui/skeleton"
import { DropOverlay } from "@/components/files/DropOverlay"
import { EntityFilesPanel } from "@/components/files/EntityFilesPanel"
import { UploadFilesDialog } from "@/components/files/UploadFilesDialog"
import { useFileDropZone } from "@/components/files/useFileDropZone"
import { LocationsBreadcrumb } from "@/components/locations/LocationsBreadcrumb"
import { AreaFormDialog } from "@/components/locations/AreaFormDialog"
import { DeleteWithItemsDialog } from "@/components/locations/DeleteWithItemsDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { AreaItemsPanel } from "@/pages/areas/AreaItemsPanel"
import { useArea, useDeleteArea, useUpdateArea } from "@/features/areas/hooks"
import { useCommodities } from "@/features/commodities/hooks"
import { useLocation, useLocations } from "@/features/locations/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import type { DeleteStrategy } from "@/features/areas/api"

interface AreaDetailPageProps {
  initialMode?: "edit"
}

// /areas/:id — single-area detail. Header + edit/delete actions plus an
// inline items panel modelled on `design-mocks/src/views/LocationPickerView.tsx`
// Level 3 (stats strip + full toolbar / filters / sort / view-mode /
// pagination, scoped to the area), and an Area Files panel beneath. The
// drop overlay catches files dragged over the page and routes them
// through the unified `UploadFilesDialog` with `linkedEntityType="area"`
// — mirrors the LocationDetailPage shape (#1448).
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
  // Drives the empty-vs-non-empty delete branch (#2137). We only need the
  // total, so a single-row page keeps the request cheap.
  const itemsInArea = useCommodities(
    { areaId: id, perPage: 1, includeInactive: true },
    { enabled: !!currentGroup && !!id }
  )
  const itemCount = itemsInArea.data?.total ?? 0
  const updateArea = useUpdateArea(id)
  const deleteArea = useDeleteArea()

  const toast = useAppToast()
  const confirm = useConfirm()

  const [editOpen, setEditOpen] = useState(initialMode === "edit")
  // Open state for the non-empty (strategy-choice) delete dialog. #2137
  const [deleteWithItemsOpen, setDeleteWithItemsOpen] = useState(false)

  // Drop-overlay + upload dialog wiring — same pattern as
  // LocationDetailPage. Dragging files anywhere on the page surfaces the
  // overlay; releasing seeds the dialog with the drop's files and the
  // area's id/name so the unified UploadFilesDialog links them on
  // success.
  const [uploadOpen, setUploadOpen] = useState(false)
  const [pendingDropFiles, setPendingDropFiles] = useState<File[]>([])
  const dropZone = useFileDropZone({
    onFiles: (files) => {
      setPendingDropFiles(files)
      setUploadOpen(true)
    },
    disabled: uploadOpen,
  })

  const editMatch = useMatch({ path: "/g/:groupSlug/areas/:id/edit", end: true })
  useEffect(() => {
    // Deep-link sync from URL → local dialog state.
    // eslint-disable-next-line react-hooks/set-state-in-effect
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

  async function handleEdit(values: { name: string; location_id: string; icon: string }) {
    await updateArea.mutateAsync(values)
    toast.success(t("locations:toast.areaUpdated"))
  }

  function navigateAfterDelete() {
    if (slug && area.data?.location_id) {
      navigate(
        `/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(area.data.location_id)}`,
        { replace: true }
      )
    } else if (slug) {
      navigate(`/g/${encodeURIComponent(slug)}/locations`, { replace: true })
    }
  }

  // Shared delete path. `strategy` is omitted for an empty area (the BE's
  // safe default) and supplied when the user picks one in the non-empty
  // dialog (#2137).
  async function runDelete(strategy?: DeleteStrategy) {
    if (!id) return
    try {
      await deleteArea.mutateAsync({ id, strategy })
      toast.success(t("locations:toast.areaDeleted"))
      navigateAfterDelete()
    } catch {
      toast.error(t("locations:toast.areaDeleteError"))
    }
  }

  async function handleDelete() {
    if (!id) return
    // Non-empty → offer the cascade/unlink choice; empty → plain confirm.
    if (itemCount > 0) {
      setDeleteWithItemsOpen(true)
      return
    }
    const ok = await confirm({
      title: t("locations:delete.areaTitle", { name: area.data?.name ?? "" }),
      description: t("locations:delete.areaDescription"),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (!ok) return
    await runDelete()
  }

  if (area.isError) {
    return (
      <Page width="wide">
        <RouteTitle title={t("locations:areaDetail.errorTitle")} />
        <Alert variant="destructive" data-testid="area-detail-error">
          <AlertTitle>{t("locations:areaDetail.errorTitle")}</AlertTitle>
          <AlertDescription>{t("locations:areaDetail.errorDescription")}</AlertDescription>
        </Alert>
      </Page>
    )
  }

  const locationsHref = slug ? `/g/${encodeURIComponent(slug)}/locations` : "#"
  const parentHref =
    slug && area.data?.location_id
      ? `/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(area.data.location_id)}`
      : undefined
  // The breadcrumb's back chevron mirrors the most-natural "one level
  // up" target: parent location when known, locations list otherwise.
  const breadcrumbBackHref = parentHref ?? locationsHref

  return (
    <>
      <RouteTitle title={area.data?.name ?? t("locations:areaDetail.fallbackTitle")} />
      <Page
        width="wide"
        className="relative"
        data-testid="page-area-detail"
        {...dropZone.bindProps}
      >
        {dropZone.isDragging ? (
          <DropOverlay
            label={t("files:entityPanel.dropOverlay_area")}
            hint={t("files:entityPanel.dropHint")}
          />
        ) : null}
        <LocationsBreadcrumb
          backHref={breadcrumbBackHref}
          backLabel={t("locations:areaDetail.back")}
          navLabel={t("locations:breadcrumb.navLabel")}
          segments={[
            {
              label: t("locations:breadcrumb.locations"),
              to: locationsHref,
              testId: "breadcrumb-locations",
            },
            {
              label: parent.data?.name ?? t("locations:detail.fallbackTitle"),
              to: parentHref,
              testId: "breadcrumb-location",
            },
            {
              label: area.data?.name ?? t("locations:areaDetail.fallbackTitle"),
              testId: "breadcrumb-current",
            },
          ]}
          testId="area-detail-breadcrumb"
        />

        {area.isLoading ? (
          <div className="space-y-3" data-testid="area-detail-loading">
            <Skeleton className="h-8 w-64" />
            <Skeleton className="h-4 w-96" />
          </div>
        ) : area.data ? (
          <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0">
              <h1 className="flex items-center gap-2 text-2xl font-semibold tracking-tight">
                {area.data.icon ? (
                  <span
                    className="text-2xl leading-none"
                    aria-hidden="true"
                    data-testid="area-detail-icon"
                  >
                    {area.data.icon}
                  </span>
                ) : (
                  <Package className="size-5 text-muted-foreground" aria-hidden="true" />
                )}
                <span className="truncate">{area.data.name}</span>
              </h1>
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

        {area.data ? (
          <>
            <AreaItemsPanel areaId={id} />
            <EntityFilesPanel
              linkedEntityType="area"
              linkedEntityId={id}
              onAttachClick={() => {
                setPendingDropFiles([])
                setUploadOpen(true)
              }}
            />
          </>
        ) : null}
      </Page>

      <AreaFormDialog
        open={editOpen}
        onOpenChange={(open) => (open ? null : closeDialog())}
        area={area.data}
        locations={allLocations.data ?? []}
        onSubmit={handleEdit}
        isPending={updateArea.isPending}
      />

      <DeleteWithItemsDialog
        open={deleteWithItemsOpen}
        kind="area"
        name={area.data?.name ?? ""}
        itemCount={itemCount}
        isPending={deleteArea.isPending}
        onResolve={(strategy) => {
          setDeleteWithItemsOpen(false)
          if (strategy) void runDelete(strategy)
        }}
      />

      <UploadFilesDialog
        open={uploadOpen}
        onOpenChange={(open) => {
          setUploadOpen(open)
          if (!open) setPendingDropFiles([])
        }}
        linkedEntity={{
          type: "area",
          id,
          name: area.data?.name,
        }}
        initialFiles={pendingDropFiles}
      />
    </>
  )
}
