import type { TFunction } from "i18next"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { ITEM_TYPE_IN_USE, ITEM_TYPE_LAST, type ItemType } from "@/features/itemTypes/api"
import { itemTypeIconFor, itemTypeLabel } from "@/features/itemTypes/display"
import { useDeleteItemType } from "@/features/itemTypes/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import {
  classifyServerError,
  getServerErrorCode,
  getServerErrorMeta,
  type ClassifiedServerError,
} from "@/lib/server-error"

export interface ItemTypeDeleteDialogProps {
  // The type to delete; null closes the dialog.
  itemType: ItemType | null
  // The group's types, which the items can move to.
  types: readonly ItemType[]
  onClose: () => void
}

interface Usage {
  live: number
  trashed: number
  drafts: number
}

interface DialogState {
  open: boolean
  type: ItemType | null
  // The id of the type the items move to.
  target: string
  // What the server reported when it refused a delete without a target:
  // items or drafts got the type after the page loaded its counts.
  refusedUsage: Usage | null
  // The server refused to delete the group's last type.
  last: boolean
  serverError: ClassifiedServerError | null
}

function freshState(type: ItemType | null): DialogState {
  return {
    open: type !== null,
    type,
    target: "",
    refusedUsage: null,
    last: false,
    serverError: null,
  }
}

function usageOf(type: ItemType | null): Usage {
  return {
    live: type?.item_count ?? 0,
    trashed: type?.trashed_item_count ?? 0,
    drafts: type?.draft_count ?? 0,
  }
}

// description says what deleting the type does to the items and drafts
// that have it.
function description(t: TFunction, name: string, usage: Usage): string {
  const items = usage.live + usage.trashed
  if (items === 0) {
    return usage.drafts > 0
      ? t("itemTypes:removal.draftsOnly", { name, count: usage.drafts })
      : t("itemTypes:removal.unused", { name })
  }
  const text =
    usage.trashed > 0
      ? t("itemTypes:removal.inUseWithTrash", { name, count: items, trashed: usage.trashed })
      : t("itemTypes:removal.inUse", { name, count: usage.live })
  return usage.drafts > 0
    ? `${text} ${t("itemTypes:removal.drafts", { count: usage.drafts })}`
    : text
}

// ItemTypeDeleteDialog deletes an item type (#2802). A type that items
// have, in Trash included, or drafts have, is deleted only together with
// moving them to another type the user picks, which is also how two types
// are merged. The mock has no item types; the dialog follows the confirm
// dialog (useConfirm) with a Select of the other types.
export function ItemTypeDeleteDialog({ itemType, types, onClose }: ItemTypeDeleteDialogProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const deleteMutation = useDeleteItemType()
  // The dialog's state belongs to one opening. It is reset each time the
  // dialog opens, and kept while it closes, so the closing animation still
  // shows the type's name.
  const [state, setState] = useState<DialogState>(() => freshState(itemType))
  if (itemType && (!state.open || itemType.id !== state.type?.id)) {
    setState(freshState(itemType))
  } else if (!itemType && state.open) {
    setState((prev) => ({ ...prev, open: false }))
  }
  const { type: shown, target, refusedUsage, last, serverError } = state
  const setTarget = (value: string) => setState((prev) => ({ ...prev, target: value }))

  const listed = shown ? (types.find((type) => type.id === shown.id) ?? shown) : null
  const usage = refusedUsage ?? usageOf(listed)
  const items = usage.live + usage.trashed
  const inUse = refusedUsage !== null || items + usage.drafts > 0
  const draftsOnly = items === 0 && usage.drafts > 0
  const others = types.filter((type) => type.id !== shown?.id)
  const name = shown ? itemTypeLabel(t, shown) : ""
  const targetType = others.find((type) => type.id === target)

  async function confirm() {
    if (!shown) return
    if (inUse && !targetType) return
    setState((prev) => ({ ...prev, serverError: null }))
    try {
      const moved = await deleteMutation.mutateAsync({
        id: shown.id,
        reassignTo: inUse ? targetType?.id : undefined,
      })
      toast.success(
        moved > 0 && targetType
          ? t("itemTypes:toast.deletedMoved", {
              name,
              count: moved,
              target: itemTypeLabel(t, targetType),
            })
          : t("itemTypes:toast.deleted", { name })
      )
      onClose()
    } catch (err) {
      const code = getServerErrorCode(err)
      if (code === ITEM_TYPE_IN_USE) {
        const meta = getServerErrorMeta(err)
        const refused = {
          live: Number(meta?.item_count ?? 0) || 0,
          trashed: Number(meta?.trashed_item_count ?? 0) || 0,
          drafts: Number(meta?.draft_count ?? 0) || 0,
        }
        setState((prev) => ({ ...prev, refusedUsage: refused }))
        return
      }
      if (code === ITEM_TYPE_LAST) {
        setState((prev) => ({ ...prev, last: true }))
        return
      }
      const error = classifyServerError(err, t("itemTypes:toast.deleteError"))
      setState((prev) => ({ ...prev, serverError: error }))
    }
  }

  const pending = deleteMutation.isPending

  return (
    <Dialog
      open={itemType !== null}
      onOpenChange={(open) => {
        if (!open && !pending) onClose()
      }}
    >
      <DialogContent className="sm:max-w-md" data-testid="item-type-delete-dialog">
        <DialogHeader>
          <DialogTitle>{t("itemTypes:removal.title", { name })}</DialogTitle>
          <DialogDescription data-testid="item-type-delete-description">
            {description(t, name, usage)}
          </DialogDescription>
        </DialogHeader>

        {refusedUsage ? (
          <p className="text-sm text-muted-foreground" data-testid="item-type-delete-refused">
            {t("itemTypes:removal.gotItems")}
          </p>
        ) : null}

        {inUse ? (
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="item-type-delete-target">
              {draftsOnly
                ? t("itemTypes:removal.targetLabelDrafts")
                : t("itemTypes:removal.targetLabel")}
            </Label>
            <Select value={target} onValueChange={setTarget} disabled={pending}>
              <SelectTrigger
                id="item-type-delete-target"
                className="w-full"
                data-testid="item-type-delete-target"
              >
                <SelectValue placeholder={t("itemTypes:removal.targetPlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                {others.map((type) => {
                  const Icon = itemTypeIconFor(type)
                  return (
                    <SelectItem key={type.id} value={type.id}>
                      <Icon aria-hidden="true" />
                      {itemTypeLabel(t, type)}
                    </SelectItem>
                  )
                })}
              </SelectContent>
            </Select>
          </div>
        ) : null}

        {last ? (
          <p className="text-sm text-destructive" role="alert" data-testid="item-type-delete-last">
            {t("itemTypes:removal.lastType")}
          </p>
        ) : null}

        <ServerErrorBanner error={serverError} testId="item-type-delete-error" />

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={pending}
            data-testid="item-type-delete-cancel"
          >
            {t("common:actions.cancel")}
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={() => void confirm()}
            disabled={pending || last || (inUse && !targetType)}
            data-testid="item-type-delete-confirm"
          >
            {!inUse
              ? t("itemTypes:removal.confirm")
              : draftsOnly
                ? t("itemTypes:removal.confirmMoveDrafts")
                : t("itemTypes:removal.confirmMove")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
