import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Plus } from "lucide-react"

import { PortalEventBoundary } from "@/components/PortalEventBoundary"
import { ItemTypeFormDialog } from "@/components/itemTypes/ItemTypeFormDialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import type { ItemType } from "@/features/itemTypes/api"
import { itemTypeIconFor, itemTypeLabel } from "@/features/itemTypes/display"
import { useCreateItemType, useItemTypes } from "@/features/itemTypes/hooks"
import type { ItemTypeFormValues } from "@/features/itemTypes/schemas"
import { useAppToast } from "@/hooks/useAppToast"

// The value of the "New type" entry. It is never stored: picking it opens
// the create dialog instead. No slug can take it, since a slug starts with
// a letter or a digit.
const CREATE_VALUE = "__create__"

export interface ItemTypeFieldProps {
  // The selected type's slug, "" for none.
  value: string
  onChange: (slug: string) => void
  // The select's id, for the host's label.
  id: string
  invalid?: boolean
  // Set when a form field carries an error, so the field can refetch the
  // list and tell whether the type was deleted meanwhile.
  errorMessage?: string
  ariaRequired?: boolean
  ariaDescribedBy?: string
}

// ItemTypeField picks an item's type from the group's item types (#2802).
// The list ends with "New type", which opens the Item types page's create
// dialog over the form and selects what it created, as the seller and the
// location pickers do. Item types are group content, so every member who
// may edit the item may create one. Without a group (the form of a visitor
// who has not signed up yet) it offers the built-in types and no "New
// type". The host renders the label and the field error.
export function ItemTypeField({
  value,
  onChange,
  id,
  invalid = false,
  errorMessage,
  ariaRequired,
  ariaDescribedBy,
}: ItemTypeFieldProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const types = useItemTypes()
  const createType = useCreateItemType()
  const canCreate = !!useOptionalCurrentGroup()?.currentGroup?.slug
  // Types created here, with the time the create was sent. Each one is
  // shown until a fetch of the list lands after that time (a successful
  // create refetches it); from then on the list is the truth.
  const [created, setCreated] = useState<{ type: ItemType; sentAt: number }[]>([])
  const listed = types.data ?? []
  const pending = created
    .filter((c) => c.sentAt > types.dataUpdatedAt && !listed.some((s) => s.id === c.type.id))
    .map((c) => c.type)
  const all = [...listed, ...pending]

  // The selected type is missing from the group's list, which can be up to
  // a minute old. The list is fetched once more, and the type counts as gone
  // (another member deleted it and its items moved to another type) only
  // when a list fetched after the doubt arose still lacks it. A failed fetch
  // proves nothing, and a fetch started for an earlier value is ignored.
  const absent =
    !!value && types.isSuccess && !types.isPlaceholderData && !all.some((s) => s.slug === value)
  const [checked, setChecked] = useState<{ slug: string; startedAt: number } | null>(null)
  const gone = absent && checked?.slug === value && types.dataUpdatedAt >= checked.startedAt
  const { refetch } = types
  const checking = useRef("")
  const check = useCallback(
    (slug: string) => {
      if (checking.current === slug) return
      checking.current = slug
      const startedAt = Date.now()
      void refetch().then(() => {
        if (checking.current === slug) checking.current = ""
        setChecked({ slug, startedAt })
      })
    },
    [refetch]
  )
  useEffect(() => {
    if (absent && !gone) check(value)
  }, [absent, gone, value, check])

  // The server refuses a type the group does not have with a 422 on the
  // field, even when the list here still has it, so that is a reason to
  // check as well.
  useEffect(() => {
    if (errorMessage && value) check(value)
  }, [errorMessage, value, check])

  // The list closes before the dialog opens, or the dialog would open under
  // it; a letter typed on the closed select picks without opening the list,
  // and then the dialog opens at once.
  const listOpen = useRef(false)
  const requested = useRef(false)
  const trigger = useRef<HTMLButtonElement>(null)
  const [dialog, setDialog] = useState<{ id: number } | null>(null)

  // Escape closes the dialog while its create still runs, and the dialog can
  // open again before the create ends. The number of the opening a create
  // was sent from tells whether that dialog is still open when it ends.
  const lastOpening = useRef(0)
  const openNow = useRef(0)

  function openDialog() {
    lastOpening.current += 1
    openNow.current = lastOpening.current
    setDialog({ id: lastOpening.current })
  }

  function closeDialog(opening: number) {
    if (openNow.current !== opening) return
    openNow.current = 0
    setDialog(null)
  }

  function pick(next: string) {
    if (next === CREATE_VALUE) {
      if (listOpen.current) requested.current = true
      else openDialog()
      return
    }
    onChange(next)
  }

  function openRequested() {
    if (!requested.current) return
    requested.current = false
    openDialog()
  }

  const opening = dialog?.id ?? 0
  async function submit(values: ItemTypeFormValues) {
    const sentAt = Date.now()
    const type = await createType.mutateAsync(values)
    toast.success(t("itemTypes:toast.created"))
    setCreated((prev) => [...prev, { type, sentAt }])
    // Selected only while its dialog is still open, so a create that ends
    // after the dialog closed leaves a choice made meanwhile alone.
    if (openNow.current === opening && type.slug) onChange(type.slug)
  }

  return (
    <>
      {/* An empty string, not undefined, keeps the select controlled while it
          shows the placeholder, so the create entry never sticks as its value. */}
      <Select
        value={value}
        onValueChange={pick}
        onOpenChange={(open) => {
          listOpen.current = open
          if (open) requested.current = false
        }}
      >
        <SelectTrigger
          id={id}
          ref={trigger}
          className="w-full"
          aria-required={ariaRequired}
          aria-invalid={invalid || gone}
          aria-describedby={ariaDescribedBy}
          data-testid={id}
        >
          <SelectValue placeholder={t("commodities:fields.typePlaceholder")} />
        </SelectTrigger>
        <SelectContent onCloseAutoFocus={openRequested}>
          {all
            .filter((type) => type.slug)
            .map((type) => {
              const Icon = itemTypeIconFor(type)
              return (
                <SelectItem key={type.id} value={type.slug ?? ""}>
                  <Icon aria-hidden="true" />
                  {itemTypeLabel(t, type)}
                </SelectItem>
              )
            })}
          {canCreate && all.length > 0 ? <SelectSeparator /> : null}
          {canCreate ? (
            <SelectItem value={CREATE_VALUE} data-testid={`${id}-new`}>
              <Plus aria-hidden="true" />
              {t("itemTypes:field.newType")}
            </SelectItem>
          ) : null}
        </SelectContent>
      </Select>
      {gone ? (
        <p className="text-xs text-destructive" role="alert" data-testid={`${id}-gone`}>
          {t("itemTypes:field.gone")}
        </p>
      ) : null}
      <PortalEventBoundary>
        <ItemTypeFormDialog
          open={dialog !== null}
          onOpenChange={(open) => {
            if (!open) closeDialog(opening)
          }}
          types={all}
          onSubmit={submit}
          isPending={createType.isPending}
          onUseExisting={(type) => {
            if (openNow.current !== opening || !type.slug) return
            onChange(type.slug)
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault()
            trigger.current?.focus()
          }}
        />
      </PortalEventBoundary>
    </>
  )
}
