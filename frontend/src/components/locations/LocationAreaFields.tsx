import {
  useEffect,
  useRef,
  useState,
  type DragEvent,
  type KeyboardEvent,
  type ReactNode,
  type RefObject,
} from "react"
import { useTranslation } from "react-i18next"
import { Plus } from "lucide-react"

import { AreaFormDialog } from "@/components/locations/AreaFormDialog"
import { LocationFormDialog } from "@/components/locations/LocationFormDialog"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useCreateArea } from "@/features/areas/hooks"
import type { AreaFormInput } from "@/features/areas/schemas"
import { canManageLocations, useMyGroupRole } from "@/features/group/roles"
import { useCreateLocation } from "@/features/locations/hooks"
import type { LocationFormInput } from "@/features/locations/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { withId } from "@/lib/with-id"

export interface AreaOption {
  id?: string
  name?: string
  location_id?: string
}

export interface LocationOption {
  id?: string
  name?: string
}

// The value of the "New location" and "New area" entries. It is never
// stored: picking the entry opens the create dialog instead.
const CREATE_VALUE = "__create__"

type CreateKind = "location" | "area"

export interface LocationAreaFieldsProps {
  areas: AreaOption[]
  locations: LocationOption[]
  areaId: string
  onAreaChange: (areaId: string) => void
  /** The selects get ids `<prefix>-location` and `<prefix>-area`. */
  idPrefix: string
  areaError?: { message?: string }
  /** Renders a field label, for a form whose labels differ from the plain one. */
  renderLabel?: (htmlFor: string, text: string) => ReactNode
}

// The mock's paired Location / Area selects. Only the area is stored; the
// location follows from it and narrows the areas offered. Picking another
// location clears the area, and "Clear location" leaves the item unassigned.
//
// A member who may manage locations (admin, owner) also gets "New location"
// and "New area" at the end of the lists. Each opens the create dialog of
// the Locations page over the form and selects what it created, so a place
// that does not exist yet does not mean leaving the form (#2796).
export function LocationAreaFields({
  areas,
  locations,
  areaId,
  onAreaChange,
  idPrefix,
  areaError,
  renderLabel = defaultLabel,
}: LocationAreaFieldsProps) {
  const { t } = useTranslation()
  const canCreate = canManageLocations(useMyGroupRole())
  // Entries created here, kept until the lists passed in include them.
  const [createdLocations, setCreatedLocations] = useState<LocationOption[]>([])
  const [createdAreas, setCreatedAreas] = useState<AreaOption[]>([])
  const allLocations = withCreated(locations, createdLocations)
  const allAreas = withCreated(areas, createdAreas)

  const locationOfArea = (id: string) => areas.find((a) => a.id === id)?.location_id ?? ""
  const [locationId, setLocationId] = useState(() => locationOfArea(areaId))
  // The areas can arrive after the first render; fill the location in once
  // they do, unless the user has picked one already.
  useEffect(() => {
    if (locationId || !areaId) return
    const found = areas.find((a) => a.id === areaId)?.location_id
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time back-fill once the areas query resolves; a location the user picked is left alone.
    if (found) setLocationId(found)
  }, [areas, areaId, locationId])
  const visibleAreas = locationId ? allAreas.filter((a) => a.location_id === locationId) : []

  // The select whose list is open, the create dialog asked for in that list
  // while it closes, and the dialog that is open. Closing a dialog returns
  // the focus to the select it was opened from.
  const listOpen = useRef<CreateKind | null>(null)
  const requested = useRef<CreateKind | null>(null)
  const [creating, setCreating] = useState<CreateKind | null>(null)
  const locationTrigger = useRef<HTMLButtonElement>(null)
  const areaTrigger = useRef<HTMLButtonElement>(null)

  // A pick from the open list waits for the list to close (openRequested),
  // so the dialog never opens under it. Typing a letter on a closed select
  // picks the matching entry without opening the list, and then nothing
  // would close it; the dialog opens at once.
  function request(kind: CreateKind) {
    if (listOpen.current === kind) requested.current = kind
    else setCreating(kind)
  }

  function trackOpen(kind: CreateKind, open: boolean) {
    listOpen.current = open ? kind : null
    if (open) requested.current = null
  }

  function pickLocation(next: string) {
    if (next === CREATE_VALUE) {
      request("location")
      return
    }
    if (next === locationId) return
    setLocationId(next)
    onAreaChange("")
  }

  function pickArea(next: string) {
    if (next === CREATE_VALUE) {
      request("area")
      return
    }
    onAreaChange(next)
  }

  // Runs as a list closes, after its pick.
  function openRequested() {
    const kind = requested.current
    requested.current = null
    if (kind) setCreating(kind)
  }

  function locationCreated(created: LocationOption) {
    if (!created.id) return
    setCreatedLocations((prev) => [...prev, created])
    setLocationId(created.id)
    onAreaChange("")
  }

  function areaCreated(created: AreaOption) {
    if (!created.id) return
    setCreatedAreas((prev) => [...prev, created])
    // The dialog lets the user pick another parent than the one selected.
    if (created.location_id) setLocationId(created.location_id)
    onAreaChange(created.id)
  }

  return (
    <div className="grid grid-cols-2 gap-3">
      <div className="flex flex-col gap-1.5">
        <div className="flex items-center justify-between gap-2">
          {renderLabel(`${idPrefix}-location`, t("commodities:fields.location"))}
          {locationId ? (
            <button
              type="button"
              onClick={() => pickLocation("")}
              className="text-xs text-muted-foreground transition-colors hover:text-foreground"
              data-testid={`${idPrefix}-location-clear`}
            >
              {t("commodities:fields.clearLocation")}
            </button>
          ) : null}
        </div>
        {/* An empty string, not undefined, keeps the select controlled while
            it shows the placeholder, so the create entry never sticks as its
            value. */}
        <Select
          value={locationId}
          onValueChange={pickLocation}
          onOpenChange={(open) => trackOpen("location", open)}
        >
          <SelectTrigger id={`${idPrefix}-location`} ref={locationTrigger} className="w-full">
            <SelectValue placeholder={t("commodities:fields.locationPlaceholder")} />
          </SelectTrigger>
          <SelectContent onCloseAutoFocus={openRequested}>
            {withId(allLocations).map((l) => (
              <SelectItem key={l.id} value={l.id}>
                {l.name}
              </SelectItem>
            ))}
            {canCreate ? (
              <CreateEntry
                separated={allLocations.length > 0}
                label={t("commodities:fields.newLocation")}
                testId={`${idPrefix}-location-new`}
              />
            ) : null}
          </SelectContent>
        </Select>
      </div>
      <div className="flex flex-col gap-1.5">
        {renderLabel(`${idPrefix}-area`, t("commodities:fields.area"))}
        <Select
          // Remount on a location change so the trigger drops the old area's
          // label and shows the placeholder again.
          key={locationId || "no-location"}
          value={areaId}
          onValueChange={pickArea}
          onOpenChange={(open) => trackOpen("area", open)}
          disabled={!locationId}
        >
          <SelectTrigger
            id={`${idPrefix}-area`}
            ref={areaTrigger}
            className="w-full"
            aria-invalid={!!areaError}
          >
            <SelectValue
              placeholder={
                locationId
                  ? t("commodities:fields.areaPlaceholder")
                  : t("commodities:fields.areaPlaceholderNoLocation")
              }
            />
          </SelectTrigger>
          <SelectContent onCloseAutoFocus={openRequested}>
            {withId(visibleAreas).map((a) => (
              <SelectItem key={a.id} value={a.id}>
                {a.name}
              </SelectItem>
            ))}
            {canCreate ? (
              <CreateEntry
                separated={visibleAreas.length > 0}
                label={t("commodities:fields.newArea")}
                testId={`${idPrefix}-area-new`}
              />
            ) : null}
          </SelectContent>
        </Select>
        {areaError?.message ? (
          <p className="text-xs text-destructive" role="alert">
            {t(areaError.message)}
          </p>
        ) : null}
      </div>
      {canCreate ? (
        <CreateDialogs
          creating={creating}
          onClose={() => setCreating(null)}
          locations={allLocations}
          locationId={locationId}
          locationTrigger={locationTrigger}
          areaTrigger={areaTrigger}
          onLocationCreated={locationCreated}
          onAreaCreated={areaCreated}
        />
      ) : null}
    </div>
  )
}

function defaultLabel(htmlFor: string, text: string) {
  return (
    <Label htmlFor={htmlFor} className="text-sm font-medium">
      {text}
    </Label>
  )
}

function withCreated<T extends { id?: string }>(list: T[], created: T[]): T[] {
  const extra = created.filter((c) => !list.some((item) => item.id === c.id))
  return extra.length > 0 ? [...list, ...extra] : list
}

function CreateEntry({
  separated,
  label,
  testId,
}: {
  separated: boolean
  label: string
  testId: string
}) {
  return (
    <>
      {separated ? <SelectSeparator /> : null}
      <SelectItem value={CREATE_VALUE} data-testid={testId}>
        <Plus aria-hidden="true" />
        {label}
      </SelectItem>
    </>
  )
}

interface CreateDialogsProps {
  creating: CreateKind | null
  onClose: () => void
  locations: LocationOption[]
  locationId: string
  locationTrigger: RefObject<HTMLButtonElement | null>
  areaTrigger: RefObject<HTMLButtonElement | null>
  onLocationCreated: (created: LocationOption) => void
  onAreaCreated: (created: AreaOption) => void
}

// The create dialogs of the Locations page, opened over the form. Mounted
// only for a member who may create, as the mutations need the group.
function CreateDialogs({
  creating,
  onClose,
  locations,
  locationId,
  locationTrigger,
  areaTrigger,
  onLocationCreated,
  onAreaCreated,
}: CreateDialogsProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const createLocation = useCreateLocation()
  const createArea = useCreateArea()

  // A failure rejects, and the dialog keeps what was typed and shows the
  // server's error.
  async function submitLocation(values: LocationFormInput) {
    const created = await createLocation.mutateAsync(values)
    toast.success(t("locations:toast.locationCreated"))
    onLocationCreated(created)
  }

  async function submitArea(values: AreaFormInput) {
    const created = await createArea.mutateAsync(values)
    toast.success(t("locations:toast.areaCreated"))
    onAreaCreated(created)
  }

  const closeOnDismiss = (open: boolean) => {
    if (!open) onClose()
  }
  const focusOnClose = (trigger: RefObject<HTMLButtonElement | null>) => (event: Event) => {
    event.preventDefault()
    trigger.current?.focus()
  }

  return (
    // The dialogs render in a portal, outside the form in the page, but React
    // still bubbles their events through the components around this picker.
    // A submit would reach the item form and save the item, Enter would reach
    // the step-by-step dialog and move it to the next step, and a file dropped
    // on a dialog would reach the item page, which takes it as an attachment.
    <div
      role="presentation"
      className="contents"
      onSubmit={stopEvent}
      onKeyDown={stopEnter}
      onDragEnter={refuseFileDrag}
      onDragOver={refuseFileDrag}
      onDragLeave={refuseFileDrag}
      onDrop={refuseFileDrag}
    >
      <LocationFormDialog
        open={creating === "location"}
        onOpenChange={closeOnDismiss}
        onSubmit={submitLocation}
        isPending={createLocation.isPending}
        onCloseAutoFocus={focusOnClose(locationTrigger)}
      />
      <AreaFormDialog
        open={creating === "area"}
        onOpenChange={closeOnDismiss}
        locations={locations}
        defaultLocationId={locationId || undefined}
        onSubmit={submitArea}
        isPending={createArea.isPending}
        onCloseAutoFocus={focusOnClose(areaTrigger)}
      />
    </div>
  )
}

function stopEvent(event: { stopPropagation: () => void }) {
  event.stopPropagation()
}

function stopEnter(event: KeyboardEvent) {
  if (event.key === "Enter") event.stopPropagation()
}

// The default is prevented too, or the browser would open the dropped file
// in place of the page and drop the form with it.
function refuseFileDrag(event: DragEvent) {
  if (!Array.from(event.dataTransfer?.types ?? []).includes("Files")) return
  event.stopPropagation()
  event.preventDefault()
  event.dataTransfer.dropEffect = "none"
}
